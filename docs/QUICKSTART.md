# 一键部署、更新与回退

中央平台支持 Ubuntu / Debian + systemd，amd64 / arm64；运行时不需要 Go、Node.js 或 Docker。Agent 支持的系统见 [Agent 文档](AGENTS.md)。

## 一键入口

独立运行可直接使用服务器 IPv4 和应用端口（默认 8080），无需 Nginx。使用域名和已有反向代理时，将域名解析到中央服务器并开放 443；只有自动申请证书才需要开放 80。数据库端口无需对公网开放。私有仓库先在当前普通用户下执行 `gh auth login`，账号需要仓库读取权限。

```sh
git clone https://github.com/xiuzhenzhi/SSH.git
cd SSH
sh scripts/quick-start.sh
```

首次安装时，最后一条命令下载固定版本 `v0.3.11` 的对应架构发行包，校验 SHA256，再调用安装向导；已有安装选「9」则查询最新正式版。下载使用当前用户的 GitHub 登录，只有安装时调用 sudo。不要把 GitHub Token 放进命令行或发给 Agent。

运行安装脚本后进入编号菜单，首次安装选择「1 安装主控」，输入端口（回车使用 8080）即可；已有安装选择「9 更新主控（下载最新正式版）」，回车默认选「9」。也可使用 `sh scripts/quick-start.sh --port 8081` 安装。已有 Nginx / OpenResty 不影响此流程。首次交互安装显示初始化链接，打开后创建管理员，随后自动登录并进入服务器管理。默认使用 SQLite，无需额外数据库服务；仅在需要时展开「存储设置」选择 MySQL / PostgreSQL，并预先创建专用空数据库。初始化口令通过链接片段传递，不进入 HTTP 访问日志；同一标签页刷新时保留到初始化完成，完成后清除。

忘记链接时，在主控终端执行 `sudo sh /opt/sshguard/deploy.sh --setup-link`，或菜单选择「8 查看初始化链接」。口令只在交互终端显示，CI/重定向日志不会包含。初始化已完成时不会重新开放入口。

添加服务器后会自动显示安装命令。通过 Agent 能访问的主控 IP 和端口打开控制台，选择目标操作系统并复制命令，无需另填域名。放行主控端口（默认 TCP 8080）；NAT 环境需使用公网 IP 和映射端口，Agent 无需开放入站端口。

网页初始化配置存放在 `/var/lib/sshguard-api/setup.json`，与数据库及 `/etc/sshguard/api.env` 一起备份。初始化完成后入口关闭；已有安装升级不会重新创建管理员或切换数据库。显式 `--config` 的无人值守安装仍按配置文件初始化数据库和管理员，无需网页操作。

### 卸载中央平台

运行管理菜单选择「7 卸载中央平台」，或 `sudo sh /opt/sshguard/deploy.sh --uninstall`。确认后停止并移除中央服务与当前程序，保留数据库、配置和历史目录，不卸载 Nginx/OpenResty、外部数据库服务或 Agent。

独立卸载脚本为 `scripts/uninstall.sh`，支持标准 `/opt/sshguard` 安装路径。无人值守可加 `--yes`，非标准服务路径会停止自动卸载。反代条目在原面板中自行移除；Compose 部署在项目目录执行 `docker compose down`，不加 `-v` 以保留数据卷。

再次运行安装可沿用保留的配置与数据；要全新初始化，请先完成备份并由管理员明确处理旧数据，脚本不会自动清空数据库。

下面的接入方式与证书仅用于显式指定 `--proxy external` / `--proxy managed` 或配置文件中的 TLS 参数，不属于默认安装必填项：

1. 默认独立运行。反向代理是可选的后续配置，不根据已有 Web 服务自动切换安装模式。
2. 默认无需填写域名或 IP；安装后使用实际 IP 和端口打开网页。内置 HTTPS 通过配置文件提供 DOMAIN、TLS_CERT 和 TLS_KEY。
3. 数据库：默认 `sqlite`，无需 DSN；`local-postgres` 自动安装本机 PostgreSQL；`mysql` / `postgres` 连接已有数据库，DSN 隐藏输入。
4. 只有托管 Nginx 模式才配置 Nginx 证书，支持已有证书或 Certbot/Let's Encrypt（接受 ACME 服务条款，公网 80 端口可达）。复用模式沿用现有站点证书。

