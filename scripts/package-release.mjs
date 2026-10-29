// Run after build-release for both API architectures. Produces assets for CI/Release.
import { execFileSync } from 'node:child_process';
import { mkdirSync, cpSync, readdirSync, readFileSync, writeFileSync } from 'node:fs';
import { createHash } from 'node:crypto';
import { fileURLToPath } from 'node:url';
process.chdir(fileURLToPath(new URL('../', import.meta.url)));
mkdirSync('dist/release',{recursive:true});
for (const arch of ['amd64','arm64']) execFileSync('tar',['-czf',`dist/release/sshguard-linux-${arch}.tar.gz`,'-C','dist',`sshguard-linux-${arch}`],{stdio:'inherit'});
const downloads='dist/sshguard-linux-amd64/web/downloads';
for (const name of readdirSync(downloads)) cpSync(`${downloads}/${name}`,`dist/release/${name}`);
cpSync('VERSION','dist/release/VERSION');
cpSync('scripts/deploy.sh','dist/release/deploy.sh');
cpSync('scripts/uninstall.sh','dist/release/uninstall.sh');
cpSync('docs/RELEASE-NOTES.md','dist/release/RELEASE-NOTES.md');
const names=readdirSync('dist/release').filter(n=>n!=='SHA256SUMS').sort();
writeFileSync('dist/release/SHA256SUMS',names.map(name=>`${createHash('sha256').update(readFileSync(`dist/release/${name}`)).digest('hex')}  ${name}`).join('\n')+'\n');
console.log('Release assets prepared at dist/release');
