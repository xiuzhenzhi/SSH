# 开发与测试

Go 版本以 go.mod 为准；Node.js 24 LTS；PostgreSQL 16 或 MySQL 8.4。依赖使用 go.sum / package-lock.json 锁定。

```bash
git clone https://github.com/xiuzhenzhi/SSH.git
cd SSH
go mod download
```

## 本地源码开发（无需 Docker）

先创建本地数据库。两个终端分别执行：

```powershell
# 终端 1：Windows PowerShell，PostgreSQL 示例
$env:DB_DRIVER='postgres'
$env:DATABASE_URL='postgres://sshguard:YOUR_PASSWORD@127.0.0.1:5432/sshguard?sslmode=disable'
$env:PUBLIC_URL='http://localhost:5173'
$env:LISTEN_ADDR='127.0.0.1:8080'
$env:ADMIN_PASSWORD='REPLACE_WITH_RANDOM_12_TO_72_BYTE_PASSWORD'
go run ./cmd/api
```

```powershell
# 终端 2
Set-Location web
npm.cmd ci
npm.cmd run dev
```

打开 `http://localhost:5173`（须与 `PUBLIC_URL` 完全一致，包括主机名和端口）。Vite 将 API 请求代理到 8080。`.env` 由 Compose 自动读取；直接运行 Go 时通过 shell 环境或 systemd 环境文件注入，不自动读取 `.env`。


## 构建与检查

```bash
# 仅构建四个 Agent
node scripts/build-agents.mjs
node scripts/check-agent-artifacts.mjs dist/agents
# 中央平台构建（包含 Web 和全部 Agent）
node scripts/build-release.mjs amd64
node scripts/build-release.mjs arm64 --reuse-web
node scripts/package-release.mjs
# 代码与依赖检查
go test ./...
go vet ./...
go run golang.org/x/vuln/cmd/govulncheck@latest ./...
cd web
npm ci
npm run build
npm audit
```

--reuse-web 仅可在 web/dist 已成功构建且前端未改动时使用。Windows 下构建前停止本项目 Vite，避免 npm ci 替换 esbuild 时文件被占用。

真实数据库测试必须使用独立测试库（写入测试账号/资产，不删库）；未设置 DSN 会显式跳过：

```bash
TEST_DB_DRIVER=postgres TEST_DATABASE_URL='postgres://USER:PASS@127.0.0.1:5432/sshguard_test?sslmode=disable' go test ./internal/api -run TestIntegration -count=1 -v
TEST_DB_DRIVER=mysql TEST_DATABASE_URL='USER:PASS@tcp(127.0.0.1:3306)/sshguard_test?parseTime=true&loc=UTC&charset=utf8mb4' go test ./internal/api -run TestIntegration -count=1 -v
```

Windows ACL helper 测试使用 scripts/test-windows-installer.ps1，只在工作区创建目录，不注册服务。测试局限见 [验证记录](VALIDATION.md)。

CI 另有临时机器专用的完整安装测试：Windows 的 `scripts/test-windows-service.ps1` 注册真实 SCM 服务；Linux 的 `scripts/test-native-deploy.sh` 安装 PostgreSQL/Nginx/systemd 服务并启动 Alpine OpenRC 测试容器。这些脚本会修改固定系统路径，禁止在业务主机运行。工作流还执行固定工具版本的 govulncheck；不会自动忽略可达漏洞。
