# 登录防护

## 配置验证码

1. 打开「登录防护」，选择验证码服务。
2. 填写服务商密钥和允许访问的主机名。
3. 输入当前管理员密码并保存。
4. 保留当前会话，在另一个窗口测试登录。

| 服务 | 填写内容 |
|---|---|
| Cloudflare Turnstile | Site Key、Secret Key，action 为 `login` |
| Google reCAPTCHA v2 | 复选框网站的 Site Key、Secret Key |
| 极验第四代 | Captcha ID、Captcha Key |

主机名填写域名或 IP，不带协议、端口和路径。默认不启用验证码；禁用后 IP 封禁规则仍有效。

## 60 秒内错误 5 次，封禁 24 小时

按来源 IP 统计密码和验证码错误。第 5 次起封禁 24 小时；验证码服务故障不累计错误次数。

管理员可在页面解除封禁。共用 NAT 出口的用户会共用计数，SSH IP 白名单不影响此规则。

## 反代设置

反代需传递真实客户端 IP。跨机器或 Compose 部署时配置 `TRUSTED_PROXY_CIDRS`，见 [反代说明](REVERSE-PROXY.md#审计来源-ip-与可信代理)。

验证码无法加载时，检查站点 CSP 和浏览器对第三方资源的拦截。

## 本机恢复

优先通过已有管理员会话修正设置。无法登录时，备份数据库并停服后处理：

- 关闭验证码：删除 `login_protection` 中 `id=1` 的记录。
- 解除指定 IP：清空 `login_guards` 对应记录的 `blocked_until`，并删除该 `source_hash` 的 `login_failures`。

只处理目标记录，不清空整张安全记录表。
