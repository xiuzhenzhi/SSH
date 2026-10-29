#!/bin/sh
# Run from an extracted, checksum-verified native release. Ubuntu/Debian + systemd.
# No shell sourcing/eval of user configuration. Secrets never appear in argv.
set -eu
umask 077
die() { echo "$*" >&2; exit 1; }
if [ "${1:-}" = --help ]; then
    printf 'SSHGuard 安装与管理\n\n安装：sh deploy.sh [--proxy standalone|external|managed] [--config 文件]\n更新：sh deploy.sh --update\n状态：sh deploy.sh --status\n日志：sh deploy.sh --logs\n重启：sh deploy.sh --restart\n\nstandalone：直接提供网页/API，可选内置 HTTPS；无需 Nginx。\nexternal：复用 Nginx/OpenResty/宝塔，只需反代整个应用。\nmanaged：兼容系统 Nginx 管理模式。\n'
    exit 0
fi
[ "$(id -u)" -eq 0 ] || die 'Run with sudo sh deploy.sh.'
if [ "$#" -eq 0 ] && [ -t 0 ]; then
    printf '\n============== SSHGuard 管理菜单 ==============\n'
    if [ -f /etc/sshguard/deploy-managed ]; then printf '状态：已安装\n'; else printf '状态：未安装\n'; fi
    printf '  1) 安装 / 更新\n  2) 查看状态\n  3) 查看日志\n  4) 重启\n  5) 启动\n  6) 停止\n  0) 退出\n请选择 [默认 1]: '
    IFS= read -r choice
    case "${choice:-1}" in
        1) if [ -f /etc/sshguard/deploy-managed ]; then set -- --update; fi ;;
        2) set -- --status ;; 3) set -- --logs ;; 4) set -- --restart ;;
        5) set -- --start ;; 6) set -- --stop ;; 0) exit 0 ;; *) die '无效选项。' ;;
    esac
fi
case "${1:-}" in
    --status|--logs|--restart|--start|--stop)
        [ "$#" -eq 1 ] && [ -f /etc/sshguard/deploy-managed ] || die '未找到向导管理的安装，或参数无效。'
        case "$1" in
            --logs) journalctl -u sshguard-api -n 100 --no-pager; exit $? ;;
            --start) systemctl start sshguard-api ;;
            --stop) systemctl stop sshguard-api ;;
            --restart) systemctl restart sshguard-api; systemctl is-active --quiet sshguard-api || die '启动失败，请查看 --logs。' ;;
        esac
        printf '\nSSHGuard 运行状态\n'
        systemctl show sshguard-api --property=ActiveState,SubState,MainPID --no-pager
        [ ! -f /opt/sshguard/VERSION ] || printf 'Version=%s\n' "$(cat /opt/sshguard/VERSION)"
        # Display only non-secret fields; never source the environment file.
        sed -n '/^PUBLIC_URL=/p; /^LISTEN_ADDR=/p; /^DB_DRIVER=/p' /etc/sshguard/api.env
        printf '日志：sh /opt/sshguard/deploy.sh --logs\n'
        exit 0 ;;
esac
bundle=$(CDPATH='' cd -- "$(dirname -- "$0")" && pwd)
[ -f "$bundle/sshguard-api" ] && [ -f "$bundle/web/index.html" ] || die 'Run deploy.sh from an extracted native release, not the source scripts directory.'
version=$(cat "$bundle/VERSION")
printf '%s' "$version" | grep -Eq '^[A-Za-z0-9.+_-]{1,32}$' || die 'Invalid release version.'
command -v apt-get >/dev/null 2>&1 && [ -d /run/systemd/system ] || die 'Central deployment requires Ubuntu/Debian with systemd.'
mode=install
config_file=''
proxy_arg=''
while [ "$#" -gt 0 ]; do
    case "$1" in
        --install) shift ;;
        --update) mode=update; shift ;;
        --config) [ "$#" -ge 2 ] || die '--config requires a file'; config_file=$2; shift 2 ;;
        --proxy) [ "$#" -ge 2 ] || die '--proxy requires standalone, external or managed'; proxy_arg=$2; shift 2 ;;
        *) die 'Usage: deploy.sh [--update] [--proxy standalone|external|managed] [--config /path/deploy.conf]' ;;
    esac
