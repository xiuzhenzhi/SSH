import test from 'node:test';
import assert from 'node:assert/strict';
import {createHash} from 'node:crypto';
import {verifyArchive,reuseWeb} from '../build/reuse-web.mjs';

test('reused release frontend requires exactly one matching checksum',()=>{
 const bytes=Buffer.from('fixture'),hash=createHash('sha256').update(bytes).digest('hex'),line=hash+'  sshguard-linux-amd64.tar.gz';
 assert.doesNotThrow(()=>verifyArchive(bytes,line,'sshguard-linux-amd64.tar.gz'));
 for(const manifest of ['',line+'\n'+line,hash+'  other.tar.gz'])assert.throws(()=>verifyArchive(bytes,manifest,'sshguard-linux-amd64.tar.gz'));
 assert.throws(()=>verifyArchive(Buffer.from('tampered'),line,'sshguard-linux-amd64.tar.gz'));
});
test('changed frontend or missing release history cannot silently reuse assets',()=>{
 const calls=[];
 const run=(command,args)=>{
  calls.push([command,...args]);
  if(command==='gh')return JSON.stringify([{tag_name:'v0.3.17',assets:[{name:'sshguard-linux-amd64.tar.gz'}]}]);
  if(args[0]==='diff')throw Error('frontend changed');
  return '';
 };
 assert.throws(()=>reuseWeb('.', 'owner/repo',run),/No verified release/);
 assert.equal(calls.some(call=>call.includes('download')),false);
 assert.throws(()=>reuseWeb('.','../bad',run),/Invalid repository/);
});

test('reuse copies verified frontend without overwriting newly selected Agent downloads',async()=>{
 const {mkdtempSync,mkdirSync,writeFileSync,readFileSync,cpSync,existsSync,rmSync}=await import('node:fs');
 const {tmpdir}=await import('node:os');const {join}=await import('node:path');const {execFileSync}=await import('node:child_process');
 const root=mkdtempSync(join(tmpdir(),'sshguard-web-reuse-'));
 try{
  const source=join(root,'fixture','sshguard-linux-amd64','web');mkdirSync(join(source,'assets'),{recursive:true});mkdirSync(join(source,'downloads'));
  writeFileSync(join(source,'index.html'),'<html>verified</html>');writeFileSync(join(source,'assets','test.svg'),'<svg/>');writeFileSync(join(source,'downloads','old-agent'),'old');
  const archive=join(root,'fixture.tar.gz');execFileSync('tar',['-czf',archive,'-C',join(root,'fixture'),'sshguard-linux-amd64']);
  const hash=createHash('sha256').update(readFileSync(archive)).digest('hex');
  const run=(command,args,options)=>{
   if(command==='git')return '';
   if(command==='gh'&&args[0]==='api')return JSON.stringify([{tag_name:'v0.3.17',assets:[{name:'sshguard-linux-amd64.tar.gz'}]}]);
   if(command==='gh'){
    const dir=args[args.indexOf('--dir')+1];cpSync(archive,join(dir,'sshguard-linux-amd64.tar.gz'));writeFileSync(join(dir,'SHA256SUMS'),hash+'  sshguard-linux-amd64.tar.gz\n');return '';
   }
   return execFileSync(command,args,options);
  };
  reuseWeb(root,'owner/repo',run);
  assert.equal(readFileSync(join(root,'web/dist/index.html'),'utf8'),'<html>verified</html>');
  assert.equal(existsSync(join(root,'web/dist/assets/test.svg')),true);
  assert.equal(existsSync(join(root,'web/dist/downloads')),false);
 }finally{rmSync(root,{recursive:true,force:true});}
});
