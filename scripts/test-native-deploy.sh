#!/bin/sh
# Destructive to fixed SSHGuard paths: ONLY run on a disposable CI VM.
set -eu
[ "${GITHUB_ACTIONS:-}" = true ] || { echo 'Disposable GitHub Actions VM only.' >&2; exit 1; }
[ "$(id -u)" -eq 0 ] || exit 1
bundle=$(realpath "$1")
tmp=$(mktemp -d)
trap 'rm -rf -- "$tmp"' EXIT
openssl req -x509 -newkey rsa:2048 -nodes -days 2 -subj /CN=guard.sshguard.test -addext subjectAltName=DNS:guard.sshguard.test -keyout "$tmp/key.pem" -out "$tmp/cert.pem" 2>/dev/null
printf '127.0.0.1 guard.sshguard.test\n' >> /etc/hosts
cp "$tmp/cert.pem" /usr/local/share/ca-certificates/sshguard-ci.crt
update-ca-certificates >/dev/null
driver=${SSHGUARD_NATIVE_DB:-local-postgres}
proxy_mode=${SSHGUARD_NATIVE_PROXY:-managed}
case "$driver" in local-postgres|sqlite) ;; *) exit 1 ;; esac
case "$proxy_mode" in managed|external|standalone) ;; *) exit 1 ;; esac
origin=https://guard.sshguard.test
if [ "$proxy_mode" = standalone ]; then origin=https://guard.sshguard.test:8080; fi
export SSHGUARD_TEST_ORIGIN="$origin"
deploy_path=$PATH
if [ "$proxy_mode" = external ]; then
    apt-get update
    apt-get install -y nginx curl openssl ca-certificates
    printf 'server { listen 80; server_name existing.sshguard.test; location / { return 200 "existing site"; } }\n' > /etc/nginx/conf.d/existing-site.conf
    systemctl enable --now nginx
    nginx -t && systemctl reload nginx
    existing_config=$(sha256sum /etc/nginx/conf.d/existing-site.conf)
fi
if [ "$proxy_mode" != managed ]; then
    mkdir "$tmp/bin"
    # Fail if external-mode installation or upgrade tries to control Nginx or apt.
    for command in nginx openresty apt-get; do
        printf '#!/bin/sh\necho "Standalone/external deploy must not invoke %s" >&2\nexit 99\n' "$command" > "$tmp/bin/$command"
    done
    real_systemctl=$(command -v systemctl)
    printf '#!/bin/sh\ncase " $* " in *nginx*) echo "External deploy must not control nginx" >&2; exit 99 ;; esac\nexec "%s" "$@"\n' "$real_systemctl" > "$tmp/bin/systemctl"
    chmod 755 "$tmp/bin/"*
    deploy_path="$tmp/bin:$PATH"
fi
printf 'DOMAIN=guard.sshguard.test\nDB_DRIVER=%s\nPROXY_MODE=%s\nTLS_CERT=%s/cert.pem\nTLS_KEY=%s/key.pem\n' "$driver" "$proxy_mode" "$tmp" "$tmp" > "$tmp/deploy.conf"
chmod 600 "$tmp/deploy.conf"
PATH="$deploy_path" sh "$bundle/deploy.sh" --config "$tmp/deploy.conf"
if [ "$proxy_mode" = external ]; then
    [ "$existing_config" = "$(sha256sum /etc/nginx/conf.d/existing-site.conf)" ]
    [ "$(cat /etc/sshguard/proxy-mode)" = external ]
    [ ! -e /etc/sshguard/nginx-managed ]
    printf 'server { listen 443 ssl; server_name guard.sshguard.test; ssl_certificate %s/cert.pem; ssl_certificate_key %s/key.pem; include /etc/sshguard/nginx-existing.conf; }\n' "$tmp" "$tmp" > /etc/nginx/conf.d/sshguard-panel.conf
    nginx -t && systemctl reload nginx
fi
check_sqlite_state() {
    [ "$driver" = sqlite ] || return 0
    [ "$(stat -c %a /var/lib/sshguard-api)" = 700 ]
    [ "$(stat -c %U /var/lib/sshguard-api)" = sshguard-api ]
    [ -s /var/lib/sshguard-api/sshguard.db ]
    [ "$(stat -c %a /var/lib/sshguard-api/sshguard.db)" = 600 ]
    curl -fsS "$origin/api/v1/health" | grep -Fq '"database":"sqlite"'
}
check_sqlite_state
PATH="$deploy_path" sh "$bundle/deploy.sh" --status > "$tmp/status.txt"
if grep -Eq 'ADMIN_PASSWORD|ENCRYPTION_KEY|DATABASE_URL' "$tmp/status.txt"; then echo 'Management status leaked secrets' >&2; exit 1; fi
PATH="$deploy_path" sh "$bundle/deploy.sh" --restart
curl --retry 10 --retry-connrefused --retry-delay 1 -fsS "$origin/api/v1/health" >/dev/null
curl -fsS "$origin/" | grep -q '<html'
export NODE_EXTRA_CA_CERTS="$tmp/cert.pem"
export SSHGUARD_TEST_STATE="$tmp"
node scripts/test-native-api.mjs enroll
token=$(cat "$tmp/enrollment")
sh "$bundle/web/downloads/install-agent.sh" "$origin" "$token"
unset token
[ "$(stat -c %a /etc/sshguard)" = 711 ]
if su sshguard -s /bin/sh -c 'test -r /etc/sshguard/api.env'; then echo 'Agent can read API secrets' >&2; exit 1; fi
if su sshguard-api -s /bin/sh -c 'test -r /etc/sshguard/agent.json'; then echo 'API can read Agent credentials' >&2; exit 1; fi
if [ "$proxy_mode" = standalone ]; then
    su sshguard-api -s /bin/sh -c 'test -r /etc/sshguard/tls/privkey.pem'
    if su sshguard -s /bin/sh -c 'test -r /etc/sshguard/tls/privkey.pem'; then echo 'Agent can read API TLS key' >&2; exit 1; fi