done
domain=''; driver=''; dsn=''; certificate=''; private_key=''; email=''; admin_password=''; proxy_mode=''
app_port=''; public_origin=''; builtin_tls=false; initial_setup=false
if [ -n "$config_file" ]; then
    [ -f "$config_file" ] && [ ! -L "$config_file" ] || die 'Configuration must be a regular file.'
    [ "$(stat -c %a "$config_file")" = 600 ] || die 'Set configuration permissions to 600.'
    while IFS= read -r line || [ -n "$line" ]; do
        case "$line" in ''|'#'*) continue ;; esac
        key=${line%%=*}; value=${line#*=}
        case "$key" in
            DOMAIN) domain=$value ;; DB_DRIVER) driver=$value ;; DATABASE_URL) dsn=$value ;;
            TLS_CERT) certificate=$value ;; TLS_KEY) private_key=$value ;; ACME_EMAIL) email=$value ;;
            ADMIN_PASSWORD) admin_password=$value ;; PROXY_MODE) proxy_mode=$value ;; APP_PORT) app_port=$value ;; *) die "Unknown configuration key: $key" ;;
        esac
    done < "$config_file"
fi
prompt() { printf '%s' "$1" >&2; IFS= read -r answer; }
for path in /opt/sshguard-releases /etc/sshguard /etc/sshguard/api.env /etc/sshguard/deploy-managed /etc/sshguard/proxy-mode /etc/sshguard/nginx-existing.conf; do
    [ ! -L "$path" ] || die "Refusing symlink: $path"
done
if [ -e /opt/sshguard ] && [ ! -L /opt/sshguard ]; then die 'Existing /opt/sshguard is a directory. Follow the legacy migration instructions before using this updater.'; fi
if [ "$mode" = update ]; then
    [ -L /opt/sshguard ] && [ -f /etc/sshguard/deploy-managed ] || die 'No wizard-managed installation found.'
elif [ -L /opt/sshguard ]; then die 'Already installed. Use --update.'; fi
printf '\n============================================================\n'
printf '  SSHGuard %s · 安装 / 更新向导\n' "$version"
printf '============================================================\n'
printf '\n[1/5] 检查运行环境\n'
existing_web=false
custom_web=false
if command -v openresty >/dev/null 2>&1; then custom_web=true; fi
for web_binary in /www/server/nginx/sbin/nginx /usr/local/nginx/sbin/nginx /usr/local/openresty/nginx/sbin/nginx /usr/local/openresty/bin/openresty /www/server/openresty/nginx/sbin/nginx; do
    if [ -x "$web_binary" ]; then custom_web=true; fi
done
if command -v nginx >/dev/null 2>&1 || "$custom_web"; then existing_web=true; fi
if command -v ss >/dev/null 2>&1 && ss -H -ltn '( sport = :80 or sport = :443 )' 2>/dev/null | grep -q .; then existing_web=true; fi
if "$existing_web"; then printf '  已有 Web 服务保持原样，SSHGuard 使用独立端口。\n';
else printf '  未检测到现有 Web 服务。\n'; fi
[ -z "$proxy_arg" ] || proxy_mode=$proxy_arg
if [ -f /etc/sshguard/api.env ] && [ -f /etc/sshguard/deploy-managed ]; then
    saved_proxy=managed
    [ ! -f /etc/sshguard/proxy-mode ] || saved_proxy=$(cat /etc/sshguard/proxy-mode)
    [ -z "$proxy_mode" ] || [ "$proxy_mode" = "$saved_proxy" ] || die 'Proxy mode changes require explicit migration; updates preserve the installed mode.'
    proxy_mode=$saved_proxy
