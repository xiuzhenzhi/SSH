# 原生部署

一键安装见 [快速开始](../guides/QUICKSTART.md)。以下为手工部署，需要已安装 Nginx、curl 和证书。

## 下载发布包

```sh
gh release download v0.3.15 --repo xiuzhenzhi/SSH --pattern sshguard-linux-amd64.tar.gz --pattern SHA256SUMS
sha256sum --ignore-missing -c SHA256SUMS
tar -xzf sshguard-linux-amd64.tar.gz
cd sshguard-linux-amd64
```

ARM64 将 `amd64` 换成 `arm64`。

## 准备数据库

SQLite 无需创建数据库。使用 PostgreSQL 或 MySQL 时，先创建专用库和账号。

PostgreSQL：

```sql
CREATE USER sshguard WITH PASSWORD 'REPLACE_DB_PASSWORD';
CREATE DATABASE sshguard OWNER sshguard;
```

MySQL：

```sql
CREATE DATABASE sshguard CHARACTER SET utf8mb4 COLLATE utf8mb4_bin;
CREATE USER 'sshguard'@'127.0.0.1' IDENTIFIED BY 'REPLACE_DB_PASSWORD';
GRANT ALL PRIVILEGES ON sshguard.* TO 'sshguard'@'127.0.0.1';
```

## 安装文件

首次安装执行：

```sh
id sshguard-api >/dev/null 2>&1 || sudo useradd --system --user-group --no-create-home --shell /usr/sbin/nologin sshguard-api
sudo install -d -m 755 /opt/sshguard /etc/sshguard
sudo install -m 755 sshguard-api /opt/sshguard/sshguard-api
sudo cp -a web /opt/sshguard/
sudo chmod -R a+rX /opt/sshguard/web
sudo install -m 600 deploy/api.env.example /etc/sshguard/api.env
sudo install -m 644 deploy/sshguard-api.service /etc/systemd/system/sshguard-api.service
sudo install -m 644 deploy/nginx.conf /etc/nginx/conf.d/sshguard.conf
sudo install -d -m 700 /etc/sshguard/certs
```

## 填写配置

编辑 `/etc/sshguard/api.env`：

| 配置 | 填写内容 |
|---|---|
| `DB_DRIVER` | `sqlite`、`postgres` 或 `mysql` |
| `DATABASE_URL` | 按下表填写 |
| `PUBLIC_URL` | 实际访问地址，如 `https://guard.example.com` |
| `LISTEN_ADDR` | 同机反代使用 `127.0.0.1:8080` |
| `ADMIN_PASSWORD` | 12–72 字节的初始密码 |
| `ENCRYPTION_KEY` | `openssl rand -hex 32` 生成，后续保留 |

| 数据库 | `DATABASE_URL` 示例 |
|---|---|
| SQLite | `file:/var/lib/sshguard-api/sshguard.db` |
| PostgreSQL | `postgres://sshguard:YOUR_PASSWORD@127.0.0.1:5432/sshguard?sslmode=disable` |
| MySQL | `sshguard:YOUR_PASSWORD@tcp(127.0.0.1:3306)/sshguard?parseTime=true&loc=UTC&charset=utf8mb4` |

编辑 Nginx 配置中的 `guard.example.com`，替换成自己的域名，安装证书后启动：

```sh
sudo install -m 644 /path/to/fullchain.pem /etc/sshguard/certs/fullchain.pem
sudo install -m 600 /path/to/privkey.pem /etc/sshguard/certs/privkey.pem
sudo systemctl daemon-reload
sudo systemctl enable --now sshguard-api
sudo nginx -t
sudo systemctl enable --now nginx
sudo systemctl reload nginx
curl -fsS https://guard.example.com/api/v1/health
```

已有宝塔或 OpenResty 站点使用 [反代配置](REVERSE-PROXY.md)。

## 更新与维护

手工部署时停止服务，替换 API、Web 和下载目录，保留环境配置后重启。自动更新器仅用于一键安装。

[Agent 接入](../guides/AGENTS.md) · [日志与备份](OPERATIONS.md)
