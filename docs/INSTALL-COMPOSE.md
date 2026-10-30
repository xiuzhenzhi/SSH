# Docker Compose 部署

需要 Docker Engine、Compose 2.24.4+ 和 Node.js。

```sh
git clone https://github.com/xiuzhenzhi/SSH.git
cd SSH
```

## 本机体验

选择一种数据库生成配置：

```sh
node scripts/init-env.mjs sqlite http://localhost:8088
# 或将 sqlite 换成 postgres / mysql
docker compose config --quiet
docker compose up -d --build
docker compose ps
```

打开 `http://localhost:8088`。账号为 `admin`，密码见 `.env`。本机模式不用于 Agent 接入。

## 公网 HTTPS

在新的部署目录执行，替换域名与证书路径：

```sh
node scripts/init-env.mjs sqlite https://guard.example.com
mkdir -p deploy/certs
cp /path/to/fullchain.pem deploy/certs/fullchain.pem
cp /path/to/privkey.pem deploy/certs/privkey.pem
chmod 600 deploy/certs/privkey.pem
docker compose config --quiet
docker compose up -d --build
```

已有 `.env` 时直接编辑。从本机模式切换 HTTPS，还需从 `COMPOSE_FILE` 移除 `:compose.local.yaml`。

## 更新

备份 `.env` 和数据卷，在原项目目录执行：

```sh
git pull --ff-only
docker compose config --quiet
docker compose up -d --build
docker compose ps
```

保留原数据库类型和加密密钥；原生安装的 `update.sh` 不用于 Compose。

## 停止与数据

```sh
docker compose down
```

不要加 `-v`，否则会删除数据卷。备份与恢复见 [运维手册](OPERATIONS.md#备份与恢复)。
