# API 参考

## API 概览

响应为 `{ "data": ... }` 或 `{ "error": "..." }`；列表为 `data.items`、`data.has_more`。默认每页 20 条，最大 100；无隐含全量导出。

| 方法 | 路径（前缀 /api/v1） | 权限 |
|---|---|---|
| GET | /health | 公开，仅健康信息 |
| POST | /auth/login | Origin + 登录限速 |
| GET / POST | /auth/me、/auth/logout | 登录用户 |
| GET | /dashboard/summary | 全角色 |
| GET | /dashboard/overview、/alerts | 全角色 |
| POST | /alerts/:id/acknowledge | admin/operator |
| POST | /alerts/acknowledge | admin/operator，批量确认 |
| GET | /dashboard/country-ranking | 全角色，近 24 小时认证失败来源国家 TOP 5 |
| GET / PATCH | /settings/geoip | admin |
| POST | /settings/geoip/test、/settings/geoip/update | admin |
| GET / POST / PATCH | /channels、/channels/:id | admin |
| POST | /channels/:id/test | admin |
| GET / POST / PATCH | /rules、/rules/:id | admin |
| GET / POST | /notifications、/notifications/:id/retry | admin |
| GET (WebSocket) | /live | 登录会话 + 精确 Origin |
| GET (WebSocket) | /agent/stream | Agent Bearer，无浏览器 Origin |
| GET | /servers、/servers/:id | 全角色 |
| POST / PATCH | /servers、/servers/:id | admin/operator |
| POST | /servers/:id/enrollment、/servers/:id/revoke | admin |
| GET / POST | /users | admin |
| GET | /audit-logs | admin |
| POST | /agent/enroll | 单次注册凭证 |
| POST | /agent/heartbeat、/agent/events/batch | Agent Bearer |
| GET | /events、/events/:id | 全角色 |

所有 Web 写操作包括登录均要求与 `PUBLIC_URL` 一致的 Origin；默认直连未配置 PUBLIC_URL 时，按实际请求地址核对。CLI 写请求也必须携带 Origin。Agent 使用 Bearer 身份，不使用 Web Cookie。`GET /events` 支持 `server_id/source_ip/username/outcome/event_type/from/to/page/page_size`，时间为 RFC3339。`PATCH /servers/:id` 提交完整可编辑表单 `name/group_name/notes`，未提供分组和备注会清空它们。

```json
{
  "events": [{
    "event_id": "3f0cb95d-97c4-4dbe-8f24-1d4b69d1e7a1",
    "event_type": "ssh.login",
    "username": "root",
    "source_ip": "203.0.113.10",
    "source_port": 51234,
    "auth_method": "publickey",
    "outcome": "success",
    "occurred_at": "2026-10-28T07:39:09Z"
  }]
}
```

向 `/api/v1/agent/events/batch` POST，头部 `Authorization: Bearer <AGENT_TOKEN>`。请将时间换为当前时间、事件 UUID 换为新 UUID；重试时必须保留同一 UUID。响应给出 `accepted/inserted/duplicates`，整批校验失败不会部分写入；服务端从凭证决定 server_id，不信任客户端声明。事件留存清理已实现，规则见 [运维手册](OPERATIONS.md)；原始日志备份与会话打开/关闭事件不在当前采集范围内。


## 安装命令响应

POST /api/v1/servers/:id/enrollment 返回 command（Ubuntu 命令）、commands（windows/ubuntu/debian/alpine）、expires_at、phase。四条命令共享一个单次凭证，不要公开保存响应。

GET /api/v1/health 返回 status、database、phase、version，不返回连接串或密码。

安装凭证错误/过期/重复使用返回 401；角色不足或 Origin 不匹配返回 403；参数错误返回 400；登录限速返回 429；重复事件仍返回 200，并在 duplicates 字段计数。

## 维护与身份接口

- `GET /api/v1/agent/update-commands`：admin 获取各系统更新命令与平台版本，无注册令牌。
- `GET /api/v1/agent/uninstall-commands`：admin 获取各系统手动卸载命令，不远程执行。
- `POST /api/v1/servers/:id/update`：admin + 同源校验创建固定版本更新任务；无 URL、脚本或命令参数。需要近期在线的维护服务，无重复运行任务。
- `POST /api/v1/agent/update/poll`：Agent 凭证认证，维护服务领取绑定自己服务器及当前凭证的任务；响应 `update: null` 或 `{id, version}`，任务只领取一次。
- `POST /api/v1/agent/update/result`：Agent 上报 `{id, status}`，status 只允许 success / failed；必须匹配自己当前运行且未过期的任务。
- `POST /api/v1/auth/password`：已登录用户提交 `current_password`、`new_password`（12–72 字节），成功后全部旧会话失效，需重新登录。
- 心跳接受可选 `version`（最多 32 字符），更新服务器 Agent 版本。
- 统计摘要返回 `version`；请求上下文 20 秒超时。登录/改密共享每进程每分钟 60 次预算；登录另有数据库级账号窗口限速。

