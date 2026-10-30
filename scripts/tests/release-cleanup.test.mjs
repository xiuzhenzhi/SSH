import test from 'node:test';
import assert from 'node:assert/strict';
import {readFileSync,mkdtempSync,writeFileSync,rmSync,mkdirSync} from 'node:fs';
import {join} from 'node:path';
import {tmpdir} from 'node:os';
import {spawnSync} from 'node:child_process';
test('controller cleanup preserves Agent files, extra data and external links',t=>{
 const root=mkdtempSync(join(tmpdir(),'sshguard-prune-'));
 t.after(()=>rmSync(root,{recursive:true,force:true}));
 const shell=process.env.SSHGUARD_TEST_SHELL||(process.platform==='win32'?join(process.env.ProgramFiles,'Git/bin/sh.exe'):'sh');
 const unix=spawnSync(shell,['-c','cd "$TEST_ROOT" && pwd -P'],{env:{...process.env,TEST_ROOT:root},encoding:'utf8'}).stdout.trim();
 assert.ok(unix.startsWith('/'));
 const source=readFileSync('sshguard.sh','utf8');
 const helper=source.slice(source.indexOf('remove_release_payload() ('),source.indexOf('management_action() {')).replaceAll('/opt/',unix+'/opt/');
 if(process.platform==='win32'){
  mkdirSync(join(root,'bin'));
  writeFileSync(join(root,'bin','mountpoint'),'#!/bin/sh\nexit 1\n',{mode:0o755});
 }
 writeFileSync(join(root,'check.sh'),`set -eu
${helper}
cd "$TEST_ROOT"
root=$(pwd -P)
if [ -d "$root/bin" ]; then export PATH="$root/bin:$PATH"; fi
mkdir -p opt/SSHGuard/Server/data opt/SSHGuard/Agent external
printf keep > external/data
touch opt/SSHGuard/Server/sshguard-api opt/SSHGuard/Agent/agent.json
printf config > opt/SSHGuard/Server/api.env
printf database > opt/SSHGuard/Server/data/sshguard.db
mkdir -p opt/.SSHGuard-backup.aaaaaa/web opt/.SSHGuard-backup.bbbbbb
touch opt/.SSHGuard-backup.aaaaaa/sshguard-api opt/.SSHGuard-backup.bbbbbb/database.db
ln -s "$root/external" opt/.SSHGuard-backup.aaaaaa/web/external
ln -s "$root/external" opt/.SSHGuard-backup.cccccc
remove_release_payload "$root/opt/.SSHGuard-backup.aaaaaa"
test ! -e opt/.SSHGuard-backup.aaaaaa
if remove_release_payload "$root/opt/.SSHGuard-backup.bbbbbb"; then exit 1; fi
if remove_release_payload "$root/opt/.SSHGuard-backup.cccccc"; then exit 1; fi
if remove_release_payload "$root/opt/SSHGuard/Agent"; then exit 1; fi
test -f opt/.SSHGuard-backup.bbbbbb/database.db
test -f opt/SSHGuard/Agent/agent.json
test -f external/data
test -f opt/SSHGuard/Server/sshguard-api
if remove_release_payload "$root/opt/SSHGuard/Server"; then exit 1; fi
if remove_program_item data; then exit 1; fi
if remove_program_item api.env; then exit 1; fi
if remove_program_item ../Agent; then exit 1; fi
remove_program_item sshguard-api
test ! -e opt/SSHGuard/Server/sshguard-api
test "$(cat opt/SSHGuard/Server/api.env)" = config
test "$(cat opt/SSHGuard/Server/data/sshguard.db)" = database
test -f opt/SSHGuard/Agent/agent.json
`);
 const result=spawnSync(shell,[join(root,'check.sh')],{env:{...process.env,TEST_ROOT:root,MSYS:'winsymlinks:nativestrict'},encoding:'utf8',timeout:15000});
 assert.equal(result.status,0,result.stdout+result.stderr);
});
