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
rc-service syslog start
sh /ci/install-agent.sh https://guard.sshguard.test "$SSHGUARD_ENROLLMENT_TOKEN"
unset SSHGUARD_ENROLLMENT_TOKEN
before=$(sha256sum /etc/sshguard/agent.json)
rc-service sshguard-agent stop
printf '\n' >> /usr/local/bin/sshguard-agent
rc-service sshguard-agent start
sh /ci/install-agent.sh https://guard.sshguard.test --update
sh /ci/install-agent.sh https://guard.sshguard.test --update
[ "$before" = "$(sha256sum /etc/sshguard/agent.json)" ]
rc-service sshguard-agent status
rc-service sshguard-agent stop
echo 'Alpine OpenRC install/update/idempotence passed.'