fi
proxy_mode=${proxy_mode:-standalone}
case "$proxy_mode" in standalone|external|managed) ;; *) die 'PROXY_MODE must be standalone, external or managed.' ;; esac
if [ "$proxy_mode" = managed ]; then
    if "$custom_web"; then
        die '检测到 OpenResty、面板或自定义 Nginx，请使用 --proxy external，避免安装第二套 Nginx。'
    fi
    if "$existing_web" && [ ! -x /usr/sbin/nginx ]; then
        die '已有 Web 服务占用端口，请使用 --proxy external，避免端口冲突。'
    fi
fi
printf '  本次模式：%s\n' "$proxy_mode"
mkdir /run/sshguard-deploy.lock 2>/dev/null || die 'Another deployment is active (or /run/sshguard-deploy.lock is stale).'
switched=false
old=''
stage=''
hidden=false
nginx_changed=false
nginx_backup=''
nginx_existed=false
cleanup() {
    status=$?
    trap - EXIT HUP INT TERM
    if "$hidden"; then stty echo 2>/dev/null || true; fi
    if "$switched" && [ "$status" -ne 0 ]; then
        if [ -n "$old" ]; then
            ln -s "$old" /opt/sshguard.next
            mv -Tf /opt/sshguard.next /opt/sshguard
            systemctl restart sshguard-api || true
            echo 'Deployment failed; previous application restored. Database migrations are not reversed.' >&2
        else
            systemctl stop sshguard-api || true
            if [ "$(readlink /opt/sshguard)" = "$stage" ]; then rm /opt/sshguard; fi
        fi
    fi
    if "$nginx_changed" && [ "$status" -ne 0 ]; then
        if "$nginx_existed"; then cp "$nginx_backup" /etc/nginx/conf.d/sshguard.conf
        else rm -f /etc/nginx/conf.d/sshguard.conf; fi
        nginx -t && systemctl reload nginx || true
    fi
    [ -z "$nginx_backup" ] || rm -f "$nginx_backup"
    rmdir /run/sshguard-deploy.lock
    exit "$status"
}
trap cleanup EXIT
trap 'exit 1' HUP INT TERM
export DEBIAN_FRONTEND=noninteractive
apt_updated=false
install_packages() {
    if ! "$apt_updated"; then apt-get update; apt_updated=true; fi
    apt-get install -y "$@"
}
if [ "$mode" = install ]; then
    # Install only missing dependencies; never upgrade a working web stack.
    command -v curl >/dev/null 2>&1 || install_packages curl
    command -v openssl >/dev/null 2>&1 || install_packages openssl
    [ -s /etc/ssl/certs/ca-certificates.crt ] || install_packages ca-certificates
    if [ "$proxy_mode" = managed ] && [ ! -x /usr/sbin/nginx ]; then install_packages nginx; fi
fi
command -v curl >/dev/null 2>&1 || die 'curl is required.'
printf '\n[2/5] 设置应用端口\n'
if [ -f /etc/sshguard/api.env ]; then
    [ -f /etc/sshguard/deploy-managed ] || die 'Existing api.env is not wizard-managed; migrate explicitly, do not overwrite credentials.'
    public_origin=$(sed -n 's/^PUBLIC_URL="\([^"]*\)"$/\1/p' /etc/sshguard/api.env)
    domain=$(printf '%s' "$public_origin" | sed 's|^https\{0,1\}://||; s/:[0-9]*$//')
    app_port=$(sed -n 's/^LISTEN_ADDR="[^:]*:\([0-9]*\)"$/\1/p' /etc/sshguard/api.env)
    if grep -q '^TLS_CERT_FILE=' /etc/sshguard/api.env; then builtin_tls=true; fi
