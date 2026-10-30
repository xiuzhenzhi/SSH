# 地理位置与公网 IP

管理员打开侧栏「地理位置」，选择数据提供商并保存。服务器名称前显示按公网 IP 查询到的国家 / 地区旗帜；列表及详情只显示公网 IPv4 / IPv6。默认提供商为 geojs.io，无需密钥。选择「无」关闭国家 / 地区查询及旗帜，公网 IP 仍正常显示。

| 选项 | 数据来源 | 配置 |
| --- | --- | --- |
| 无 | 关闭查询 | 无 |
| MaxMind | 本地 GeoLite2-Country MMDB | 保存后点击「更新数据库」 |
| ip-api.com | 在线 API | 免费接口使用 HTTP，每分钟最多 45 次，仅限非商业用途；可选 Pro Key 使用 HTTPS |
| geojs.io | 在线 HTTPS API | 无需密钥 |
| ipinfo.io | 在线 HTTPS API | 可选 Token；填写后使用 IPinfo Lite，免 Token 接口受服务商配额限制 |

在线查询从主控发出，只提交公网 IP。可选凭证使用现有 `ENCRYPTION_KEY` 加密存储，不返回浏览器；同一提供商下留空保留原凭证，勾选「清除已保存凭证」可移除，切换提供商会清空旧凭证。查询成功缓存 24 小时，失败短暂缓存并省略旗帜；服务故障不影响服务器状态和日志上报。

MaxMind 数据库从 [Loyalsoldier GeoIP](https://github.com/Loyalsoldier/geoip) 的 GeoLite2-Country 发布文件下载，与 Komari 的默认来源一致。该项目基于 MaxMind GeoLite2 数据构建。下载限制 32 MiB，校验 MMDB 格式和完整性后在事务中替换；失败保留旧库。数据库随 SQLite / PostgreSQL / MySQL 一起持久化和备份，无需额外数据目录。多实例在下一次查询时按数据库版本加载更新。

「测试 GeoIP」使用已保存的提供商查询输入的公网 IPv4 / IPv6，并展示旗帜、国家 / 地区名称和代码。设置尚未保存、选择「无」或 MaxMind 尚未下载时，测试按钮不可用。新增的 `010_geoip.sql` 迁移仅创建设置、数据库和心跳公网来源三张表，不改写原有事件或服务器配置。

NAT 地址的含义、外部探测与反代配置见 [Agent 手册](AGENTS.md#上报设置与-ip-地址)。Agent 与服务器 API 都过滤内网地址，兼容旧前端的 IP 字段也只返回公网地址。详情打开期间会跟随最新心跳刷新。SVG 旗帜使用 [flag-icons](https://github.com/lipis/flag-icons)，随前端打包，不依赖浏览器访问第三方旗帜服务；构建中包含其 [MIT 许可证](https://github.com/xiuzhenzhi/SSH/blob/main/web/public/assets/flag-icons-LICENSE.txt)。

## SSH 事件与首页统计

SSH 事件列表、事件详情和首页最近 5 条事件共用当前 GeoIP 提供商，显示来源国家 / 地区和旗帜。内网 / 保留地址单独标识；公网查询失败显示未知，事件仍可查看。归属地按当前数据源解析，不作为登录身份或安全结论。

首页「认证失败来源国家 TOP 5」统计近 24 小时失败事件，先按来源 IP 计数，再按国家 / 地区汇总。同一国家多个 IP 合并；未知归属的事件保留在总数及未定位计数中，不进入国家排名。选择「无」、MaxMind 未就绪或服务商不可用时显示对应空状态。

国家统计有独立的 5 秒短缓存，并发页面共享查询；单个页面取消不会中断其他页面，更换提供商会使旧统计失效。统计接口与字段见 [API 参考](API.md)。
