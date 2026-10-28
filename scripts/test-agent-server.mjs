// Isolated CI HTTPS fixture. Never use this mock enrollment server in production.
import https from 'node:https';
import {readFileSync} from 'node:fs';
import {join} from 'node:path';
if(process.env.GITHUB_ACTIONS!=='true') throw Error('Disposable CI only');
const events=new Set();
const server=https.createServer({pfx:readFileSync(process.env.SSHGUARD_TEST_PFX),passphrase:process.env.SSHGUARD_TEST_PFX_PASSWORD},(req,res)=>{
 if(req.method==='GET' && /^\/downloads\/sshguard-agent-windows-amd64\.exe(?:\.sha256)?$/.test(req.url)){
   try {res.end(readFileSync(join(process.env.SSHGUARD_TEST_DOWNLOAD_ROOT,req.url.split('/').pop())));}catch{res.writeHead(404).end();}return;
 }
 if(req.url==='/ready'){res.end('ready');return;}
 if(req.url==='/collected'){res.setHeader('Content-Type','application/json');res.end(JSON.stringify({count:events.size}));return;}
 let body='';req.on('data',chunk=>{body+=chunk;if(body.length>4096)req.destroy();});
 req.on('end',()=>{
   try{
     const input=JSON.parse(body);
     res.setHeader('Content-Type','application/json');
     if(req.url==='/api/v1/agent/enroll' && input.token==='a'.repeat(64)){res.end(JSON.stringify({data:{server_id:'windows-ci',token:'b'.repeat(64),log_file:process.env.SSHGUARD_TEST_LOG}}));return;}
     if(req.url==='/api/v1/agent/heartbeat' && req.headers.authorization==='Bearer '+'b'.repeat(64)){res.end(JSON.stringify({data:{received:true}}));return;}
     if(req.url==='/api/v1/agent/events/batch' && req.headers.authorization==='Bearer '+'b'.repeat(64)){
       const previous=events.size;for(const event of input.events)events.add(event.event_id);
       const inserted=events.size-previous;res.end(JSON.stringify({data:{accepted:input.events.length,inserted,duplicates:input.events.length-inserted}}));return;
     }
   }catch{}
   res.writeHead(401).end('{}');
 });
});
server.listen(9443,'::');
