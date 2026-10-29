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
case "$driver" in local-postgres|sqlite) ;; *) exit 1 ;; esac
printf 'DOMAIN=guard.sshguard.test\nDB_DRIVER=%s\nTLS_CERT=%s/cert.pem\nTLS_KEY=%s/key.pem\n' "$driver" "$tmp" "$tmp" > "$tmp/deploy.conf"
chmod 600 "$tmp/deploy.conf"
sh "$bundle/deploy.sh" --config "$tmp/deploy.conf"
check_sqlite_state() {
    [ "$driver" = sqlite ] || return 0
    [ "$(stat -c %a /var/lib/sshguard-api)" = 700 ]
    [ "$(stat -c %U /var/lib/sshguard-api)" = sshguard-api ]
    [ -s /var/lib/sshguard-api/sshguard.db ]
    [ "$(stat -c %a /var/lib/sshguard-api/sshguard.db)" = 600 ]
    curl -fsS http://127.0.0.1:8080/api/v1/health | grep -Fq '"database":"sqlite"'
}
check_sqlite_state
curl -fsS https://guard.sshguard.test/api/v1/health >/dev/null
export NODE_EXTRA_CA_CERTS="$tmp/cert.pem"
export SSHGUARD_TEST_STATE="$tmp"
node scripts/test-native-api.mjs enroll
token=$(cat "$tmp/enrollment")
sh "$bundle/web/downloads/install-agent.sh" https://guard.sshguard.test "$token"
unset token
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
sh "$bundle/web/downloads/install-agent.sh" https://guard.sshguard.test --update
[ "$before_config" = "$(sha256sum /etc/sshguard/agent.json)" ]
cmp /usr/local/bin/sshguard-agent "$bundle/web/downloads/sshguard-agent-linux-amd64"
# Idempotent second update must also succeed without enrollment.
sh "$bundle/web/downloads/install-agent.sh" https://guard.sshguard.test --update
node scripts/test-native-api.mjs verify
# Simulate a checksum-verified but broken release served by the test platform.
download=/opt/sshguard/web/downloads/sshguard-agent-linux-amd64
cp "$download" "$tmp/good-agent"
cp "$download.sha256" "$tmp/good-hash"
printf '#!/bin/sh\ncase "$1" in -version|-check) exit 0 ;; *) exit 1 ;; esac\n' > "$download"
sha256sum "$download" > "$download.sha256"
if sh "$bundle/web/downloads/install-agent.sh" https://guard.sshguard.test --update; then echo 'Broken Agent update unexpectedly succeeded' >&2; exit 1; fi
systemctl is-active --quiet sshguard-agent
[ "$before_config" = "$(sha256sum /etc/sshguard/agent.json)" ]
cmp /usr/local/bin/sshguard-agent "$tmp/good-agent"
cp "$tmp/good-agent" "$download"
cp "$tmp/good-hash" "$download.sha256"
old=$(readlink /opt/sshguard)
sh "$bundle/deploy.sh" --update
check_sqlite_state
[ "$old" != "$(readlink /opt/sshguard)" ]
[ "$before_env" = "$(sha256sum /etc/sshguard/api.env)" ]
node scripts/test-native-api.mjs verify
# Preflight must reject an invalid binary before switching the working deployment.
mkdir "$tmp/broken"
cp -R "$bundle/." "$tmp/broken/"
printf '#!/bin/sh\nexit 1\n' > "$tmp/broken/sshguard-api"
old=$(readlink /opt/sshguard)
if sh "$tmp/broken/deploy.sh" --update; then echo 'Invalid API accepted' >&2; exit 1; fi
[ "$old" = "$(readlink /opt/sshguard)" ]
curl -fsS https://guard.sshguard.test/api/v1/health >/dev/null
echo 'Native deploy, HTTPS, real systemd Agent install/update/idempotence/rollback, and platform update passed.'
SSHGUARD_TEST_SERVER_NAME=CI-Alpine-OpenRC node scripts/test-native-api.mjs enroll
SSHGUARD_ENROLLMENT_TOKEN=$(cat "$tmp/enrollment")
export SSHGUARD_ENROLLMENT_TOKEN
docker run --rm --network host --add-host guard.sshguard.test:127.0.0.1 \
    -e SSHGUARD_ENROLLMENT_TOKEN \
    -v "$(pwd)/scripts:/ci:ro" \
    -v "$tmp/cert.pem:/usr/local/share/ca-certificates/sshguard-ci.crt:ro" \
    alpine:3.22 sh /ci/test-alpine-agent.sh
unset SSHGUARD_ENROLLMENT_TOKEN
node scripts/test-native-api.mjs verify-alpine
