# HTTP(S) + WebSocket 上报协议

平台支持 HTTPS/WSS，以及原生直连模式显式授权的 HTTP/WS。HTTP 不提供传输加密，HTTPS 仍严格校验证书。Agent 使用 Authorization: Bearer 长期令牌；浏览器使用 HttpOnly 会话 Cookie，两种身份不能互换。Agent stream 拒绝浏览器 Origin，浏览器 live 要求 Origin 与配置的 PUBLIC_URL 完全相等；未配置 PUBLIC_URL 的直连模式按实际请求地址检查，不信任转发头。

## 数据路径

1. 采集器将事件及读取位置在同一个 bbolt 事务中保存并同步到磁盘。
2. 每批最多 100 个事件，优先通过 WSS 长连接发送。
3. 服务端重新核验凭证，锁定对应服务器，按 server_id + event_id 去重。
4. 新事件、命中告警、通知任务一起提交事务，提交成功才回复确认。
5. Agent 校验请求 ID、状态码与批次计数，确认全部记录已接收才删除本地批次。

确认丢失时原事件仍在队列中，HTTPS 重传使用同一事件 ID，重复记录不再创建告警。WebSocket 失败自动退避重连；HTTPS fallback 不要求负载均衡会话粘滞。单实例浏览器刷新提示是进程内广播，多 API 实例时仍有 30 秒轮询兜底。

## Agent WebSocket

`GET /api/v1/agent/stream`，Authorization 请求头认证，无查询字符串令牌。单服务器最多一个连接，单实例最多 4096 个 Agent 连接。每条消息上限 1 MiB，处理上限 20 秒，75 秒无业务消息关闭。

```json
{
  "type": "events",
  "request_id": "bb16d836-8df1-4a44-a47c-62c2d1ebdc7c",
  "data": {
    "events": [{
      "event_id": "5997976c-c66b-4d2a-b983-65d81d204803",
      "event_type": "ssh.login",
      "username": "root",
      "source_ip": "192.0.2.10",
      "source_port": 52100,
      "auth_method": "publickey",
      "outcome": "success",
      "occurred_at": "2026-10-28T09:00:00Z"
    }]
  }
}
```

```json
{"type":"ack","request_id":"bb16d836-8df1-4a44-a47c-62c2d1ebdc7c","status":200,"data":{"accepted":1,"inserted":1,"duplicates":0}}
```

`heartbeat` 消息的 data 与 HTTPS heartbeat 一致，包含 version、collector_state、collector_message、queue_depth、rejected_events、last_poll_at、last_event_at，以及 report_interval_seconds 与 ip_addresses。

| 状态 | Agent 行为 |
|---|---|
| 200 且确认匹配 | 成功，删除对应队列记录 |
| 400 | 单条拆分确认无效记录，转入本地隔离区；有效同批记录继续上报 |
| 401 | 保留队列；管理员处理凭证后重试 |
| 503 / 超时 / 不匹配 ACK | 不确认，保留队列并重试 |

HTTPS 使用 `POST /api/v1/agent/events/batch` 与 `POST /api/v1/agent/heartbeat`，响应外层为 data，处理语义相同。服务端接收最近 30 天及最多未来 5 分钟的事件。

## 浏览器实时更新

`GET /api/v1/live`，每用户最多 4 个连接、每实例最多 256 个。消息仅为 `{"type":"refresh","at":"…"}`，不携带日志或密钥。前端合并一秒内提示，再按角色访问普通 API；重连后重新读取快照。会话每 30 秒复核，退出时关闭连接，后台标签暂停数据刷新。

随包 Nginx 模板为两条 stream 路由配置 HTTP/1.1、Upgrade/Connection、90 秒读取超时。自备反代须保留这些设置，不能缓存 API；配置错误时 Agent 会继续通过 HTTPS 工作。

## 明确边界

本地队列不是操作系统日志备份。断网超过日志保留时间、磁盘损坏、手工删除队列、日志截断竞态均可能导致缺口。队列满会停止推进读取位置；缺口与隔离记录需人工处理。外部通知是至少一次投递，渠道已接受但确认丢失时可能重复，不能保证第三方 exactly-once。
