# 反向代理与 HTTPS

默认使用 IP + 端口即可完成主控与 Agent 安装。需要 HTTPS 时可复用 Nginx / OpenResty / 宝塔，不必重新部署数据库或创建管理员。已有安装先备份 `/etc/sshguard/api.env`、`setup.json`、数据库与加密密钥。

## 已有 Nginx / OpenResty / 宝塔

1. 在已有站点配置证书和 HTTPS 监听，保留面板管理的域名、证书与续期配置。
2. 将 [server 块内反代模板](../deploy/native/nginx-existing.conf) 放入现有 server 块，替换原来的根 `location`。宝塔可在对应站点的反向代理配置中粘贴；不要再包一层 `server {}`。主控不是 8080 时修改 `proxy_pass`。
3. 修改 `/etc/sshguard/api.env`，使外部地址与浏览器地址一致，例如：

   ```ini
   PUBLIC_URL="https://guard.example.com"
   LISTEN_ADDR="127.0.0.1:8080"
   ALLOW_INSECURE_HTTP="false"
   ```

4. 用面板或该 Web 服务实际使用的命令验证并重载配置，再执行 `sudo systemctl restart sshguard-api`。宝塔 / OpenResty 不要误用另一套系统 Nginx 命令。
5. 检查 `https://guard.example.com/api/v1/health`、登录和实时状态。此后主控的 8080 不需要对公网开放；移除云安全组中旧端口的开放规则前确认反代可用。

`PUBLIC_URL` 包含协议、主机和可选端口，不包含路径、查询参数或 `#`。程序不根据 `X-Forwarded-Host` / `X-Forwarded-Proto` 自动推断可信地址。不要在反代中覆盖浏览器的 `Origin`，也不要为解决 403 而关闭同源检查。

目前应部署在独立站点的根路径，不支持挂载到 `/sshguard/` 等子路径。

### 自定义端口提示

外部 HTTPS 端口与主控的内部端口是两个配置。例如外部访问 `https://guard.example.com:8443`，内部仍为 `127.0.0.1:8080`：将 `PUBLIC_URL` 设置为前者，反代 `proxy_pass` 设置为后者，并在站点 / 云安全组放行外部端口。模板不会自动改动站点监听、端口映射或 HTTP 跳转规则。

Nginx 的 `$host` 不带客户端端口；主控使用显式 `PUBLIC_URL` 做同源检查和生成 Agent 命令。如果其他上游逻辑也需要 Host 中的原端口，可手动将 `proxy_set_header Host $host;` 改为 `proxy_set_header Host $http_host;`。443 / 80 等默认端口会按浏览器 Origin 规则规范化。更改后重新验证配置。

### WebSocket 与超时

模板包含 HTTP/1.1、Upgrade / Connection、90 秒读写超时与关闭缓存。便于粘贴的 server 片段使用 Nginx 官方支持的简单 Upgrade 写法，不依赖面板外的 `map`；完整托管模板在 http 层使用 `map`。原生部署反代整个应用，由主控限制可公开访问的文件；不要把 `/etc/sshguard` 或数据目录设为站点根目录。

不要对 `/api/` 开启代理缓存。初始化连接 / 迁移可能超过默认的 30 秒，前置 CDN、负载均衡也应允许至少 90 秒。浏览器流为 `/api/v1/live`，Agent 流为 `/api/v1/agent/stream`；不得要求 Agent 跳转到额外登录页。平台保留 HTTP(S) 上报回退与浏览器轮询。

## 已有 HTTP Agent 切换 HTTPS

修改主控 `PUBLIC_URL` 不会改写目标机器上的 Agent 地址。原更新命令会核对现有 endpoint，避免将凭证发到不同地址，因此不能用更新命令直接迁移。

先确认旧队列已上传并备份本地状态，再通过新 HTTPS 地址打开控制台，在原服务器记录上重新生成安装命令，逐台重新接入并检查采集状态。注册会轮换该服务器凭证；endpoint 属于队列身份的一部分，旧状态必须按 [Agent 手册](AGENTS.md) 的身份变更说明处理，不能直接删除尚未上传的证据。切换前安排短暂采集窗口，迁移后验证一次真实登录事件。

## 完整托管 / Compose

新服务器可显式选择 `--proxy managed`，使用 [完整原生模板](../deploy/native/nginx.conf)。升级不会覆盖你已有的托管站点配置，模板变化需手动合并、验证并重载。Compose 使用 [HTTPS 模板](../deploy/nginx.conf) 或 [本机 HTTP 模板](../deploy/nginx.local.conf)，配置见 [Compose 安装](INSTALL-COMPOSE.md)。

参考：[Nginx WebSocket 代理](https://nginx.org/en/docs/http/websocket.html)、[proxy_set_header](https://nginx.org/en/docs/http/ngx_http_proxy_module.html#proxy_set_header)、[proxy_read_timeout](https://nginx.org/en/docs/http/ngx_http_proxy_module.html#proxy_read_timeout)。
