# 故障排查

> 本文v0.3.20 起使用新目录布局，旧版不自动迁移，请先使用旧版脚本卸载后再安装新版本。v0.3.19 用户请查阅 [对应版本文档](https://github.com/xiuzhenzhi/SSHGuard/tree/v0.3.19/docs)。

## 常见问题

| 问题 | 处理 |
|---|---|
| 旧菜单没有更新选项 | [拉取最新更新脚本](QUICKSTART.md#更新中央平台)，之后使用 `/opt/SSHGuard/Server/deploy.sh` |
| GitHub 下载失败或 404 | 检查网络和版本；私有仓库先执行 `gh auth login` |
| 登录或保存返回 403 | 检查 `PUBLIC_URL` 是否与浏览器的协议、主机、端口一致 |
| 密码错误 | 使用初始化时创建的账号；修改 `ADMIN_PASSWORD` 不会重置现有密码 |
| 登录返回 429 | 在「登录防护」检查封禁，或按 `Retry-After` 等待 |
| 安装凭证返回 401 | 重新生成安装命令，凭证可能已过期或使用 |
| Agent 心跳返回 401 | 检查凭证是否撤销，必要时重新注册 |
| TLS / x509 错误 | 检查证书域名、证书链、CA 包和系统时间 |
| Nginx 502 | 检查 API 服务及 `proxy_pass` 端口 |
| 反代初始化超时 | 将代理、CDN 或负载均衡超时设为至少 90 秒 |
| 自定义 HTTPS 端口不能登录 | 在 `PUBLIC_URL` 中填写完整外部端口，见 [端口配置](../deployment/REVERSE-PROXY.md#自定义端口提示) |
| 重新注册或主控地址变更后本地状态不匹配 | Agent 会自动清空旧状态并按新凭证采集，按 [地址迁移步骤](AGENTS.md#http-地址迁移到-https) 重新接入 |
| 提示 cannot initialize durable Agent state | 查看 Agent 日志或控制台采集异常详情中的具体原因，检查数据库占用、文件权限、磁盘空间或文件损坏 |
| 独立模式网页 404 | 检查程序旁的 `web/index.html` 或 `WEB_DIR` |
| MySQL 时间字段读取失败 | 在 DSN 中加入 `parseTime=true&loc=UTC` |
| 修改数据库密码后无法启动 | 先修改实际数据库账号密码，再同步 `DATABASE_URL` |
| Alpine 服务安装失败 | 确认 OpenRC 已运行，安装 `curl` 和 `ca-certificates` |
| Windows 服务安装失败 | 使用管理员 PowerShell，检查同名服务路径 |
| Windows 脚本被策略阻止 | 使用组织允许的签名或分发方式 |
| 页面没有 SSH 事件 | 检查采集状态和队列，测试一次真实 SSH 登录 |
| 出现重复事件计数 | 属于重传去重统计，不需要重新注册 |

## 查看日志

```sh
# 原生主控
journalctl -u sshguard-api -n 100 --no-pager
# Compose
docker compose logs --tail=100 api web
# Ubuntu / Debian Agent
journalctl -u sshguard-agent -n 100 --no-pager
# Alpine Agent
tail -n 100 /var/log/sshguard/agent.log
```

```powershell
Get-Service SSHGuardAgent
Get-WinEvent -FilterHashtable @{LogName='Application';ProviderName='SSHGuardAgent'} -MaxEvents 30
```

反馈问题时附上版本、系统、部署方式和脱敏日志。