else
    [ "$mode" = install ] || die 'Missing /etc/sshguard/api.env.'
    initial_setup=true
    if [ "$proxy_mode" != standalone ] || [ -n "$certificate" ]; then
        if [ -z "$domain" ] && [ -z "$config_file" ]; then prompt 'HTTPS domain: '; domain=$answer; fi
        printf '%s' "$domain" | grep -Eq '^[A-Za-z0-9][A-Za-z0-9.-]*\.[A-Za-z0-9-]+$' || die 'DOMAIN required for explicit HTTPS/proxy mode.'
    fi
    if [ "$proxy_mode" = standalone ]; then
        if [ -n "$certificate" ]; then builtin_tls=true; fi
        if [ -z "$app_port" ] && [ -z "$config_file" ]; then
            prompt '应用端口 [默认 8080，范围 1024–65535]: '; app_port=${answer:-8080}
        fi
    fi
    app_port=${app_port:-8080}
    case "$app_port" in ''|*[!0-9]*) die 'APP_PORT must be 1024–65535.' ;; esac
    [ "${#app_port}" -le 5 ] && [ "$app_port" -ge 1024 ] && [ "$app_port" -le 65535 ] || die 'APP_PORT must be 1024–65535.'
    if command -v ss >/dev/null 2>&1 && ss -H -ltn "sport = :$app_port" 2>/dev/null | grep -q .; then die "应用端口 $app_port 已被占用，请通过 APP_PORT 选择其他端口。"; fi
    public_origin="https://$domain"
    listen_address="127.0.0.1:$app_port"
    if [ "$proxy_mode" = standalone ]; then
        listen_address="0.0.0.0:$app_port"
        if "$builtin_tls"; then public_origin="https://$domain:$app_port"; else public_origin="http://$domain:$app_port"; fi
    fi
    configured_origin=$public_origin
    if [ "$proxy_mode" = standalone ] && ! "$builtin_tls" && [ -z "$domain" ]; then configured_origin=''; public_origin="http://SERVER_IP:$app_port"; fi
    driver=${driver:-sqlite}
    case "$driver" in
        sqlite)
            # Keep writable state outside release directories and inside the
            # systemd StateDirectory. Do not accept arbitrary filesystem paths.
            [ -z "$dsn" ] || [ "$dsn" = 'file:/var/lib/sshguard-api/sshguard.db' ] || die 'SQLite wizard uses file:/var/lib/sshguard-api/sshguard.db; omit DATABASE_URL.'
            dsn='file:/var/lib/sshguard-api/sshguard.db' ;;
        local-postgres)
            [ -z "$dsn" ] || die 'Use DB_DRIVER=postgres for an existing database.'
            command -v psql >/dev/null 2>&1 || install_packages postgresql
            systemctl enable --now postgresql
            db_password=$(openssl rand -hex 24)
            # Do not alter a pre-existing role/database owned by another installation.
            exists=$(su postgres -s /bin/sh -c "psql -Atqc \"SELECT count(*) FROM pg_roles WHERE rolname='sshguard'\"")
            [ "$exists" = 0 ] || die 'PostgreSQL role sshguard already exists. Configure an external DSN explicitly.'
            printf "CREATE ROLE sshguard LOGIN PASSWORD '%s';\nCREATE DATABASE sshguard OWNER sshguard;\n" "$db_password" | su postgres -s /bin/sh -c 'psql -v ON_ERROR_STOP=1'
            dsn="postgres://sshguard:$db_password@127.0.0.1:5432/sshguard?sslmode=disable"
            unset db_password
            driver=postgres ;;
        postgres|mysql)
            if [ -z "$dsn" ] && [ -z "$config_file" ]; then
                printf 'Database DSN (hidden; create database/user first): ' >&2
                hidden=true; stty -echo; IFS= read -r dsn; stty echo; hidden=false; printf '\n' >&2
            fi
            [ -n "$dsn" ] || die 'Set DATABASE_URL for your existing database.' ;;
        *) die 'DB_DRIVER must be sqlite, local-postgres, postgres or mysql.' ;;
    esac
    [ -n "$admin_password" ] || admin_password=$(openssl rand -hex 20)
    [ "${#admin_password}" -ge 12 ] && [ "${#admin_password}" -le 72 ] || die 'ADMIN_PASSWORD must be 12–72 bytes.'
    mkdir -p /etc/sshguard
    # Shared parent for API and Agent: traversal only for other accounts.
    # Secrets remain protected by their own file/subdirectory permissions.
    chown root:root /etc/sshguard
    chmod 0711 /etc/sshguard
    # systemd EnvironmentFile escaping, never interpreted by a shell.
    env_line() { printf '%s="' "$1"; printf '%s' "$2" | sed 's/\\/\\\\/g; s/"/\\"/g'; printf '"\n'; }
    {
        env_line DB_DRIVER "$driver"; env_line DATABASE_URL "$dsn"; env_line PUBLIC_URL "$configured_origin"
        env_line LISTEN_ADDR "$listen_address"; env_line ADMIN_USERNAME admin; env_line ADMIN_PASSWORD "$admin_password"; env_line GIN_MODE release
        if [ "$proxy_mode" = standalone ]; then
            if "$builtin_tls"; then
                env_line TLS_CERT_FILE /etc/sshguard/tls/fullchain.pem; env_line TLS_KEY_FILE /etc/sshguard/tls/privkey.pem
            else env_line ALLOW_INSECURE_HTTP true; fi
        fi
    } > /etc/sshguard/api.env
    chmod 0600 /etc/sshguard/api.env
    touch /etc/sshguard/deploy-managed
    unset dsn admin_password
