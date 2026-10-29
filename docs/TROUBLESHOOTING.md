# 故障排查

先记录版本、部署方式、数据库和目标操作系统。提供日志前移除令牌、密码、Cookie、连接串和真实敏感 IP；不要提交完整 .env 或 agent.json。

| 现象 | 常见原因 | 排查方法 |
|---|---|---|
| 登录或保存返回 403 Origin 校验失败 | PUBLIC_URL 与浏览器地址不一致 | 协议、域名、端口必须完全一致；localhost 与 127.0.0.1 不等同 |
| 登录提示密码错误 | 使用了旧数据库；ADMIN_PASSWORD 只初始化一次 | 确认当前数据库与最初账号；不要假设编辑 .env 会重置密码 |
| 登录返回 429 | 同账号 15 分钟窗口超过 10 次，或 Nginx IP 限速 | 停止重试，按 Retry-After 等待，检查外部扫描流量 |
| 安装凭证 401 | 过期、已使用或重新生成后旧凭证失效 | 在该服务器条目生成新命令，仅执行一次 |
| Agent 心跳 401 | 凭证撤销或重装轮换 | 确认管理员操作，重新注册；不要在日志中打印旧令牌 |
| TLS / x509 错误 | 域名不匹配、证书链缺失、根证书未安装或时间错误 | 修复证书链、CA 包与系统时间；不使用跳过证书校验 |
| Nginx 502 | API 未运行、端口/上游不匹配、SELinux 网络策略 | 检查 API 服务与 health；核对原生 127.0.0.1:8080 / Compose api:8080 |
| 已有宝塔 / OpenResty，旧脚本又安装 Nginx 后失败 | 两套服务占用 80/443，系统包可能处于未配置状态 | 新向导选择独立或 external 模式；不要停止原网站。先确认实际 Web 服务路径和包归属，再修复包管理状态，不要直接卸载面板依赖 |
| HTTP 面板能访问但无法生成 Agent 命令 | 使用旧版本，或未显式启用 HTTP | v0.3.8 原生默认安装支持 IP 直连；旧安装可升级后显式配置 ALLOW_INSECURE_HTTP=true，或按需启用 HTTPS；HTTP 传输未加密 |
| 反代初始化出现超时 / 504 | 前置代理超时偏短 | 主控模板使用 90 秒；同步检查 CDN / 负载均衡，响应丢失后页面会核对初始化状态 |
| 自定义 HTTPS 端口下登录 403 / Agent 地址错误 | PUBLIC_URL 未包含外部端口 | 设置为浏览器完整 origin，例如 https://guard.example.com:8443；内部 proxy_pass 仍用主控端口；详见 [端口提示](REVERSE-PROXY.md#自定义端口提示) |
| 切换 HTTPS 后 Agent 报 state belongs to a different server | endpoint 是队列身份的一部分 | 先上传待传队列，停服务并归档原状态，再按 [迁移说明](AGENTS.md#http-地址迁移到-https) 在原记录重新接入；不要删除未上传证据 |
| 独立模式网页 404 | 缺少发布包中的 web 目录 | 保留程序旁的 web/index.html，或指定服务账号可读的 WEB_DIR；不要把 WEB_DIR 指向配置或数据目录 |
| MySQL 扫描时间字段失败 | DSN 缺少 parseTime=true | 使用文档中的 DSN，设置 UTC 时区 |
| 改数据库密码后无法启动 | 卷中的实际账号密码没变 | 在数据库管理会话修改密码，再同步 DATABASE_URL；环境变量不会重建已有库 |
| Alpine 安装提示 init 不可用 | 容器没有 OpenRC，或未安装 supervise-daemon | 使用已运行 OpenRC 的主机；容器场景自行以前台进程管理 |
| Alpine 没有 curl/证书 | 精简系统缺少前置依赖 | root 执行 apk add --no-cache curl ca-certificates |
| Windows 服务安装拒绝 | 未提权，或已存在同名不同路径服务 | 管理员 PowerShell 执行；确认旧安装归属，不覆盖不相关服务 |
| Windows 下载/脚本被组织策略阻止 | 企业签名、执行或代理策略 | 走组织批准的分发流程；不关闭终端防护或持久修改执行策略 |
| 页面没有 SSH 事件 | 日志源不可读、未启用 SSH 日志或队列未送达 | 查看采集状态与队列；实际发起一次 SSH 登录；检查 journal / auth.log / OpenSSH 通道权限 |
| 重复事件计数大于 0 | Agent 重试同一事件 UUID | 正常幂等行为，不要为重试生成新 UUID |

日志位置：

```bash
# 原生中央 API
journalctl -u sshguard-api -n 100 --no-pager
# Compose
docker compose logs --tail=100 api web db
# Ubuntu / Debian Agent
journalctl -u sshguard-agent -n 100 --no-pager
# Alpine Agent
tail -n 100 /var/log/sshguard/agent.log
```

```powershell
# Windows Agent
Get-Service SSHGuardAgent
Get-WinEvent -FilterHashtable @{LogName='Application';ProviderName='SSHGuardAgent'} -MaxEvents 30
```

Actions 失败：进入失败 job 查看第一处错误。依赖下载错误可重跑同一提交；代码/测试问题应提交修复后重跑。Tag 与 VERSION 不一致必须创建正确的新标签，不移动已有 Release 标签。若组织禁用了 Actions 或限制了依赖 Action，需仓库/组织管理员按自身策略启用所需能力。
