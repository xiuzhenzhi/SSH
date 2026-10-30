# 地理位置与公网 IP

## 选择数据源

进入「地理位置」，选择提供商并保存，默认使用无需密钥的 geojs.io。

| 提供商 | 配置 |
|---|---|
| 无 | 关闭国家查询与旗帜 |
| MaxMind | 保存后点击「更新数据库」 |
| ip-api.com | 免费接口为 HTTP；Pro Key 可启用 HTTPS |
| geojs.io | 无需密钥，HTTPS |
| ipinfo.io | 可选 Token，填写后使用 IPinfo Lite |

同一提供商下留空会保留原凭证；勾选「清除已保存凭证」可删除。MaxMind 数据来自 [Loyalsoldier GeoIP](https://github.com/Loyalsoldier/geoip)。

## 测试查询

保存设置后，在「测试 GeoIP」输入公网 IPv4 或 IPv6，点击测试。选择「无」或 MaxMind 未下载数据库时不可测试。

## 查看结果

- 服务器名称前显示国家旗帜，详情显示公网 IPv4 / IPv6。
- SSH 事件和首页最近事件显示来源国家，查询失败显示未知。
- 首页「认证失败来源国家 TOP 5」统计近 24 小时失败次数，同一国家的多个 IP 合并计算。
- 未定位事件计入总数，不进入国家排名。

公网地址识别见 [Agent 管理](AGENTS.md#上报设置与-ip-地址)，接口字段见 [API 参考](API.md)。
