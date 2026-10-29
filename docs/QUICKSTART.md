# 一键部署、更新与回退

中央平台支持 Ubuntu / Debian + systemd，amd64 / arm64；运行时不需要 Go、Node.js 或 Docker。Agent 支持的系统见 [Agent 文档](AGENTS.md)。

## 一键入口

先将公网域名解析到中央服务器，开放 80/443。数据库端口无需对公网开放。私有仓库先在当前普通用户下执行 `gh auth login`，账号需要仓库读取权限。

```sh
git clone https://github.com/xiuzhenzhi/SSH.git
cd SSH
sh scripts/quick-start.sh
```

最后一条命令下载固定版本 `v0.3.0` 的对应架构发行包，校验 SHA256，再调用安装向导。下载使用当前用户的 GitHub 登录，只有安装时调用 sudo。不要把 GitHub Token 放进命令行或发给 Agent。

向导询问：

1. 公网域名，例如 `guard.example.com`。
2. 数据库：`sqlite` 使用本机文件数据库；`local-postgres` 自动安装本机 PostgreSQL；`mysql` / `postgres` 连接已建好的数据库。
3. 外部数据库 DSN（输入隐藏）；本机 PostgreSQL 自动生成随机密码；SQLite 无需填写 DSN。
4. TLS：填写已有证书和私钥路径；或者留空，填写邮箱，由 Certbot/Let's Encrypt 自动申请。此选项接受 ACME 服务条款，域名必须指向本机且公网可访问 80 端口。

向导安装依赖、初始化数据库、创建低权限服务账号、配置 systemd / Nginx，最后检查 API 健康状态。初始账号 `admin`，随机密码仅保存在 `/etc/sshguard/api.env`，不会输出到部署日志。登录后点右上角「修改密码」，旧会话会全部失效。

SHA256 用于检查下载完整性；发行包与校验文件来自同一个受信任仓库，它不等同于独立数字签名。

SQLite 安装支持需要包含该功能的新发行包，固定版本下载不会使用源码目录中的未发布修改。向导将数据库放在 `/var/lib/sshguard-api/sshguard.db`，由 systemd 创建权限为 0700 的专用目录；升级保留数据库和加密密钥。无人值守部署设置 `DB_DRIVER=sqlite` 并省略 `DATABASE_URL`，其他域名和证书配置与下述示例相同。使用本机磁盘，备份前停止 API，同时保存数据库文件、存在的 WAL/SHM 文件和 `/etc/sshguard/api.env`。

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
SSHGUARD_VERSION=v0.3.0 sh scripts/quick-start.sh --update
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
sudo systemctl status sshguard-api nginx
sudo journalctl -u sshguard-api -n 100 --no-pager
curl -fsS https://guard.example.com/api/v1/health
```

自动申请证书会配置 Certbot 续期和 Nginx reload hook，保留 80 端口 ACME 验证路径。使用已有证书时向导复制证书，以后需自行更新 `/etc/sshguard/certs/` 并 reload Nginx。向导不调整防火墙，不覆盖其他虚拟主机。

数据库检查失败不会切换程序；首次安装中断会保留配置方便排错。残留锁目录仅在确认没有部署进程后删除。旧发行目录、备份和发行包按自己的策略清理。
