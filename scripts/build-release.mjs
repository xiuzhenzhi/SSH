import { execFileSync } from 'node:child_process';
import { mkdirSync, cpSync, writeFileSync, readFileSync, rmSync } from 'node:fs';
import { fileURLToPath } from 'node:url';
import { resolve, relative } from 'node:path';
const arch = process.argv[2] || 'amd64';
if (!['amd64','arm64'].includes(arch)) throw Error('Choose amd64 or arm64');
const root = new URL('../', import.meta.url);
process.chdir(fileURLToPath(root));
const out = `dist/sshguard-linux-${arch}`;
const webOutput = resolve(out, 'web');
if (relative(resolve('dist'), webOutput).replaceAll('\\','/') !== `sshguard-linux-${arch}/web`) throw Error('Invalid release output path');
rmSync(webOutput, {recursive:true, force:true});
mkdirSync(`${out}/web/downloads`, {recursive:true});
const version = process.env.SSHGUARD_VERSION || readFileSync('VERSION','utf8').trim();
if (!/^[a-zA-Z0-9.+_-]{1,32}$/.test(version)) throw Error('Invalid version');
execFileSync('go', ['build','-trimpath','-ldflags',`-s -w -X sshguard/internal/api.Version=${version}`,'-o',`${out}/sshguard-api`,'./cmd/api'], {stdio:'inherit',env:{...process.env,CGO_ENABLED:'0',GOOS:'linux',GOARCH:arch}});
execFileSync(process.execPath,['scripts/build-agents.mjs',`${out}/web/downloads`],{stdio:'inherit'});
// npm.cmd needs cmd.exe on Windows; arguments here are fixed literals.
const npm = (args) => process.platform === 'win32' ? execFileSync('cmd.exe',['/d','/s','/c',`npm.cmd ${args.join(' ')}`],{cwd:'web',stdio:'inherit'}) : execFileSync('npm',args,{cwd:'web',stdio:'inherit'});
if (!process.argv.includes('--reuse-web')) { npm(['ci','--no-audit','--no-fund']); npm(['run','build']); }
cpSync('web/dist',`${out}/web`,{recursive:true});
for (const name of ['install-agent.sh','install-agent.ps1','uninstall-agent.sh','uninstall-agent.ps1']) cpSync(`scripts/${name}`,`${out}/web/downloads/${name}`);
cpSync('deploy/native',`${out}/deploy`,{recursive:true});
cpSync('README.md',`${out}/README.md`);
cpSync('docs',`${out}/docs`,{recursive:true});
cpSync('VERSION',`${out}/VERSION`);
console.log(`Native Linux release ready: ${out}`);
