# 一键部署、更新与回退

中央平台支持 Ubuntu / Debian + systemd，amd64 / arm64；运行时不需要 Go、Node.js 或 Docker。Agent 支持的系统见 [Agent 文档](AGENTS.md)。

## 一键入口

独立运行可直接使用服务器 IPv4 和应用端口（默认 8080），无需 Nginx。使用域名和已有反向代理时，将域名解析到中央服务器并开放 443；只有自动申请证书才需要开放 80。数据库端口无需对公网开放。私有仓库先在当前普通用户下执行 `gh auth login`，账号需要仓库读取权限。

```sh
git clone https://github.com/xiuzhenzhi/SSH.git
cd SSH
sh scripts/quick-start.sh
```

最后一条命令下载固定版本 `v0.3.1` 的对应架构发行包，校验 SHA256，再调用安装向导。下载使用当前用户的 GitHub 登录，只有安装时调用 sudo。不要把 GitHub Token 放进命令行或发给 Agent。

向导询问：

1. 接入方式：独立运行、复用已有 Web 服务、由脚本管理系统 Nginx。检测到 Nginx / OpenResty / 宝塔或 80/443 服务时推荐复用，否则默认独立运行。
2. 访问域名或 IPv4。独立模式可选择端口、HTTP 或内置 HTTPS（提供已有证书与私钥）。
3. 数据库：默认 `sqlite`，无需 DSN；`local-postgres` 自动安装本机 PostgreSQL；`mysql` / `postgres` 连接已有数据库，DSN 隐藏输入。
4. 只有托管 Nginx 模式才配置 Nginx 证书，支持已有证书或 Certbot/Let's Encrypt（接受 ACME 服务条款，公网 80 端口可达）。复用模式沿用现有站点证书。

向导仅安装缺少的依赖、初始化数据库、创建低权限服务账号、配置 systemd，最后检查 API 健康状态。独立和复用模式不会安装或重载 Nginx / OpenResty。初始账号 `admin`，随机密码仅保存在 `/etc/sshguard/api.env`，不会输出到部署日志。登录后点右上角「修改密码」，旧会话会全部失效。

独立模式访问 `http://服务器IP:8080`，或启用内置 TLS 后访问 `https://域名:端口`。HTTP 登录信息没有传输加密，仅用于受信任网络体验；Agent 注册与更新要求 HTTPS。程序直接提供发布包中的 `web/`，无需 Node.js 或单独的 Web 服务。

复用 Nginx / OpenResty / 宝塔时选择 `--proxy external`，在已有 HTTPS 站点把整个应用反代到 `http://127.0.0.1:8080`（或自选 APP_PORT），并启用 WebSocket。参考生成的 `/etc/sshguard/nginx-existing.conf`，将 location 配置合并进现有 server 块，避免重复的 `location /`；由你自己的面板验证并重载配置。无需再配置静态网页目录。不要为了安装本项目停止已有网站或安装第二套 Nginx。

更新会保留接入方式、端口、数据库和加密密钥。切换接入方式需手动迁移配置，不能仅通过更新参数切换。以上独立运行及复用向导需要包含这些功能的新发行包；v0.3.1 仍使用旧向导。

SHA256 用于检查下载完整性；发行包与校验文件来自同一个受信任仓库，它不等同于独立数字签名。

SQLite 从 v0.3.1 开始支持，安装前确认该版本的 Release 附件已构建完成。固定版本下载不会使用源码目录中的未发布修改。向导将数据库放在 `/var/lib/sshguard-api/sshguard.db`，由 systemd 创建权限为 0700 的专用目录；升级保留数据库和加密密钥。无人值守部署设置 `DB_DRIVER=sqlite` 并省略 `DATABASE_URL`，其他域名和证书配置与下述示例相同。使用本机磁盘，备份前停止 API，同时保存数据库文件、存在的 WAL/SHM 文件和 `/etc/sshguard/api.env`。

## 离线包 / 不在服务器保存 GitHub 登录

在有权限的机器下载发行包和 `SHA256SUMS`，传到中央服务器：

```sh
awk '$2=="sshguard-linux-amd64.tar.gz" {print}' SHA256SUMS | sha256sum -c -
tar -xzf sshguard-linux-amd64.tar.gz
sudo sh sshguard-linux-amd64/deploy.sh
```

ARM64 将文件名中的 `amd64` 换成 `arm64`。发行包包含 Web 和全部 Agent，目标 Agent 只访问你的平台域名。

## 无交互部署 / MySQL 5.6

MySQL 兼容目标为 5.6.51 和 8.4，CI 使用这两种真实服务运行迁移与接口测试。向导不自动安装停止维护的 MySQL 5.6。先新建库和专用账号（密码自行替换）：