向导仅安装缺少的依赖、初始化数据库、创建低权限服务账号、配置 systemd，最后检查 API 健康状态。独立和复用模式不会安装或重载 Nginx / OpenResty。初始账号 `admin`，随机密码仅保存在 `/etc/sshguard/api.env`，不会输出到部署日志。登录后点右上角「修改密码」，旧会话会全部失效。

独立模式访问 `http://服务器IP:8080`，也可按需启用内置 TLS。默认安装显式开启 `ALLOW_INSECURE_HTTP=true`，生成的 Agent 命令包含 HTTP 授权参数并在本地配置中保存，因此注册、心跳、事件与更新都能使用同一 IP 地址。HTTP 的登录、凭证与上报未加密，应确保网络可信；SHA256 只能检查完整性。已有 HTTPS 部署继续校验证书，不会降级。程序直接提供发布包中的 `web/`，无需 Node.js 或单独的 Web 服务。

复用 Nginx / OpenResty / 宝塔时选择 `--proxy external`，在已有 HTTPS 站点把整个应用反代到 `http://127.0.0.1:8080`（或自选 APP_PORT），并启用 WebSocket。参考生成的 `/etc/sshguard/nginx-existing.conf`，将 location 配置合并进现有 server 块，避免重复的 `location /`；由你自己的面板验证并重载配置。无需再配置静态网页目录。不要为了安装本项目停止已有网站或安装第二套 Nginx。

更新会保留接入方式、端口、数据库和加密密钥。切换接入方式需手动迁移配置，不能仅通过更新参数切换。

SHA256 用于检查下载完整性；发行包与校验文件来自同一个受信任仓库，它不等同于独立数字签名。

安装器默认使用 SQLite。向导将数据库放在 `/var/lib/sshguard-api/sshguard.db`，由 systemd 创建权限为 0700 的专用目录；升级保留数据库和加密密钥。无人值守部署设置 `DB_DRIVER=sqlite` 并省略 `DATABASE_URL`，其他域名和证书配置与下述示例相同。使用本机磁盘，备份前停止 API，同时保存数据库文件、存在的 WAL/SHM 文件和 `/etc/sshguard/api.env`。

## 离线包 / 不在服务器保存 GitHub 登录

### 可选：安装后使用 Nginx / OpenResty / 宝塔反代

完整的接入、Agent 迁移和排查步骤见 [反代与 HTTPS](REVERSE-PROXY.md)。使用自定义外部端口时，手动同步 `PUBLIC_URL`、站点监听和安全组；例如 `https://guard.example.com:8443`。模板不会自动改端口，内部 `proxy_pass` 仍填写主控监听地址。

先按默认方式安装，再在现有 HTTPS 网站中反代整个应用，无需设置网站根目录。宝塔用户进入「网站 → 对应站点 → 反向代理 → 配置文件」，将该反代条目的原内容整体替换为下面配置，再保存；不要包裹 `server {}`。直接管理 Nginx/OpenResty 配置时，放在已有 server 块内并替换重复的根 location。端口不为 8080 时同步修改 proxy_pass。

```nginx
#PROXY-START/
location ^~ / {
    proxy_pass http://127.0.0.1:8080;
    proxy_http_version 1.1;
    proxy_set_header Host $http_host;
    proxy_set_header X-Real-IP $remote_addr;
    proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
    proxy_set_header X-Forwarded-Proto $scheme;
    proxy_set_header Upgrade $http_upgrade;
    proxy_set_header Connection "upgrade";
    proxy_read_timeout 90s;
    proxy_connect_timeout 10s;
    proxy_send_timeout 90s;
    proxy_buffering off;
    proxy_cache off;
    client_max_body_size 1m;
}
#PROXY-END/
```