fi
if [ -n "$public_origin" ] && [ "$public_origin" != "http://SERVER_IP:$app_port" ]; then
    printf '%s' "$domain" | grep -Eq '^[A-Za-z0-9][A-Za-z0-9.-]*\.[A-Za-z0-9-]+$' || die 'Invalid managed PUBLIC_URL.'
fi
[ -n "$public_origin" ] || public_origin="http://SERVER_IP:$app_port"
printf '%s\n' "$proxy_mode" > /etc/sshguard/proxy-mode
chmod 0600 /etc/sshguard/proxy-mode
# Upgrade old wizard installations once, preserving the key on every future update.
if ! grep -q '^ENCRYPTION_KEY=' /etc/sshguard/api.env; then
    key_file=$(mktemp /etc/sshguard/.api-env.XXXXXX)
    chmod 0600 "$key_file"
    cat /etc/sshguard/api.env > "$key_file"
    printf '\nENCRYPTION_KEY="%s"\n' "$(openssl rand -hex 32)" >> "$key_file"
    mv -f "$key_file" /etc/sshguard/api.env
fi
if ! id sshguard-api >/dev/null 2>&1; then useradd --system --user-group --no-create-home --shell /usr/sbin/nologin sshguard-api; fi
chown root:root /etc/sshguard
chmod 0711 /etc/sshguard
if [ "$mode" = install ] && [ "$proxy_mode" = standalone ] && "$builtin_tls"; then
    [ -f "$certificate" ] && [ -f "$private_key" ] || die 'TLS_CERT and TLS_KEY must exist.'
    [ ! -L /etc/sshguard/tls ] || die 'Refusing symlink: /etc/sshguard/tls'
    install -d -o root -g sshguard-api -m 0750 /etc/sshguard/tls
    install -o root -g sshguard-api -m 0640 "$certificate" /etc/sshguard/tls/fullchain.pem
    install -o root -g sshguard-api -m 0640 "$private_key" /etc/sshguard/tls/privkey.pem
fi
mkdir -p /opt/sshguard-releases
chmod 0755 /opt/sshguard-releases
stage=$(mktemp -d "/opt/sshguard-releases/$version.XXXXXX")
# A bundle may be extracted inside an existing panel website. Copy only release
# payloads, never unrelated site files, deploy.conf or certificate private keys.
for item in sshguard-api web deploy VERSION; do cp -R "$bundle/$item" "$stage/$item"; done
for item in deploy.sh README.md CHANGELOG.md CONTRIBUTING.md SECURITY.md docs; do
    [ ! -e "$bundle/$item" ] || cp -R "$bundle/$item" "$stage/$item"
