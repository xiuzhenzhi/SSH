# Docker Compose 部署（可选）

先取得源码：

```bash
git clone https://github.com/xiuzhenzhi/SSH.git
cd SSH
```

## 方式 B：Docker Compose（可选）

需要 Docker Engine 与 Compose **2.24.4+**，Node.js 仅用于生成 `.env`。本地模式的端口覆盖使用 `!override`。

```bash
# 两条命令只选一条；默认使用 localhost 体验
node scripts/init-env.mjs postgres http://localhost:8088
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
# PostgreSQL 则把 mysql 换成 postgres
mkdir -p deploy/certs
cp /path/to/fullchain.pem deploy/certs/fullchain.pem
cp /path/to/privkey.pem deploy/certs/privkey.pem
chmod 600 deploy/certs/privkey.pem
docker compose config --quiet
docker compose up -d --build
```

生成器拒绝覆盖已有 `.env`。从 localhost 切到 HTTPS，手动修改 `PUBLIC_URL` 并从 `COMPOSE_FILE` 去掉 `:compose.local.yaml`，放入证书后重建容器。`COMPOSE_FILE` 自动只启用选中的数据库，DB/API 不映射公网端口。数据库文件保存在命名卷；不要执行 `docker compose down -v`，它会删除数据卷。

**二选一指新部署选择数据库，不是数据迁移工具。** 已有 PostgreSQL 数据不会因为修改 `DB_DRIVER` 自动迁入 MySQL，反之亦然。已有数据切换需单独导出、转换和验证。Compose 镜像固定版本线，正式发布建议进一步锁定补丁和 digest。


已有数据库卷不会因编辑环境变量重新初始化。修改数据库密码时，先修改实际数据库账号，再同步 DSN。

参阅 [Agent 安装](AGENTS.md)、[运维手册](OPERATIONS.md)。
