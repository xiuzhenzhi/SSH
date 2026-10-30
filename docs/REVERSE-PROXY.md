# 反向代理与 HTTPS

## 已有 Nginx / OpenResty / 宝塔

1. 为域名配置 HTTPS 站点。
2. 将下面配置放入站点的反向代理配置，替换原根 `location`。宝塔面板中不要再包 `server {}`。

```nginx
location ^~ / {
    proxy_pass http://127.0.0.1:8080;
    proxy_http_version 1.1;
    proxy_set_header Host $http_host;
    proxy_set_header X-Real-IP $remote_addr;
    proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
    proxy_set_header X-Forwarded-Proto $scheme;
    proxy_set_header Upgrade $http_upgrade;
    proxy_set_header Connection "upgrade";
    proxy_read_timeout 90s;
    proxy_connect_timeout 10s;
    proxy_send_timeout 90s;
    proxy_buffering off;
    proxy_cache off;
    client_max_body_size 1m;
}
```

3. 在 `/etc/sshguard/api.env` 填入实际域名：

```ini
PUBLIC_URL="https://guard.example.com"
LISTEN_ADDR="127.0.0.1:8080"
ALLOW_INSECURE_HTTP="false"
```

4. 在面板中检查并重载站点，然后重启主控：

```sh
sudo systemctl restart sshguard-api
curl -fsS https://guard.example.com/api/v1/health
```

### 自定义端口提示

主控端口不是 8080 时修改 `proxy_pass`。外部使用 8443 等端口时，`PUBLIC_URL` 也要包含该端口。

### WebSocket 与超时

保留模板中的 Upgrade、90 秒超时和关闭缓存设置。应用需部署在站点根路径。

## 已有 HTTP Agent 切换 HTTPS

先清空待传队列，再停服务、归档状态目录，并通过新地址重新生成安装命令。步骤见 [Agent 地址迁移](AGENTS.md#http-地址迁移到-https)。

## 审计来源 IP 与可信代理

同机反代默认可用。跨机器或 Compose 部署时，在 API 环境中设置反代的准确地址范围，例如：

```ini
TRUSTED_PROXY_CIDRS="172.28.0.2/32"
```

填 `none` 可关闭转发头信任。配置后重启 API，核对操作审计中的来源 IP。
