#!/bin/sh
# Run from an extracted, checksum-verified native release. Ubuntu/Debian + systemd.
# No shell sourcing/eval of user configuration. Secrets never appear in argv.
set -eu
umask 077
die() { echo "$*" >&2; exit 1; }
[ "$(id -u)" -eq 0 ] || die 'Run with sudo sh deploy.sh.'
bundle=$(CDPATH='' cd -- "$(dirname -- "$0")" && pwd)
[ -f "$bundle/sshguard-api" ] && [ -f "$bundle/web/index.html" ] || die 'Run deploy.sh from an extracted native release, not the source scripts directory.'
version=$(cat "$bundle/VERSION")
printf '%s' "$version" | grep -Eq '^[A-Za-z0-9.+_-]{1,32}$' || die 'Invalid release version.'
command -v apt-get >/dev/null 2>&1 && [ -d /run/systemd/system ] || die 'Central deployment requires Ubuntu/Debian with systemd.'
mode=install
config_file=''
while [ "$#" -gt 0 ]; do
    case "$1" in
        --update) mode=update; shift ;;
        --config) [ "$#" -ge 2 ] || die '--config requires a file'; config_file=$2; shift 2 ;;
        *) die 'Usage: deploy.sh [--update] [--config /path/deploy.conf]' ;;
    esac
done
domain=''; driver=''; dsn=''; certificate=''; private_key=''; email=''; admin_password=''
if [ -n "$config_file" ]; then
    [ -f "$config_file" ] && [ ! -L "$config_file" ] || die 'Configuration must be a regular file.'
    [ "$(stat -c %a "$config_file")" = 600 ] || die 'Set configuration permissions to 600.'
    while IFS= read -r line || [ -n "$line" ]; do
        case "$line" in ''|'#'*) continue ;; esac
        key=${line%%=*}; value=${line#*=}
        case "$key" in
            DOMAIN) domain=$value ;; DB_DRIVER) driver=$value ;; DATABASE_URL) dsn=$value ;;
            TLS_CERT) certificate=$value ;; TLS_KEY) private_key=$value ;; ACME_EMAIL) email=$value ;;
            ADMIN_PASSWORD) admin_password=$value ;; *) die "Unknown configuration key: $key" ;;
        esac
    done < "$config_file"
fi
prompt() { printf '%s' "$1" >&2; IFS= read -r answer; }
for path in /opt/sshguard-releases /etc/sshguard /etc/sshguard/api.env /etc/sshguard/deploy-managed; do
    [ ! -L "$path" ] || die "Refusing symlink: $path"
done
if [ -e /opt/sshguard ] && [ ! -L /opt/sshguard ]; then die 'Existing /opt/sshguard is a directory. Follow the legacy migration instructions before using this updater.'; fi
if [ "$mode" = update ]; then
    [ -L /opt/sshguard ] && [ -f /etc/sshguard/deploy-managed ] || die 'No wizard-managed installation found.'
elif [ -L /opt/sshguard ]; then die 'Already installed. Use --update.'; fi
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
if [ "$mode" = install ]; then
    apt-get update
    apt-get install -y nginx curl openssl ca-certificates
fi
command -v curl >/dev/null 2>&1 || die 'curl is required.'
if [ -f /etc/sshguard/api.env ]; then
    [ -f /etc/sshguard/deploy-managed ] || die 'Existing api.env is not wizard-managed; migrate explicitly, do not overwrite credentials.'
    domain=$(sed -n 's/^PUBLIC_URL="https:\/\/\([A-Za-z0-9.-]*\)"$/\1/p' /etc/sshguard/api.env)
