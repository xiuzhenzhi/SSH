# API 参考

## API 概览

响应为 `{ "data": ... }` 或 `{ "error": "..." }`；列表为 `data.items`、`data.has_more`。默认每页 20 条，最大 100；无隐含全量导出。

| 方法 | 路径（前缀 /api/v1） | 权限 |
|---|---|---|
| GET | /health | 公开，仅健康信息 |
| POST | /auth/login | Origin + 登录限速 |
| GET / POST | /auth/me、/auth/logout | 登录用户 |
| GET | /dashboard/summary | 全角色 |
| GET | /servers、/servers/:id | 全角色 |
| POST / PATCH | /servers、/servers/:id | admin/operator |
| POST | /servers/:id/enrollment、/servers/:id/revoke | admin |
| GET / POST | /users | admin |
| GET | /audit-logs | admin |
| POST | /agent/enroll | 单次注册凭证 |
| POST | /agent/heartbeat、/agent/events/batch | Agent Bearer |
| GET | /events、/events/:id | 全角色 |

所有 Web 写操作包括登录均要求 `Origin: PUBLIC_URL`；CLI 请求也必须携带。Agent 使用 Bearer 身份，不使用 Web Cookie。`GET /events` 支持 `server_id/source_ip/username/outcome/event_type/from/to/page/page_size`，时间为 RFC3339。`PATCH /servers/:id` 提交完整可编辑表单 `name/group_name/notes`，未提供分组和备注会清空它们。

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

向 `/api/v1/agent/events/batch` POST，头部 `Authorization: Bearer <AGENT_TOKEN>`。请将时间换为当前时间、事件 UUID 换为新 UUID；重试时必须保留同一 UUID。响应给出 `accepted/inserted/duplicates`，整批校验失败不会部分写入；服务端从凭证决定 server_id，不信任客户端声明。事件留存清理、原始日志、会话打开/关闭事件在后续阶段实现。


## 安装命令响应

POST /api/v1/servers/:id/enrollment 返回 command（Ubuntu 兼容字段）、commands（windows/ubuntu/debian/alpine）、expires_at、phase。四条命令共享一个单次凭证，不要公开保存响应。

GET /api/v1/health 返回 status、database、phase、version，不返回连接串或密码。

安装凭证错误/过期/重复使用返回 401；角色不足或 Origin 不匹配返回 403；参数错误返回 400；登录限速返回 429；重复事件仍返回 200，并在 duplicates 字段计数。