```sql
CREATE DATABASE sshguard CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;
CREATE USER 'sshguard'@'127.0.0.1' IDENTIFIED BY 'CHANGE_TO_A_LONG_RANDOM_PASSWORD';
GRANT ALL PRIVILEGES ON sshguard.* TO 'sshguard'@'127.0.0.1';
```

远程数据库的账户 Host 设为中央服务器实际来源地址，并限制网络访问；远程连接应使用经过验证的数据库 TLS。迁移需要建表/建索引权限，不需要全局管理员权限，不使用 root 作为应用账号。

```sh
cp sshguard-linux-amd64/deploy/deploy.conf.example deploy.conf
chmod 600 deploy.conf
# 编辑 DOMAIN、DB_DRIVER=mysql、DATABASE_URL、TLS_CERT/TLS_KEY。
# 原始 KEY=value，不加 shell 引号，不执行命令替换。
sudo sh sshguard-linux-amd64/deploy.sh --config "$(pwd)/deploy.conf"
```

DSN 示例：

```text
sshguard:YOUR_PASSWORD@tcp(127.0.0.1:3306)/sshguard?charset=utf8mb4&parseTime=true&loc=UTC
```

API 强制连接字符集 utf8mb4、时间解析和会话 UTC，并禁用多 SQL 语句执行。MySQL 5.6 不执行 CHECK 约束，业务字段校验由 API 强制执行。默认 767 字节 InnoDB 索引限制即可，不要求 `innodb_large_prefix`。切换数据库不会迁移数据。

MySQL 5.6 已于 2021 年结束生命周期；应用兼容不能修复数据库本身的漏洞。已有环境可使用此路径，新环境优先选择维护中的版本。[MySQL 官方说明](https://dev.mysql.com/blog-archive/5-6-goes-deep-six/)

## 更新中央平台

先备份数据库和 `/etc/sshguard`，阅读目标版本迁移说明，再下载新版本：

```sh
SSHGUARD_VERSION=v0.3.1 sh scripts/quick-start.sh --update
# 或使用已解压的新包：
sudo sh sshguard-linux-amd64/deploy.sh --update
```

更新保留配置、管理员、服务器、Agent 凭证和数据库。先检查数据库并迁移，再原子切换 `/opt/sshguard` 符号链接和重启 API；启动或健康检查失败会切回旧程序。旧版本保留在 `/opt/sshguard-releases/`，不自动删除。

**数据库迁移不会自动回退。** 本版本新增索引兼容旧程序；未来包含不兼容数据库变更时，应使用完整备份恢复。平台不远程执行 Agent 命令；升级平台后在「服务器管理 → 更新命令」复制到目标机器执行。

### 从 v0.2 手工部署迁入

旧 `/opt/sshguard` 是普通目录，向导不会覆盖。也可继续按 [原生文档](INSTALL-NATIVE.md) 手工升级。要迁入向导，先备份并安排停机，再执行（仅适用于文档中的标准路径）：

```sh
sudo systemctl stop sshguard-api
sudo mv /opt/sshguard /opt/sshguard-legacy-v02
sudo ln -s /opt/sshguard-legacy-v02 /opt/sshguard
sudo touch /etc/sshguard/deploy-managed
# 确保 api.env 中采用双引号且不带路径/端口：
# PUBLIC_URL="https://guard.example.com"
sudo sh sshguard-linux-amd64/deploy.sh --update
```

旧 Nginx 配置和证书保持原样，证书仍按原方式续期。迁入失败时可把 `/opt/sshguard` 符号链接指回保留的旧目录并重启服务。

## 运维

```sh
sudo sh /opt/sshguard/deploy.sh           # 已安装时打开管理菜单
sudo sh /opt/sshguard/deploy.sh --status  # 状态、版本、访问地址，不输出密码
sudo sh /opt/sshguard/deploy.sh --logs    # 最近 100 条日志
sudo sh /opt/sshguard/deploy.sh --restart
curl -fsS https://guard.example.com/api/v1/health
```

也可用 `sh scripts/quick-start.sh --status`（或 `--logs` / `--restart`），这些管理命令直接操作已安装服务，无需重新下载。管理菜单的更新只安装当前包；升级到新版本仍按上文下载并校验新包后执行 `--update`。

托管 Nginx 模式自动申请证书会配置 Certbot 续期和 reload hook；手动证书更新 `/etc/sshguard/certs/` 后 reload Nginx。独立 HTTPS 的证书位于 `/etc/sshguard/tls/`，续期后保持 root:sshguard-api 所有权和 0640 权限并重启 API。复用模式由现有面板管理证书。向导不调整防火墙，不覆盖其他虚拟主机。

数据库检查失败不会切换程序；首次安装中断会保留配置方便排错。残留锁目录仅在确认没有部署进程后删除。旧发行目录、备份和发行包按自己的策略清理。
