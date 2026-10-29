#!/bin/sh
# Invoked inside an ephemeral Alpine container by test-native-deploy.sh.
set -eu
apk add --no-cache curl ca-certificates openrc busybox-openrc
update-ca-certificates >/dev/null
mkdir -p /run/openrc
touch /run/openrc/softlevel
# Networking belongs to Docker; expose it to OpenRC's dependency resolver.
cat > /etc/init.d/ci-network <<'UNIT'
#!/sbin/openrc-run
depend() { provide net clock hostname localmount; }
start() { return 0; }
UNIT
chmod 0755 /etc/init.d/ci-network
rc-service ci-network start
rc-service syslog start
sh /ci/install-agent.sh "${SSHGUARD_TEST_ORIGIN:-https://guard.sshguard.test}" "$SSHGUARD_ENROLLMENT_TOKEN"
unset SSHGUARD_ENROLLMENT_TOKEN
sleep 3
[ "$(stat -c '%U:%G:%a' /var/log/sshguard)" = root:sshguard:2750 ]
[ "$(stat -c '%U:%G:%a' /var/log/sshguard/auth.log)" = root:sshguard:640 ]
! id -Gn sshguard | grep -qw wheel
su -s /bin/sh sshguard -c 'test -r /var/log/sshguard/auth.log'
logger -p authpriv.info -t sshd 'Accepted publickey for ci-fixture from 192.0.2.10 port 2222 ssh2'
logger -p authpriv.info -t sshd 'Failed password for invalid user ci-fixture from 192.0.2.10 port 2223 ssh2'
sleep 5
mv /var/log/sshguard/auth.log /var/log/sshguard/auth.log.0
rc-service syslog restart
logger -p authpriv.info -t sshd 'Accepted publickey for ci-rotation from 192.0.2.10 port 2224 ssh2'
sleep 3
[ "$(stat -c '%U:%G:%a' /var/log/sshguard/auth.log)" = root:sshguard:640 ]
su -s /bin/sh sshguard -c 'test -r /var/log/sshguard/auth.log'
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
