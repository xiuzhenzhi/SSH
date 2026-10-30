import {execFileSync} from 'node:child_process';
import {createHash} from 'node:crypto';
import {mkdirSync,mkdtempSync,readFileSync,rmSync,cpSync} from 'node:fs';
import {resolve,join,relative,sep} from 'node:path';
import {fileURLToPath,pathToFileURL} from 'node:url';

export function verifyArchive(bytes,manifest,name) {
 const entries=manifest.split(/\r?\n/).map(line=>/^([a-f0-9]{64})\s+\*?(.+)$/.exec(line)).filter(Boolean).filter(row=>row[2]===name);
 if(entries.length!==1 || createHash('sha256').update(bytes).digest('hex')!==entries[0][1])throw Error('Published frontend archive checksum mismatch');
}

export function reuseWeb(root,repository,run=execFileSync) {
 if(!/^[A-Za-z0-9][A-Za-z0-9-]*\/[A-Za-z0-9_.-]+$/.test(repository||''))throw Error('Invalid repository');
 const command=(name,args)=>run(name,args,{cwd:root,encoding:'utf8',stdio:['ignore','pipe','pipe']});
 const releases=JSON.parse(command('gh',['api',`repos/${repository}/releases?per_page=100`]));
 let tag;
 for(const release of releases){
  if(release.draft || !/^v\d+\.\d+\.\d+(?:-[\w.-]+)?$/.test(release.tag_name))continue;
  if(!release.assets?.some(a=>a.name==='sshguard-linux-amd64.tar.gz'))continue;
  try {
   command('git',['merge-base','--is-ancestor',release.tag_name,'HEAD']);
   command('git',['diff','--exit-code',release.tag_name,'HEAD','--','web']);
   tag=release.tag_name;break;
  }catch{ /* A changed tree or missing history must never reuse stale assets. */ }
 }
 if(!tag)throw Error('No verified release has identical frontend inputs; run full validation to build the frontend');
 const dist=resolve(root,'dist');mkdirSync(dist,{recursive:true});
 const temporary=mkdtempSync(join(dist,'.reuse-web-'));
 try {
  command('gh',['release','download',tag,'--repo',repository,'--pattern','sshguard-linux-amd64.tar.gz','--pattern','SHA256SUMS','--dir',temporary]);
  const archive=join(temporary,'sshguard-linux-amd64.tar.gz');
  verifyArchive(readFileSync(archive),readFileSync(join(temporary,'SHA256SUMS'),'utf8'),'sshguard-linux-amd64.tar.gz');
  command('tar',['-xzf',archive,'-C',temporary,'sshguard-linux-amd64/web']);
  const source=join(temporary,'sshguard-linux-amd64','web'),target=resolve(root,'web/dist');
  if(relative(resolve(root,'web'),target)!=='dist')throw Error('Invalid frontend output path');
  rmSync(target,{recursive:true,force:true});
  cpSync(source,target,{recursive:true,filter:path=>relative(source,path).split(sep)[0]!=='downloads'});
  console.log(`Reused verified frontend from ${tag}; skipped frontend CI and build`);
 } finally {rmSync(temporary,{recursive:true,force:true});}
}
if(process.argv[1]&&import.meta.url===pathToFileURL(process.argv[1]).href){
 reuseWeb(fileURLToPath(new URL('../../',import.meta.url)),process.env.GITHUB_REPOSITORY||'xiuzhenzhi/SSHGuard');
}
