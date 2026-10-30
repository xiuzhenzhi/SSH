# HTTP(S) + WebSocket 上报协议

Agent 使用 Bearer 令牌，浏览器使用会话 Cookie。默认使用 HTTPS/WSS，直连模式可显式启用 HTTP/WS。

浏览器 Origin 必须与 `PUBLIC_URL` 一致；未配置时按实际请求地址检查。Agent stream 不接受浏览器 Origin。

## 数据路径

1. 采集器将事件及读取位置在同一个 bbolt 事务中保存并同步到磁盘。
2. 每批最多 100 个事件，优先通过 WSS 长连接发送。
3. 服务端重新核验凭证，锁定对应服务器，按 server_id + event_id 去重。
4. 新事件、命中告警、通知任务一起提交事务，提交成功才回复确认。
5. Agent 校验请求 ID、状态码与批次计数，确认全部记录已接收才删除本地批次。

确认丢失时保留队列，重传沿用事件 ID。WebSocket 失败后自动重连，并回退到 HTTP(S) 上报。

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

`GET /api/v1/live`，每用户最多 4 个连接、每实例最多 256 个。消息仅为 `{"type":"refresh","at":"…"}`，不携带日志或密钥。服务端按每个会话 5 秒合并刷新，前端以 500 毫秒短延迟合并读取，再按角色访问普通 API；重连后重新读取快照。会话每 30 秒复核，退出时关闭连接，后台标签暂停数据刷新。

随包 Nginx 模板为两条 stream 路由配置 HTTP/1.1、Upgrade/Connection、90 秒读取超时。自备反代须保留这些设置，不能缓存 API；配置错误时 Agent 会继续通过 HTTPS 工作。