fi
systemctl is-active --quiet sshguard-agent
sleep 3
logger -t sshd 'Accepted publickey for ci-fixture from 192.0.2.10 port 2222 ssh2'
logger -t sshd 'Failed password for invalid user ci-fixture from 192.0.2.10 port 2223 ssh2'
before_config=$(sha256sum /etc/sshguard/agent.json)
before_env=$(sha256sum /etc/sshguard/api.env)
# Give the installed ELF a harmless trailing byte so update exercises replacement.
systemctl stop sshguard-agent
printf '\n' >> /usr/local/bin/sshguard-agent
systemctl start sshguard-agent
sh "$bundle/web/downloads/install-agent.sh" "$origin" --update
[ "$before_config" = "$(sha256sum /etc/sshguard/agent.json)" ]
cmp /usr/local/bin/sshguard-agent "$bundle/web/downloads/sshguard-agent-linux-amd64"
# Idempotent second update must also succeed without enrollment.
sh "$bundle/web/downloads/install-agent.sh" "$origin" --update
node scripts/test-native-api.mjs verify
# The updater is a fixed local maintenance service, not a command execution channel.
systemctl stop sshguard-agent-update.timer
systemctl stop sshguard-agent
printf '\n' >> /usr/local/bin/sshguard-agent
systemctl start sshguard-agent
sleep 3
node scripts/test-native-api.mjs queue-update
systemctl start sshguard-agent-update.service
node scripts/test-native-api.mjs verify-update
# Simulate a checksum-verified but broken release served by the test platform.
download=/opt/sshguard/web/downloads/sshguard-agent-linux-amd64
cp "$download" "$tmp/good-agent"
cp "$download.sha256" "$tmp/good-hash"
printf '#!/bin/sh\ncase "$1" in -version|-check) exit 0 ;; *) exit 1 ;; esac\n' > "$download"
sha256sum "$download" > "$download.sha256"
if sh "$bundle/web/downloads/install-agent.sh" "$origin" --update; then echo 'Broken Agent update unexpectedly succeeded' >&2; exit 1; fi
systemctl is-active --quiet sshguard-agent
[ "$before_config" = "$(sha256sum /etc/sshguard/agent.json)" ]
cmp /usr/local/bin/sshguard-agent "$tmp/good-agent"
cp "$tmp/good-agent" "$download"
cp "$tmp/good-hash" "$download.sha256"
old=$(readlink /opt/sshguard)
PATH="$deploy_path" sh "$bundle/deploy.sh" --update
if [ "$proxy_mode" = external ]; then
    [ "$existing_config" = "$(sha256sum /etc/nginx/conf.d/existing-site.conf)" ]
    [ "$(cat /etc/sshguard/proxy-mode)" = external ]
fi
check_sqlite_state
[ "$old" != "$(readlink /opt/sshguard)" ]
[ "$before_env" = "$(sha256sum /etc/sshguard/api.env)" ]
node scripts/test-native-api.mjs verify
# Preflight must reject an invalid binary before switching the working deployment.
mkdir "$tmp/broken"
cp -R "$bundle/." "$tmp/broken/"
printf '#!/bin/sh\nexit 1\n' > "$tmp/broken/sshguard-api"
old=$(readlink /opt/sshguard)
if PATH="$deploy_path" sh "$tmp/broken/deploy.sh" --update; then echo 'Invalid API accepted' >&2; exit 1; fi
[ "$old" = "$(readlink /opt/sshguard)" ]
curl -fsS "$origin/api/v1/health" >/dev/null
echo 'Native deploy, HTTPS, real systemd Agent install/update/idempotence/rollback, and platform update passed.'
SSHGUARD_TEST_SERVER_NAME=CI-Alpine-OpenRC node scripts/test-native-api.mjs enroll
SSHGUARD_ENROLLMENT_TOKEN=$(cat "$tmp/enrollment")
export SSHGUARD_ENROLLMENT_TOKEN
pulled=false
for attempt in 1 2 3; do
    if docker pull alpine:3.22; then pulled=true; break; fi
    sleep 5
done
"$pulled" || { echo 'Alpine test image download failed after retries' >&2; exit 1; }
docker run --rm --network host --add-host guard.sshguard.test:127.0.0.1 \
    -e SSHGUARD_ENROLLMENT_TOKEN -e SSHGUARD_TEST_ORIGIN \
    -v "$(pwd)/scripts:/ci:ro" \
    -v "$tmp/cert.pem:/usr/local/share/ca-certificates/sshguard-ci.crt:ro" \
    alpine:3.22 sh /ci/test-alpine-agent.sh
unset SSHGUARD_ENROLLMENT_TOKEN
node scripts/test-native-api.mjs verify-alpine
sh "$bundle/web/downloads/uninstall-agent.sh"
if systemctl is-active --quiet sshguard-agent; then echo 'Agent still running after uninstall' >&2; exit 1; fi
[ ! -e /usr/local/bin/sshguard-agent ]
[ ! -e /etc/systemd/system/sshguard-agent-update.timer ]
[ -f /etc/sshguard/agent.json ]
curl -fsS "$origin/api/v1/health" >/dev/null