done
# Public binaries/assets must be readable; configuration stays in /etc with 600.
find "$stage" -type d -exec chmod 0755 {} +
find "$stage" -type f -exec chmod 0644 {} +
chmod 0755 "$stage/sshguard-api"
printf '\n[3/5] 检查程序与数据库迁移\n'
# Validate database/migrations before switching or stopping the current API.
systemd-run --quiet --wait --pipe --collect --unit="sshguard-check-$$" -p User=sshguard-api -p StateDirectory=sshguard-api -p StateDirectoryMode=0700 -p UMask=0077 -p EnvironmentFile=/etc/sshguard/api.env "$stage/sshguard-api" -check
printf '\n[4/5] 配置站点接入（%s）\n' "$proxy_mode"
if [ "$mode" = install ] && [ "$proxy_mode" = managed ]; then
    [ ! -e /etc/nginx/conf.d/sshguard.conf ] || [ -f /etc/sshguard/nginx-managed ] || die 'Existing nginx sshguard.conf is not wizard-managed.'
    nginx_backup=$(mktemp /etc/sshguard/.nginx-backup.XXXXXX)
    if [ -f /etc/nginx/conf.d/sshguard.conf ]; then nginx_existed=true; cp /etc/nginx/conf.d/sshguard.conf "$nginx_backup"; fi
    nginx_changed=true
    mkdir -p /etc/sshguard/certs
    if [ -z "$certificate" ] && [ -z "$config_file" ]; then
        prompt "TLS certificate fullchain path (empty = automatic Let's Encrypt): "; certificate=$answer
        if [ -n "$certificate" ]; then prompt 'TLS private key path: '; private_key=$answer; fi
    fi
    if [ -n "$certificate" ]; then
        [ -f "$certificate" ] && [ -f "$private_key" ] || die 'TLS_CERT and TLS_KEY must exist.'
        openssl x509 -in "$certificate" -noout -checkend 86400 >/dev/null || die 'Certificate invalid or expires in less than one day.'
        cp "$certificate" /etc/sshguard/certs/fullchain.pem
        cp "$private_key" /etc/sshguard/certs/privkey.pem
        chmod 0600 /etc/sshguard/certs/privkey.pem
    else
        if [ -z "$email" ] && [ -z "$config_file" ]; then prompt "Email for Let's Encrypt notifications (ACME terms accepted by deployment): "; email=$answer; fi
        [ -n "$email" ] || die 'Set ACME_EMAIL or TLS_CERT/TLS_KEY.'
        command -v certbot >/dev/null 2>&1 || install_packages certbot
        mkdir -p /var/lib/sshguard/acme
        chmod 0755 /var/lib/sshguard /var/lib/sshguard/acme
        printf 'server { listen 80; server_name %s; location /.well-known/acme-challenge/ { root /var/lib/sshguard/acme; } location / { return 404; } }\n' "$domain" > /etc/nginx/conf.d/sshguard.conf
        touch /etc/sshguard/nginx-managed
        nginx -t
        systemctl enable --now nginx
        systemctl reload nginx
        certbot certonly --webroot -w /var/lib/sshguard/acme -d "$domain" --email "$email" --agree-tos --non-interactive
        ln -sf "/etc/letsencrypt/live/$domain/fullchain.pem" /etc/sshguard/certs/fullchain.pem
        ln -sf "/etc/letsencrypt/live/$domain/privkey.pem" /etc/sshguard/certs/privkey.pem
        mkdir -p /etc/letsencrypt/renewal-hooks/deploy
        printf '#!/bin/sh\nnginx -t && systemctl reload nginx\n' > /etc/letsencrypt/renewal-hooks/deploy/sshguard-nginx.sh
        chmod 0755 /etc/letsencrypt/renewal-hooks/deploy/sshguard-nginx.sh
    fi
    sed -e "s/guard.example.com/$domain/g" -e "s/127.0.0.1:8080/127.0.0.1:$app_port/g" "$bundle/deploy/nginx.conf" > /etc/nginx/conf.d/sshguard.conf
    touch /etc/sshguard/nginx-managed
    nginx -t
fi
if [ "$proxy_mode" = external ]; then
    cat > /etc/sshguard/nginx-existing.conf <<'NGINX'