## 配置、审计与趋势

- `POST /servers/:id/enrollment`（admin）：可带 `{"report_interval_seconds":5}`，范围 1–30；设置绑定单次凭证，仅在注册安装时写入。服务器新增 / 编辑拒绝该字段，没有运行时间隔编辑接口。
- 注册响应返回安装 `report_interval_seconds`；HTTP / WebSocket 心跳 ACK 只确认接收，不下发配置。心跳可带实际间隔及最多 16 个有效 `ip_addresses`；无效值拒绝整条心跳。
- 服务器列表返回安装 / 实际间隔及 JSON 字符串 `ip_addresses`，仅包含公网地址；Agent 在后台探测 NAT 公网出口，主控还使用经过认证的连接来源补充地址。查询结果和边界见 [地理位置](GEOIP.md)。
- `GET /ip-allowlist`、`POST /ip-allowlist`、`DELETE /ip-allowlist/:id` 仅 admin；新增 body 为 `{"cidr":"192.0.2.1","notes":"运维出口"}`。支持 IP 或 CIDR，规范化重复 / 数量上限返回 409。白名单只抑制新 SSH 事件告警，不丢弃事件。
- `GET /audit-logs` 增加 `source_ip`、`user_agent`。后端保留稳定动作标识，前端显示中文。
- `GET /dashboard/overview?range=24h|7d|14d|30d` 返回对应 24 个小时 / 7、14、30 个 UTC 日桶；不支持的范围返回 400。统计和摘要共享每进程最多 5 个键、5 秒缓存，不按用户或任意输入无限建缓存。

- `GET /auth/protection` 公开返回验证码类型和站点标识；`GET/PATCH /auth/protection/settings` 仅 admin。保存提交 provider（none / turnstile / recaptcha / geetest）、site_key、secret、hostnames、current_password，密钥只写不回显。
- 登录 body 增加可选 `captcha`：Turnstile / Google 为 `{token}`，极验为 `{lot_number,captcha_output,pass_token,gen_time}`；启用验证时服务端强制检查。
- `GET /auth/blocked-ips` 与 `DELETE /auth/blocked-ips/:id` 仅 admin，解除会清空本轮失败计数并审计。第 5 次 60 秒窗口内登录错误返回 429；封禁 24 小时，Retry-After 为剩余秒数。第三方故障返回 503，不计入错误。

## 批量告警、地理位置与网页初始化

- `POST /alerts/acknowledge` 接受 `{"ids":["告警UUID"]}`（最多 1,000 个）或 `{"all":true}`，两种方式互斥。返回 `{"acknowledged":数量}`；只修改未确认且未恢复的告警。所选 ID 不存在时整个操作失败，不部分提交；动作写入 `alert.acknowledge.batch` 审计。
- 事件列表与详情返回 `country_code`、`country_name`、`source_is_public`，国家由当前 GeoIP 提供商查询；失败时国家为空，不阻断事件查询。
- `GET /dashboard/country-ranking` 返回 `items`（最多 5 项，每项含国家代码、名称、失败次数）、`total`、`unlocated`、`country_count`、`geoip_status`、`from`、`as_of`；统计近 24 小时认证失败，按来源 IP 查询后汇总。
- `GET/PATCH /settings/geoip` 读取 / 保存提供商，provider 为 none / maxmind / ipapi / geojs / ipinfo；可选 token 只写不回显，`clear_token:true` 清除凭证。`POST /settings/geoip/test` 接受 `{"ip":"公网IPv4或IPv6"}`；`POST /settings/geoip/update` 更新 MaxMind 本地库。
- `GET /setup/status` 返回是否需要网页初始化。仅初始化阶段开放 `POST /setup/test` 和 `POST /setup/complete`，要求同源与 `X-Setup-Token`；完成后不再提供初始化写入口。口令来自安装器的终端链接片段，不能放进查询参数或日志。

主控自身升级通过本机脚本执行，不提供浏览器触发主控升级的 API；`/servers/:id/update` 只更新 Agent。
