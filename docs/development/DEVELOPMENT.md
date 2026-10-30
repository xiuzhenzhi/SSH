# 开发与测试

需要 Go（版本见 `go.mod`）和 Node.js 24。

```sh
git clone https://github.com/xiuzhenzhi/SSHGuard.git
cd SSHGuard
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
node --test scripts/tests/ci-plan.test.mjs scripts/tests/release-version.test.mjs scripts/tests/prepare-release.test.mjs scripts/tests/quick-start.test.mjs
node scripts/checks/check-docs.mjs
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

Agent 性能与兼容性检查：

```sh
go test -tags=nethttpomithttp2 ./internal/agent
go test ./internal/agent -run '^$' -bench 'Benchmark(ParseSSH|Spool|Idle)' -benchmem
go test ./internal/agent -run '^$' -fuzz '^FuzzSSHParserCompatibility$' -fuzztime=10s
```

发行版 Agent 通过 `scripts/build/build-agents.mjs` 构建，保留静态 Linux / Windows 的 amd64、arm64 支持，使用 `nethttpomithttp2` 裁掉未使用的 HTTP/2 代码；HTTPS 和 WebSocket 使用 HTTP/1.1。比较占用时应使用相同配置、队列规模及日志负载，并计入日志读取子进程；二进制大小、常驻内存和 CPU 分别记录。

```sh
node scripts/build/build-release.mjs amd64
node scripts/build/build-release.mjs arm64 --reuse-web --reuse-agents
node scripts/build/package-release.mjs
```

真实服务安装测试只在一次性测试机器运行，脚本会修改系统服务与固定路径。

[发布流程](RELEASING.md) · [文档站开发](DOCS-SITE.md)

## 目录约定

- `cmd/`：仅保留主控与 Agent 的启动入口；运行与采集逻辑分别在 `internal/server/`、`internal/agent/`。
- `internal/`：后端 API、存储、通知及公共内部逻辑。
- `internal/storage/migrations/`：按数据库分类的迁移，随 Go 程序打包。
- `web/src/pages/`：按需加载的页面；`components/`：公共组件；`lib/`：请求、路由与工具；`styles/`：样式。
- `deploy/`：容器、反代及系统服务配置。
- `scripts/build/`：构建与打包；`release/`：版本发布；`checks/`：静态检查；`tests/`：回归与安装测试；`dev/`：开发环境工具。
- `docs/`：按 guides、deployment、development 分类，仅用于源码开发与在线文档站，不随运行包安装。

主控唯一管理入口为根目录 `sshguard.sh`，包含安装、下载更新、服务管理与卸载。`--deploy` 用于部署已校验的本地发行包；普通 `--update` 下载最新正式版。`scripts/` 下的主控脚本是 当前发行包的管理入口，统一转交 `sshguard.sh`。Agent 安装脚本继续按 Linux / Windows 分开，因为执行环境及服务管理不同。

前端启动先确认初始化与会话状态，随后显示页面；总览数据及验证码配置不阻塞全屏启动。读取请求有 10 秒超时，写入请求有 30 秒超时，保留页面切换取消能力，连接失败可重试。

### 不下载文档的源码检出

普通 `git clone` 包含文档。只需要程序源码时可使用稀疏检出：

```sh
git clone --filter=blob:none --no-checkout https://github.com/xiuzhenzhi/SSHGuard.git
cd SSHGuard
git sparse-checkout set --cone cmd internal web deploy scripts .github
git checkout main
```

这保留根目录文件及选定代码目录，不检出 `docs/`。需要编辑文档时运行 `git sparse-checkout add docs`。Windows 源码打包脚本默认也不包含文档，完整开发归档可传入 `-IncludeDocs`。

菜单使用按路径区分的短时内存缓存，15 秒内复用结果；切换时可先显示两分钟内的旧数据并刷新。缓存最多 40 项，退出登录、写入和实时通知会清空；不会写入浏览器持久存储。动画保持启用。

## CI 范围

提交会按主控、数据库、Agent、前端、部署和文档分类，多个模块取并集；共用依赖和数据库访问接口会保留必要的联动验证。修改范围规则时需运行 `node --test scripts/tests/ci-plan.test.mjs scripts/tests/reuse-web.test.mjs`。具体规则与产物复用见[构建与发布](RELEASING.md#按改动范围检查与构建)。