然后在 `/etc/sshguard/api.env` 设置 `PUBLIC_URL="https://你的域名"`，同机反代时将 `LISTEN_ADDR` 改为 `127.0.0.1:8080` 限制直连，并将 `ALLOW_INSECURE_HTTP` 设为 `false`；迁移已有 HTTP Agent 前先阅读上面的迁移说明，再执行 `sudo systemctl restart sshguard-api`。证书与 Web 服务继续由原面板管理。通过自己的面板验证并加载反代配置后访问域名，Agent 安装和平台更新也会使用这个 HTTPS 地址。默认直连 HTTP 不填写 PUBLIC_URL，应用会按实际请求地址进行同源校验，并生成同一地址的 Agent 命令，不信任客户端伪造的转发头。

### 下载并校验离线包

在有权限的机器下载发行包和 `SHA256SUMS`，传到中央服务器：

```sh
awk '$2=="sshguard-linux-amd64.tar.gz" {print}' SHA256SUMS | sha256sum -c -
tar -xzf sshguard-linux-amd64.tar.gz
sudo sh sshguard-linux-amd64/deploy.sh
```

ARM64 将文件名中的 `amd64` 换成 `arm64`。发行包包含 Web 和全部 Agent，目标 Agent 只访问你的主控 IP 或域名。

## 无交互部署 / MySQL 5.6

MySQL 支持 5.6.51 和 8.4，使用已有数据库服务。先新建库和专用账号（密码自行替换）：

```sql
CREATE DATABASE sshguard CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;
CREATE USER 'sshguard'@'127.0.0.1' IDENTIFIED BY 'CHANGE_TO_A_LONG_RANDOM_PASSWORD';
GRANT ALL PRIVILEGES ON sshguard.* TO 'sshguard'@'127.0.0.1';
```

远程数据库的账户 Host 设为中央服务器实际来源地址，并限制网络访问；远程连接应使用经过验证的数据库 TLS。迁移需要建表/建索引权限，不需要全局管理员权限，不使用 root 作为应用账号。

```sh
cp sshguard-linux-amd64/deploy/deploy.conf.example deploy.conf
chmod 600 deploy.conf
# 按需编辑 DB_DRIVER=mysql、DATABASE_URL；域名和 TLS 参数可选。
# 原始 KEY=value，不加 shell 引号，不执行命令替换。
sudo sh sshguard-linux-amd64/deploy.sh --config "$(pwd)/deploy.conf"
```

DSN 示例：

```text
sshguard:YOUR_PASSWORD@tcp(127.0.0.1:3306)/sshguard?charset=utf8mb4&parseTime=true&loc=UTC
```

API 强制连接字符集 utf8mb4、时间解析和会话 UTC，并禁用多 SQL 语句执行。MySQL 5.6 不执行 CHECK 约束，业务字段校验由 API 强制执行。默认 767 字节 InnoDB 索引限制即可，不要求 `innodb_large_prefix`。切换数据库不会迁移数据。

数据库服务由管理员独立维护、更新和备份。

## 更新中央平台

先备份数据库、`/var/lib/sshguard-api/setup.json`（如有）和 `/etc/sshguard`，阅读目标版本迁移说明。在主控执行一条命令，自动查询最新正式 Release、下载当前架构的发行包、校验 SHA256 和包内版本后更新，无需先 `git pull` 或手动解压：

```sh
sh /opt/sshguard/update.sh
# 源码目录中的等效入口（旧安装还没有 update.sh 时也可用）：
sh scripts/update.sh
# 兼容原命令，同样默认更新到最新正式版：
sh scripts/quick-start.sh --update
```

也可以运行 `sudo sh /opt/sshguard/deploy.sh`，选择「9 更新主控」；在解压目录运行 `sudo sh deploy.sh` 选择「9」同样会下载新版，不会仅重装当前包。「1」只负责安装，已有安装会提示使用更新选项。非交互入口为 `sudo sh deploy.sh --update-latest`。

私有仓库先以运行更新入口的用户执行 `gh auth login`。推荐普通用户运行 `update.sh`，下载使用该用户的 GitHub 登录，验证完成后才通过 sudo 部署；若在 root 管理菜单中更新，root 也需要仓库读取权限。公开仓库可直接通过 curl 下载。查询或下载失败会停止更新，不会使用脚本内的旧版本作为替代。

指定版本或离线更新：

