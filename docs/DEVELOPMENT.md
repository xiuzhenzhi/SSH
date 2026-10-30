# 开发与测试

需要 Go（版本见 `go.mod`）和 Node.js 24。

```sh
git clone https://github.com/xiuzhenzhi/SSH.git
cd SSH
go mod download
```

## 本地运行

先创建开发数据库。终端一启动 API，以下为 PowerShell 示例：

```powershell
$env:DB_DRIVER='postgres'
$env:DATABASE_URL='postgres://sshguard:YOUR_PASSWORD@127.0.0.1:5432/sshguard?sslmode=disable'
$env:PUBLIC_URL='http://localhost:5173'
$env:LISTEN_ADDR='127.0.0.1:8080'
$env:ADMIN_PASSWORD='REPLACE_WITH_RANDOM_12_TO_72_BYTE_PASSWORD'
$env:ENCRYPTION_KEY=(node -e "process.stdout.write(require('crypto').randomBytes(32).toString('hex'))")
go run ./cmd/api
```

加密密钥首次生成后保存，后续启动沿用。直接运行 API 时通过环境变量配置，不自动读取 `.env`。

终端二启动前端：

```powershell
Set-Location web
npm.cmd ci
npm.cmd run dev
```

打开 `http://localhost:5173`。

## 检查

```sh
go test ./...
go vet ./...
node --test scripts/ci-plan.test.mjs scripts/release-version.test.mjs scripts/prepare-release.test.mjs scripts/quick-start.test.mjs
node scripts/check-docs.mjs
cd web
npm ci
npm run build
npm audit
```

未设置测试 DSN 时使用临时 SQLite。外部数据库测试使用独立测试库：

```sh
TEST_DB_DRIVER=postgres TEST_DATABASE_URL='postgres://USER:PASS@127.0.0.1:5432/sshguard_test?sslmode=disable' go test ./internal/api -run TestIntegration -count=1 -v
TEST_DB_DRIVER=mysql TEST_DATABASE_URL='USER:PASS@tcp(127.0.0.1:3306)/sshguard_test?parseTime=true&loc=UTC&charset=utf8mb4' go test ./internal/api -run TestIntegration -count=1 -v
```

## 构建

```sh
node scripts/build-release.mjs amd64
node scripts/build-release.mjs arm64 --reuse-web --reuse-agents
node scripts/package-release.mjs
```

真实服务安装测试只在一次性测试机器运行，脚本会修改系统服务与固定路径。

[发布流程](RELEASING.md) · [文档站开发](DOCS-SITE.md)
