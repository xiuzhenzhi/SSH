#!/bin/sh
# Invoked inside an ephemeral Alpine container by test-native-deploy.sh.
set -eu
apk add --no-cache curl ca-certificates openrc
update-ca-certificates >/dev/null
mkdir -p /run/openrc
touch /run/openrc/softlevel
# Networking belongs to Docker; expose it to OpenRC's dependency resolver.
cat > /etc/init.d/ci-network <<'UNIT'
#!/sbin/openrc-run
depend() { provide net; }
start() { return 0; }
UNIT
chmod 0755 /etc/init.d/ci-network
rc-service ci-network start
syslogd
sh /ci/install-agent.sh "${SSHGUARD_TEST_ORIGIN:-https://guard.sshguard.test}" "$SSHGUARD_ENROLLMENT_TOKEN"
unset SSHGUARD_ENROLLMENT_TOKEN
sleep 3
logger -t sshd 'Accepted publickey for ci-fixture from 192.0.2.10 port 2222 ssh2'
logger -t sshd 'Failed password for invalid user ci-fixture from 192.0.2.10 port 2223 ssh2'
sleep 5
before=$(sha256sum /etc/sshguard/agent.json)
rc-service sshguard-agent stop
printf '\n' >> /usr/local/bin/sshguard-agent
rc-service sshguard-agent start
sh /ci/install-agent.sh "${SSHGUARD_TEST_ORIGIN:-https://guard.sshguard.test}" --update
sh /ci/install-agent.sh "${SSHGUARD_TEST_ORIGIN:-https://guard.sshguard.test}" --update
[ "$before" = "$(sha256sum /etc/sshguard/agent.json)" ]
rc-service sshguard-agent status
rc-service sshguard-agent stop
echo 'Alpine OpenRC install/update/idempotence passed.'