```sh
SSHGUARD_VERSION=v0.3.11 sh /opt/sshguard/update.sh
# 离线包先按上文校验 SHA256 并解压，再从新包目录执行：
sudo sh sshguard-linux-amd64/deploy.sh --update
```

默认只跟随最新正式版，预发布需通过 `SSHGUARD_VERSION` 显式指定。离线更新从已解压的新包运行 `deploy.sh --update`，不访问 GitHub；从已安装的 `/opt/sshguard/deploy.sh --update` 运行则下载最新正式版。手工部署的旧 `/opt/sshguard` 目录需要先按迁移流程处理，自动更新仅支持向导管理的安装。

### 拉取最新更新脚本（旧安装也可用）

如果旧菜单仍显示「安装 / 更新主控」、没有「9」，或服务器上的脚本尚未更新，在主控终端执行下面一条命令。它直接读取仓库 `main` 的最新自包含脚本，无需先下载新发行包，也不依赖本地 `update.sh` 或 `deploy.sh`：

```sh
curl --proto '=https' --proto-redir '=https' --tlsv1.2 -fLsS https://raw.githubusercontent.com/xiuzhenzhi/SSH/main/scripts/quick-start.sh -o sshguard-update.sh && sh sshguard-update.sh --update
```

只拉取脚本、暂不执行时：

```sh
curl --proto '=https' --proto-redir '=https' --tlsv1.2 -fLsS https://raw.githubusercontent.com/xiuzhenzhi/SSH/main/scripts/quick-start.sh -o sshguard-update.sh
```

之后运行 `sh sshguard-update.sh --update`。每次重新执行下载命令即可刷新本地这份更新入口；该文件可单独运行，更新时不会寻找旧目录里的辅助脚本。仅使用主控支持的 Ubuntu / Debian + systemd；普通用户运行时最终部署会调用 sudo。

脚本版本跟随 `main`，主控 API、Web、文档和 Agent 下载文件仍取自正式 Release，并校验 SHA256 与包内版本。源码推送不会替换已有 Release 或自动改变服务器上的文件；新版完整菜单和文档随发行包更新到 `/opt/sshguard`。原先在宝塔站点目录解压的旧副本不自动覆盖，日后应使用 `/opt/sshguard` 下的入口，或重新拉取上述脚本。

更新保留配置、端口、管理员、服务器、Agent 凭证和数据库，不重新初始化。下载、校验和程序预检查完成前不会停止当前 API；先检查数据库并迁移，再原子切换 `/opt/sshguard` 符号链接和重启 API，启动或健康检查失败会恢复更新前的程序。程序备份保留在 `/opt/sshguard-releases/`，不自动删除。

**数据库迁移不会自动回退。** 数据库结构变更后的恢复使用完整备份。Agent 可在「服务器管理」发起平台更新，或复制更新命令到目标机器执行。


## 运维

```sh
sudo sh /opt/sshguard/deploy.sh           # 已安装时打开管理菜单
sudo sh /opt/sshguard/deploy.sh --status  # 状态、版本、访问地址，不输出密码
sudo sh /opt/sshguard/deploy.sh --logs    # 最近 100 条日志
sudo sh /opt/sshguard/deploy.sh --restart
curl -fsS https://guard.example.com/api/v1/health
```

也可用 `sh scripts/quick-start.sh --status`（或 `--logs` / `--restart`），这些管理命令直接操作已安装服务，无需重新下载。管理菜单的「9 更新主控」会下载最新正式版，独立更新入口为 `sh /opt/sshguard/update.sh`。

托管 Nginx 模式自动申请证书会配置 Certbot 续期和 reload hook；手动证书更新 `/etc/sshguard/certs/` 后 reload Nginx。独立 HTTPS 的证书位于 `/etc/sshguard/tls/`，续期后保持 root:sshguard-api 所有权和 0640 权限并重启 API。复用模式由现有面板管理证书。向导不调整防火墙，不覆盖其他虚拟主机。

数据库检查失败不会切换程序；首次安装中断会保留配置方便排错。残留锁目录仅在确认没有部署进程后删除。旧发行目录、备份和发行包按自己的策略清理。
