#!/bin/sh
# Only invoked on a disposable Actions VM.
set -eu
[ "${GITHUB_ACTIONS:-}" = true ] && [ "$(id -u)" -eq 0 ] || exit 1
bundle=$(realpath "$1")
tmp=$(mktemp -d)
trap 'rm -rf -- "$tmp"' EXIT
mkdir "$tmp/bin"
for tool in nginx openresty apt-get; do
    printf '#!/bin/sh\nexit 99\n' > "$tmp/bin/$tool"
    chmod 755 "$tmp/bin/$tool"
done
# The terminal menu can exit without installing; no database/domain prompts.
printf '0\n' | script -q -c "sh '$bundle/deploy.sh'" "$tmp/menu.log" >/dev/null
grep -q 'SSHGuard' "$tmp/menu.log"
# Even with an existing proxy, installation only asks for the port.
printf '\n' | PATH="$tmp/bin:$PATH" sh "$bundle/deploy.sh" > "$tmp/install.log"
export SSHGUARD_DIRECT_LOG="$tmp/install.log"
[ "$(cat /etc/sshguard/proxy-mode)" = standalone ]
grep -Fxq 'PUBLIC_URL=""' /etc/sshguard/api.env
grep -Fxq 'LISTEN_ADDR="0.0.0.0:8080"' /etc/sshguard/api.env
node --input-type=module <<'JS'
import {readFileSync} from 'node:fs';
const token=JSON.parse(readFileSync('/etc/sshguard/api.env','utf8').split('\n').find(x=>x.startsWith('SETUP_TOKEN=')).slice(12));
if(readFileSync(process.env.SSHGUARD_DIRECT_LOG,'utf8').includes(token))throw Error('Setup token leaked into redirected installation log');
const password='Direct-install-test-987';
const origin='http://127.0.0.1:8080';
for (const action of ['test','complete']) {
 const response=await fetch(origin+'/api/v1/setup/'+action,{method:'POST',headers:{Origin:origin,'Content-Type':'application/json','X-Setup-Token':token},body:JSON.stringify({driver:'sqlite',username:'admin',password})});
 if(!response.ok)throw Error(await response.text());
}
let ready=false;
for(let i=0;i<30;i++) {
 await new Promise(r=>setTimeout(r,1000));
 try{const r=await fetch(origin+'/api/v1/health');if((await r.json()).data.status==='ok'){ready=true;break;}}catch{}
}
if(!ready)throw Error('Setup did not transition to the console');
for(const host of ['127.0.0.1','localhost']) {
 const origin=`http://${host}:8080`;
 const body=JSON.stringify({username:'admin',password});
 for(const [sentOrigin,status] of [[origin,200],['http://attacker.example',403]]) {
  const response=await fetch(origin+'/api/v1/auth/login',{method:'POST',headers:{Origin:sentOrigin,'Content-Type':'application/json','X-Forwarded-Host':'attacker.example'},body});
  if(response.status!==status)throw Error(`Direct login ${host}: ${response.status}, expected ${status}`);
 }
 const response=await fetch(origin+'/');if(!response.ok || !(await response.text()).includes('<html'))throw Error('Console unavailable');
}
JS
before=$(sha256sum /etc/sshguard/api.env)
PATH="$tmp/bin:$PATH" sh "$bundle/deploy.sh" --update
[ "$before" = "$(sha256sum /etc/sshguard/api.env)" ]
curl -fsS http://127.0.0.1:8080/api/v1/health >/dev/null
test -s /var/lib/sshguard-api/setup.json
sh "$bundle/uninstall.sh" --yes
test ! -e /etc/systemd/system/sshguard-api.service
test ! -e /opt/sshguard
test -s /var/lib/sshguard-api/setup.json
test -s /var/lib/sshguard-api/sshguard.db
test -s /etc/sshguard/api.env
# Legacy directory installation: standalone uninstaller works without upgrading.
mkdir /opt/sshguard
printf 'legacy binary\n' > /opt/sshguard/sshguard-api
printf 'keep me\n' > /opt/sshguard/legacy-data
sh "$bundle/uninstall.sh" --yes
test ! -e /opt/sshguard/sshguard-api
test -s /opt/sshguard/legacy-data
sh "$bundle/uninstall.sh" --yes