# Merge inside the existing HTTPS server block; replace overlapping locations.
# Keep the existing listen/server_name/ssl_certificate directives managed by your panel.
client_max_body_size 1m;
location ~ ^/api/v1/(agent/stream|live)$ {
    proxy_pass http://127.0.0.1:8080;
    proxy_http_version 1.1;
    proxy_set_header Host $host;
    proxy_set_header Upgrade $http_upgrade;
    proxy_set_header Connection "upgrade";
    proxy_read_timeout 90s;
    proxy_send_timeout 30s;
    proxy_buffering off;
}
location / {
    proxy_pass http://127.0.0.1:8080;
    proxy_set_header Host $host;
    proxy_read_timeout 30s;
}
NGINX
    sed -i "s/127.0.0.1:8080/127.0.0.1:$app_port/g" /etc/sshguard/nginx-existing.conf
    chmod 0600 /etc/sshguard/nginx-existing.conf
    printf '  已生成 /etc/sshguard/nginx-existing.conf（兼容 OpenResty）；现有 Web 服务与证书保持原样。\n'
fi
cp "$bundle/deploy/sshguard-api.service" /etc/systemd/system/sshguard-api.service
chmod 0644 /etc/systemd/system/sshguard-api.service
systemctl daemon-reload
[ ! -e /opt/sshguard.next ] && [ ! -L /opt/sshguard.next ] || die 'Unexpected /opt/sshguard.next; inspect it first.'
if [ -L /opt/sshguard ]; then old=$(readlink /opt/sshguard); fi
ln -s "$stage" /opt/sshguard.next
mv -Tf /opt/sshguard.next /opt/sshguard
switched=true
printf '\n[5/5] 启动服务并检查健康状态\n'
systemctl enable sshguard-api
systemctl restart sshguard-api
ready=false
for attempt in 1 2 3 4 5 6 7 8 9 10; do
    if "$builtin_tls"; then
        health=$(curl -fsS --noproxy '*' --max-time 3 --resolve "$domain:$app_port:127.0.0.1" --cacert /etc/sshguard/tls/fullchain.pem "$public_origin/api/v1/health" 2>/dev/null) || health=''
    else health=$(curl -fsS --max-time 3 "http://127.0.0.1:$app_port/api/v1/health" 2>/dev/null) || health=''; fi
    if printf '%s' "$health" | grep -Fq "\"version\":\"$version\""; then ready=true; break; fi
    sleep 2
done
"$ready" || die 'API health check failed. Check journalctl -u sshguard-api.'
if [ "$proxy_mode" = managed ]; then
    systemctl enable --now nginx
    systemctl reload nginx
fi
switched=false
if [ "$public_origin" = "http://SERVER_IP:$app_port" ]; then
    detected_ip=$(hostname -I 2>/dev/null | awk '{for(i=1;i<=NF;i++) if($i ~ /^[0-9]+\.[0-9]+\.[0-9]+\.[0-9]+$/){print $i; exit}}') || detected_ip=''
    public_origin="http://${detected_ip:-127.0.0.1}:$app_port"
fi
printf '\n安装完成 · SSHGuard %s\n访问地址：%s\n' "$version" "$public_origin"
printf '管理员：admin\n'
if "$initial_setup" && [ -t 1 ] && [ -w /dev/tty ]; then
    # Only show the first-install password on the operator's terminal, never CI/redirected logs.
    initial_password=$(sed -n 's/^ADMIN_PASSWORD="\([^"]*\)"$/\1/p' /etc/sshguard/api.env)
    printf '初始密码：%s\n首次登录后请修改密码。\n' "$initial_password" > /dev/tty
    unset initial_password
else
    printf '初始密码保存在 /etc/sshguard/api.env 的 ADMIN_PASSWORD 中（已修改的登录密码保持不变）。\n'
fi
printf '可选反代、HTTPS 与 Agent 接入见文档：\nhttps://github.com/xiuzhenzhi/SSH/blob/main/docs/QUICKSTART.md\n'