else
    [ "$mode" = install ] || die 'Missing /etc/sshguard/api.env.'
    if [ -z "$domain" ] && [ -z "$config_file" ]; then prompt 'Public DNS name (e.g. guard.example.com): '; domain=$answer; fi
    printf '%s' "$domain" | grep -Eq '^[A-Za-z0-9][A-Za-z0-9.-]*\.[A-Za-z0-9-]+$' || die 'Set DOMAIN to your public DNS name.'
    if [ -z "$driver" ] && [ -z "$config_file" ]; then prompt 'Database [local-postgres / postgres / mysql] (default local-postgres): '; driver=${answer:-local-postgres}; fi
    driver=${driver:-local-postgres}
    case "$driver" in
        local-postgres)
            [ -z "$dsn" ] || die 'Use DB_DRIVER=postgres for an existing database.'
            apt-get install -y postgresql
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
        *) die 'DB_DRIVER must be local-postgres, postgres or mysql.' ;;
    esac
    [ -n "$admin_password" ] || admin_password=$(openssl rand -hex 20)
    [ "${#admin_password}" -ge 12 ] && [ "${#admin_password}" -le 72 ] || die 'ADMIN_PASSWORD must be 12–72 bytes.'
    mkdir -p /etc/sshguard
    chmod 0750 /etc/sshguard
    # systemd EnvironmentFile escaping, never interpreted by a shell.
    env_line() { printf '%s="' "$1"; printf '%s' "$2" | sed 's/\\/\\\\/g; s/"/\\"/g'; printf '"\n'; }
    {
        env_line DB_DRIVER "$driver"; env_line DATABASE_URL "$dsn"; env_line PUBLIC_URL "https://$domain"
        env_line LISTEN_ADDR 127.0.0.1:8080; env_line ADMIN_USERNAME admin; env_line ADMIN_PASSWORD "$admin_password"; env_line GIN_MODE release
    } > /etc/sshguard/api.env
    chmod 0600 /etc/sshguard/api.env
    touch /etc/sshguard/deploy-managed
    unset dsn admin_password
fi
printf '%s' "$domain" | grep -Eq '^[A-Za-z0-9][A-Za-z0-9.-]*\.[A-Za-z0-9-]+$' || die 'Invalid managed PUBLIC_URL.'
if ! id sshguard-api >/dev/null 2>&1; then useradd --system --user-group --no-create-home --shell /usr/sbin/nologin sshguard-api; fi
mkdir -p /opt/sshguard-releases
chmod 0755 /opt/sshguard-releases
stage=$(mktemp -d "/opt/sshguard-releases/$version.XXXXXX")
cp -R "$bundle/." "$stage/"
# Public binaries/assets must be readable; configuration stays in /etc with 600.
find "$stage" -type d -exec chmod 0755 {} +
find "$stage" -type f -exec chmod 0644 {} +
chmod 0755 "$stage/sshguard-api"
# Validate database/migrations before switching or stopping the current API.
systemd-run --quiet --wait --pipe --collect --unit="sshguard-check-$$" -p User=sshguard-api -p EnvironmentFile=/etc/sshguard/api.env "$stage/sshguard-api" -check
if [ "$mode" = install ]; then
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
        apt-get install -y certbot
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
    sed "s/guard.example.com/$domain/g" "$bundle/deploy/nginx.conf" > /etc/nginx/conf.d/sshguard.conf
    touch /etc/sshguard/nginx-managed
    nginx -t
fi
cp "$bundle/deploy/sshguard-api.service" /etc/systemd/system/sshguard-api.service
chmod 0644 /etc/systemd/system/sshguard-api.service
systemctl daemon-reload
[ ! -e /opt/sshguard.next ] && [ ! -L /opt/sshguard.next ] || die 'Unexpected /opt/sshguard.next; inspect it first.'
if [ -L /opt/sshguard ]; then old=$(readlink /opt/sshguard); fi
ln -s "$stage" /opt/sshguard.next
mv -Tf /opt/sshguard.next /opt/sshguard
switched=true
systemctl enable sshguard-api
systemctl restart sshguard-api
ready=false
for attempt in 1 2 3 4 5 6 7 8 9 10; do
    if curl -fsS --max-time 3 http://127.0.0.1:8080/api/v1/health | grep -Fq "\"version\":\"$version\""; then ready=true; break; fi
    sleep 2
done
"$ready" || die 'API health check failed. Check journalctl -u sshguard-api.'
systemctl enable --now nginx
systemctl reload nginx
switched=false
echo "SSHGuard $version ready: https://$domain"
echo 'Username: admin. Initial password: ADMIN_PASSWORD in /etc/sshguard/api.env (sudo access required).'
echo 'Change the password in the console after first login. No credentials were printed.'
echo 'Previous releases are retained in /opt/sshguard-releases. Database migrations are never automatically reversed.'
