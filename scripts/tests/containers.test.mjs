import test from 'node:test';
import assert from 'node:assert/strict';
import {createHash} from 'node:crypto';
import {validateTag,verifyBundle,verifyPlatforms,isLatest} from '../release/publish-containers.mjs';
test('container publication accepts release tags but rejects refs and shell input',()=>{
 for(const tag of ['v0.3.19','v1.2.3-rc.1'])assert.equal(validateTag(tag),tag);
 for(const tag of ['main','../v0.3.19','v1;echo bad','v0.3.19\n'])assert.throws(()=>validateTag(tag));
});
test('container payload requires one matching release checksum',()=>{
 const bytes=Buffer.from('verified'),name='sshguard-linux-amd64.tar.gz';
 const line=createHash('sha256').update(bytes).digest('hex')+'  '+name;
 verifyBundle(bytes,line,name);
 for(const manifest of ['',line+'\n'+line])assert.throws(()=>verifyBundle(bytes,manifest,name));
 assert.throws(()=>verifyBundle(Buffer.from('changed'),line,name));
});
test('both architectures are required and older/prerelease images cannot replace latest',()=>{
 verifyPlatforms({manifests:[{platform:{os:'linux',architecture:'amd64'}},{platform:{os:'linux',architecture:'arm64'}}]});
 assert.throws(()=>verifyPlatforms({manifests:[{platform:{os:'linux',architecture:'amd64'}}]}));
 assert.equal(isLatest('v0.3.19',{tag_name:'v0.3.19',draft:false,prerelease:false}),true);
 for(const release of [{tag_name:'v0.3.20'},{tag_name:'v0.3.19',prerelease:true},{tag_name:'v0.3.19',draft:true}])assert.equal(isLatest('v0.3.19',release),false);
});
