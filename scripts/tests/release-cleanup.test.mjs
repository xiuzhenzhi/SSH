import test from 'node:test';
import assert from 'node:assert/strict';
import {readFileSync,mkdtempSync,writeFileSync,rmSync,mkdirSync} from 'node:fs';
import {join} from 'node:path';
import {tmpdir} from 'node:os';
import {spawnSync} from 'node:child_process';

test('cleanup removes only inactive recognized releases, preserving current files and external links',t=>{
 const root=mkdtempSync(join(tmpdir(),'sshguard-prune-'));
 t.after(()=>rmSync(root,{recursive:true,force:true}));
 const source=readFileSync('sshguard.sh','utf8');
 const helper=source.slice(source.indexOf('prune_releases() ('),source.indexOf('management_action() {'));
 const shell=process.env.SSHGUARD_TEST_SHELL||(process.platform==='win32'?join(process.env.ProgramFiles,'Git/bin/sh.exe'):'sh');
 // Git Bash has no Linux mount table; these temporary Windows directories are not mounts.
 if(process.platform==='win32'){
  mkdirSync(join(root,'bin'));
  writeFileSync(join(root,'bin','mountpoint'),'#!/bin/sh\nexit 1\n',{mode:0o755});
 }
 writeFileSync(join(root,'check.sh'),`set -eu
${helper}
cd "$TEST_ROOT"
root=$(pwd -P)
if [ -d "$root/bin" ]; then export PATH="$root/bin:$PATH"; fi
mkdir releases external
printf keep > external/data
for name in 0.3.16.aaaaaa 0.3.17.bbbbbb 0.3.18.cccccc 0.3.15.dddddd; do
 mkdir -p "releases/$name/web" "releases/$name/deploy"
 printf '%s' "$name" | cut -d. -f1-3 > "releases/$name/VERSION"
 touch "releases/$name/sshguard-api"
done
printf keep > releases/0.3.15.dddddd/database.db
mkdir releases/unrelated
printf keep > releases/unrelated/data
ln -s "$root/external" releases/0.3.14.eeeeee
ln -s "$root/external" releases/0.3.16.aaaaaa/web/external
prune_releases "$root/releases" "$root/releases/0.3.18.cccccc"
test ! -e releases/0.3.16.aaaaaa
test ! -e releases/0.3.17.bbbbbb
test -f releases/0.3.18.cccccc/sshguard-api
test -f releases/0.3.15.dddddd/database.db
test -f releases/unrelated/data
test -f external/data
test -L releases/0.3.14.eeeeee
if prune_releases "$root/releases" "$root/external"; then exit 1; fi
test -f releases/0.3.18.cccccc/sshguard-api
`);
 const result=spawnSync(shell,[join(root,'check.sh')],{env:{...process.env,TEST_ROOT:root,MSYS:'winsymlinks:nativestrict'},encoding:'utf8',timeout:15000});
 assert.equal(result.status,0,result.stdout+result.stderr);
});
