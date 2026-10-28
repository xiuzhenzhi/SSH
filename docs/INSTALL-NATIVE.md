# 原生部署：Go 二进制 + systemd + Nginx

> 推荐先使用 [一键部署向导](QUICKSTART.md)，自动配置数据库、systemd、Nginx 和 HTTPS。下文保留手工部署方案。MySQL 可接入 5.6.51 或 8.4；新部署建议维护中的版本。

中央平台部署在 Linux，Agent 可运行在 Windows / Ubuntu / Debian / Alpine。

## 下载发布包

在 GitHub Releases 下载对应架构发布包和 SHA256SUMS，或从成功 CI 的 sshguard-release-assets 获取。私有仓库需要有访问权限的账号。

```bash
gh release download v0.3.0-alpha.1 --repo xiuzhenzhi/SSH --pattern sshguard-linux-amd64.tar.gz --pattern SHA256SUMS
sha256sum --ignore-missing -c SHA256SUMS
tar -xzf sshguard-linux-amd64.tar.gz
cd sshguard-linux-amd64
```

ARM64 中央服务器把 amd64 改成 arm64。包内已有 API、Web 和全部 Agent，运行时无需 Go/Node.js。源码构建见 [开发文档](DEVELOPMENT.md)。

### 2. 准备数据库（二选一）

先安装数据库，或使用已有实例。下面在数据库管理员会话中执行，把密码占位符换成随机密码；建议密码只使用十六进制字符，避免 DSN/环境文件转义问题。

PostgreSQL：

```sql
CREATE USER sshguard WITH PASSWORD 'REPLACE_DB_PASSWORD';
CREATE DATABASE sshguard OWNER sshguard;
```

MySQL 8.4：

```sql
CREATE DATABASE sshguard CHARACTER SET utf8mb4 COLLATE utf8mb4_bin;
CREATE USER 'sshguard'@'127.0.0.1' IDENTIFIED BY 'REPLACE_DB_PASSWORD';
GRANT ALL PRIVILEGES ON sshguard.* TO 'sshguard'@'127.0.0.1';
```

MySQL 用户 host 须匹配实际 API 来源。请使用 UTC 数据库时区，并保证 DSN 含 `parseTime=true&loc=UTC`。时间由 API 以 UTC 写入，Web 使用浏览器时区展示。远程数据库请按实际证书配置 PostgreSQL `sslmode=verify-full` 或 MySQL `tls=true`；示例的明文数据库连接只用于同机/隔离容器网络。

### 3. 安装原生服务

以下在 Linux 发布目录中执行。先确保已安装 nginx、curl、ca-certificates；Debian/Ubuntu 可用 apt，Rocky/AlmaLinux 可用 dnf。不要覆盖正在使用的其他 Nginx 虚拟主机配置。

```bash
id sshguard-api >/dev/null 2>&1 || sudo useradd --system --user-group --no-create-home --shell /usr/sbin/nologin sshguard-api
sudo install -d -m 755 /opt/sshguard /etc/sshguard
sudo install -m 755 sshguard-api /opt/sshguard/sshguard-api
sudo cp -a web /opt/sshguard/
sudo chmod -R a+rX /opt/sshguard/web
# 首次部署才复制环境模板；升级时保留原环境配置
sudo install -m 600 deploy/api.env.example /etc/sshguard/api.env
sudo install -m 644 deploy/sshguard-api.service /etc/systemd/system/sshguard-api.service
sudo install -m 644 deploy/nginx.conf /etc/nginx/conf.d/sshguard.conf
sudo install -d -m 700 /etc/sshguard/certs
```

编辑 `/etc/sshguard/api.env`：选择 `DB_DRIVER=postgres` 或 `mysql`，填好 `DATABASE_URL`、真实 `PUBLIC_URL=https://guard.example.com`、随机 `ADMIN_PASSWORD`（12–72 字节）。`LISTEN_ADDR=127.0.0.1:8080` 只绑定回环。环境模板展示了两种 DSN。

编辑 `/etc/nginx/conf.d/sshguard.conf`，把两处 `guard.example.com` 换成真实域名。将可信 CA 签发的证书链及私钥安装到对应路径：

```bash
sudo install -m 644 /path/to/fullchain.pem /etc/sshguard/certs/fullchain.pem
sudo install -m 600 /path/to/privkey.pem /etc/sshguard/certs/privkey.pem
sudo systemctl daemon-reload
sudo systemctl enable --now sshguard-api
sudo nginx -t
sudo systemctl enable --now nginx
sudo systemctl reload nginx
curl -fsS https://guard.example.com/api/v1/health
```

需将域名 DNS 指向中央服务器，开放 80/443。手工部署的证书由自己的 ACME/证书流程续期，之后执行 `nginx -t && systemctl reload nginx`；[一键向导](QUICKSTART.md) 则可选 Certbot 自动申请及续期。API 使用独立 `sshguard-api` 账号，与 Agent 的 `sshguard` 账号隔离。启用 SELinux 的系统需由管理员为 Web 目录配置合适标签并允许 Nginx 连接本机 API，不要关闭 SELinux。

查看运行情况：

```bash
sudo systemctl status sshguard-api
sudo journalctl -u sshguard-api -n 100 --no-pager
```


## 接入与维护

登录 PUBLIC_URL 后按 [Agent 安装](AGENTS.md) 接入服务器，日常升级和备份见 [运维手册](OPERATIONS.md)。
