import test from 'node:test';
import assert from 'node:assert/strict';
import {createServer} from 'node:http';
import {mkdtemp,writeFile,rm} from 'node:fs/promises';
import {tmpdir} from 'node:os';
import {join} from 'node:path';
import {fileURLToPath} from 'node:url';
import {execFile} from 'node:child_process';
import {promisify} from 'node:util';

test('native update verification accepts an unchanged Agent with a newer controller',async()=>{
 const root=await mkdtemp(join(tmpdir(),'sshguard-version-ci-'));
 const server=createServer((req,res)=>{res.setHeader('Content-Type','application/json');res.end(JSON.stringify({data:req.url.endsWith('/servers/fixture')?{update_status:'success',agent_version:'0.3.16'}:{}}));});
 await new Promise(resolve=>server.listen(0,'127.0.0.1',resolve));
 try{
  await writeFile(join(root,'VERSION'),'0.3.18\n');await writeFile(join(root,'AGENT_VERSION'),'0.3.16\n');await writeFile(join(root,'server'),'fixture');
  const result=await promisify(execFile)(process.execPath,[fileURLToPath(new URL('./test-native-api.mjs',import.meta.url)),'verify-update'],{
   cwd:root,timeout:5000,env:{...process.env,GITHUB_ACTIONS:'true',SSHGUARD_TEST_STATE:root,SSHGUARD_TEST_PASSWORD:'fixture',SSHGUARD_TEST_ORIGIN:'http://127.0.0.1:'+server.address().port},
  });
  assert.equal(result.stderr,'');
 }finally{await new Promise(resolve=>server.close(resolve));await rm(root,{recursive:true,force:true});}
});
