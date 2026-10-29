# Docker Compose 部署（可选）

先取得源码：

```bash
git clone https://github.com/xiuzhenzhi/SSH.git
cd SSH
```

## 方式 B：Docker Compose（可选）

需要 Docker Engine 与 Compose **2.24.4+**，Node.js 仅用于生成 `.env`。本地模式的端口覆盖使用 `!override`。

```bash
# 三条命令只选一条；默认使用 localhost 体验
node scripts/init-env.mjs sqlite http://localhost:8088
# node scripts/init-env.mjs postgres http://localhost:8088
# node scripts/init-env.mjs mysql http://localhost:8088
docker compose config --quiet
docker compose up -d --build
docker compose ps
curl -fsS http://localhost:8088/api/v1/health
```

打开 `http://localhost:8088`。初始账号 `admin`，随机密码见 `.env`。本地模式只允许从本机访问；本地 HTTP 不用于 Agent 安装。

公网 HTTPS 部署：在新的部署目录生成环境文件，再放置证书：

```bash
node scripts/init-env.mjs mysql https://guard.example.com
# PostgreSQL / SQLite 则把 mysql 换成 postgres / sqlite
mkdir -p deploy/certs
cp /path/to/fullchain.pem deploy/certs/fullchain.pem
cp /path/to/privkey.pem deploy/certs/privkey.pem
chmod 600 deploy/certs/privkey.pem
docker compose config --quiet
docker compose up -d --build
```

生成器拒绝覆盖已有 `.env`。从 localhost 切到 HTTPS，手动修改 `PUBLIC_URL` 并从 `COMPOSE_FILE` 去掉 `:compose.local.yaml`，放入证书后重建容器。`COMPOSE_FILE` 自动只启用选中的数据库，DB/API 不映射公网端口。数据库文件保存在命名卷；不要执行 `docker compose down -v`，它会删除数据卷。

**数据库选项用于新部署，不是数据迁移工具。** 已有数据不会因为修改 `DB_DRIVER` 自动迁入另一种数据库。切换需单独导出、转换和验证。Compose 镜像固定版本线，正式发布建议进一步锁定补丁和 digest。

SQLite 模式只启动 API 和 Web，数据保存在 `sqlite-data` 命名卷中的 `/var/lib/sshguard/sshguard.db`，使用本机磁盘和单个 API 实例。API 根文件系统仍为只读，仅数据卷可写。备份前先停止 API，保存卷内数据库及存在的 WAL/SHM 文件，同时备份 `.env` 中的加密密钥；恢复时停止 API 并保持数据目录归属容器用户（UID 10001）。


已有数据库卷不会因编辑环境变量重新初始化。修改数据库密码时，先修改实际数据库账号，再同步 DSN。

参阅 [Agent 安装](AGENTS.md)、[运维手册](OPERATIONS.md)。

## 更新

备份数据库卷和 .env 后更新源码，再执行 `docker compose up -d --build`。保留数据库、登录账号和 ENCRYPTION_KEY，不重新运行初始化器覆盖配置。反代须支持 WebSocket Upgrade，通知配置见 [告警手册](ALERTING.md)。
