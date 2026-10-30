import {execFileSync} from 'node:child_process';
import {createHash} from 'node:crypto';
import {readFileSync,writeFileSync,mkdirSync,mkdtempSync,rmSync} from 'node:fs';
import {tmpdir} from 'node:os';
import {join,resolve} from 'node:path';
import {pathToFileURL} from 'node:url';

export function validateTag(tag){
 if(!/^v\d+\.\d+\.\d+(?:-[A-Za-z0-9.-]+)?$/.test(tag||''))throw Error('Invalid release tag');
 return tag;
}
export function verifyBundle(bytes,manifest,name){
 const entries=manifest.split(/\r?\n/).map(line=>/^([a-f0-9]{64})\s+\*?(.+)$/.exec(line)).filter(m=>m?.[2]===name);
 if(entries.length!==1||entries[0][1]!==createHash('sha256').update(bytes).digest('hex'))throw Error('Release checksum mismatch: '+name);
}
export function verifyPlatforms(manifest){
 const platforms=new Set((manifest.manifests||[]).filter(m=>m.platform?.os==='linux').map(m=>m.platform.architecture));
 if(!platforms.has('amd64')||!platforms.has('arm64'))throw Error('Both Linux architectures must be published');
}
export function isLatest(tag,release){return !release.draft&&!release.prerelease&&release.tag_name===tag;}
export function healthyRelease(body,tag){return body?.data?.status==='ok'&&body.data.version===tag.slice(1);}

export async function publishContainers(){
 const tag=validateTag(process.env.RELEASE_TAG);
 const repository=process.env.GITHUB_REPOSITORY;
 if(!/^[\w.-]+\/[\w.-]+$/.test(repository||''))throw Error('Invalid repository');
 if(!process.env.GH_TOKEN)throw Error('Missing registry authentication');
 const run=(tool,args,options={})=>execFileSync(tool,args,{encoding:'utf8',stdio:['pipe','pipe','inherit'],...options});
 const release=JSON.parse(run('gh',['api',`repos/${repository}/releases/tags/${tag}`]));
 if(release.draft||release.tag_name!==tag)throw Error('A published Release is required');
 const revision=run('git',['rev-parse',`${tag}^{commit}`]).trim();
 if(!/^[a-f0-9]{40}$/.test(revision))throw Error('Invalid release revision');
 const temporary=mkdtempSync(join(tmpdir(),'sshguard-containers-'));
 try{
  const context=join(temporary,'context');mkdirSync(context);
  run('gh',['release','download',tag,'--repo',repository,'--dir',temporary,'--pattern','SHA256SUMS','--pattern','sshguard-linux-*.tar.gz']);
  const manifest=readFileSync(join(temporary,'SHA256SUMS'),'utf8');
  for(const arch of ['amd64','arm64']){
   const name=`sshguard-linux-${arch}.tar.gz`,archive=join(temporary,name);
   verifyBundle(readFileSync(archive),manifest,name);
   run('tar',['-xzf',archive,'-C',context,`sshguard-linux-${arch}/sshguard-api`,`sshguard-linux-${arch}/VERSION`,...(arch==='amd64'?[`sshguard-linux-${arch}/web`]:[])]);
   if(readFileSync(join(context,`sshguard-linux-${arch}/VERSION`),'utf8').trim()!==tag.slice(1))throw Error('Bundle version does not match tag');
  }
  writeFileSync(join(context,'nginx.conf'),run('git',['show',`${revision}:deploy/nginx.conf`]));
  run('docker',['login','ghcr.io','--username',repository.split('/')[0],'--password-stdin'],{input:process.env.GH_TOKEN+'\n'});
  const images=['api','web'].map(component=>({component,image:`ghcr.io/${repository.toLowerCase()}-${component}`}));
  for(const {component,image} of images){
   let existing;
   try{existing=JSON.parse(run('docker',['buildx','imagetools','inspect',`${image}:${tag}`,'--raw'],{stdio:['ignore','pipe','ignore']}));}catch{}
   if(existing){verifyPlatforms(existing);console.log('Keeping immutable image '+image+':'+tag);continue;}
   run('docker',['buildx','build','--platform','linux/amd64,linux/arm64','--file',resolve(`deploy/containers/Dockerfile.${component}`),
    '--label',`org.opencontainers.image.source=https://github.com/${repository}`,
    '--label',`org.opencontainers.image.revision=${revision}`,'--label',`org.opencontainers.image.version=${tag}`,
    '--tag',`${image}:${tag}`,'--push',context],{stdio:'inherit'});
   verifyPlatforms(JSON.parse(run('docker',['buildx','imagetools','inspect',`${image}:${tag}`,'--raw'])));
  }
  // Test exactly the published amd64 images before advancing the latest aliases.
  const api=images[0].image,web=images[1].image;
  const container=run('docker',['run','-d','--platform','linux/amd64','--read-only','--cap-drop=ALL',
   '--mount','type=volume,target=/var/lib/sshguard','-p','127.0.0.1::8080',
   '-e','DB_DRIVER=sqlite','-e','DATABASE_URL=file:/var/lib/sshguard/smoke.db','-e','PUBLIC_URL=http://localhost:8080',
   '-e','ADMIN_USERNAME=admin','-e','ADMIN_PASSWORD=Container-smoke-only-123456','-e',`ENCRYPTION_KEY=${'ab'.repeat(32)}`,`${api}:${tag}`]).trim();
  try{
   const port=run('docker',['port',container,'8080/tcp']).trim().split(':').at(-1);
   if(!/^\d+$/.test(port))throw Error('Cannot locate smoke test port');
   let ready=false;
   for(let attempt=0;attempt<30;attempt++){
    try{
     const response=await fetch(`http://127.0.0.1:${port}/api/v1/health`,{signal:AbortSignal.timeout(2000)});
     if(response.ok&&healthyRelease(await response.json(),tag)){ready=true;break;}
    }catch{}
    await new Promise(resolve=>setTimeout(resolve,1000));
   }
   if(!ready){run('docker',['logs',container],{stdio:'inherit'});throw Error('Published API health/version check failed');}
  }finally{run('docker',['rm','-fv',container]);}
  run('docker',['run','--rm','--platform','linux/amd64','--entrypoint','sh',`${web}:${tag}`,'-ec','test -s /usr/share/nginx/html/index.html; test -s /usr/share/nginx/html/downloads/sshguard-agent-linux-amd64; test -s /usr/share/nginx/html/downloads/sshguard-agent-linux-arm64']);
  const latest=JSON.parse(run('gh',['api',`repos/${repository}/releases/latest`]));
  if(isLatest(tag,latest)){
   for(const {image} of images)run('docker',['buildx','imagetools','create','--tag',`${image}:latest`,`${image}:${tag}`],{stdio:'inherit'});
  }
  console.log('Published '+images.map(({image})=>image+':'+tag).join(', '));
 }finally{rmSync(temporary,{recursive:true,force:true});}
}
if(process.argv[1]&&import.meta.url===pathToFileURL(process.argv[1]).href)await publishContainers();
