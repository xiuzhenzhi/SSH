#!/bin/sh
# Invoked inside an ephemeral Alpine container by test-native-deploy.sh.
set -eu
apk add --no-cache curl ca-certificates openrc busybox-openrc
update-ca-certificates >/dev/null
mkdir -p /run/openrc
touch /run/openrc/softlevel
# Container networking and clock are already provided by Docker.
cat > /etc/init.d/ci-network <<'UNIT'
#!/sbin/openrc-run
depend() { provide net clock; }
start() { return 0; }
UNIT
chmod 0755 /etc/init.d/ci-network
# Docker also provides the hostname and mounted filesystems. Represent those
# completed boot stages without changing container/host mounts or hostname.
for dependency in localmount hostname; do
    cat > "/etc/init.d/$dependency" <<'UNIT'
#!/sbin/openrc-run
start() { return 0; }
UNIT
    chmod 0755 "/etc/init.d/$dependency"
done
# rc-status initializes all OpenRC runtime state/lock directories. A bare
# softlevel file is insufficient on OpenRC 0.55 in Alpine 3.21.
rc-status >/dev/null
rc-service --nodeps ci-network start
rc-service --nodeps localmount start
rc-service --nodeps hostname start
rc-service syslog start
sh /ci/install-agent.sh "${SSHGUARD_TEST_ORIGIN:-https://guard.sshguard.test}" "$SSHGUARD_ENROLLMENT_TOKEN"
unset SSHGUARD_ENROLLMENT_TOKEN
sleep 3
[ "$(stat -c '%U:%G:%a' /opt/SSHGuard/Agent/logs)" = root:sshguard:2750 ]
[ "$(stat -c '%U:%G:%a' /opt/SSHGuard/Agent/logs/auth.log)" = root:sshguard:640 ]
! id -Gn sshguard | grep -qw wheel
su -s /bin/sh sshguard -c 'test -r /opt/SSHGuard/Agent/logs/auth.log'
logger -p authpriv.info -t sshd 'Accepted publickey for ci-fixture from 192.0.2.10 port 2222 ssh2'
logger -p authpriv.info -t sshd 'Failed password for invalid user ci-fixture from 192.0.2.10 port 2223 ssh2'
sleep 5
mv /opt/SSHGuard/Agent/logs/auth.log /opt/SSHGuard/Agent/logs/auth.log.0
rc-service syslog restart
logger -p authpriv.info -t sshd 'Accepted publickey for ci-rotation from 192.0.2.10 port 2224 ssh2'
sleep 3
[ "$(stat -c '%U:%G:%a' /opt/SSHGuard/Agent/logs/auth.log)" = root:sshguard:640 ]
su -s /bin/sh sshguard -c 'test -r /opt/SSHGuard/Agent/logs/auth.log'
before=$(sha256sum /opt/SSHGuard/Agent/agent.json)
rc-service sshguard-agent stop
printf '\n' >> /opt/SSHGuard/Agent/sshguard-agent-bin
rc-service sshguard-agent start
sh /ci/install-agent.sh "${SSHGUARD_TEST_ORIGIN:-https://guard.sshguard.test}" --update
sh /ci/install-agent.sh "${SSHGUARD_TEST_ORIGIN:-https://guard.sshguard.test}" --update
[ "$before" = "$(sha256sum /opt/SSHGuard/Agent/agent.json)" ]
rc-service sshguard-agent status
rc-service sshguard-agent stop
echo 'Alpine OpenRC install/update/idempotence passed.'
