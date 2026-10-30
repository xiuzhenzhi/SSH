import test from 'node:test';
import assert from 'node:assert/strict';
import {planCI,isVersionOnlyChange} from '../checks/ci-plan.mjs';
import {affectsAgent} from '../release/agent-version.mjs';
const none={go:false,agent:false,database:false,web:false,deploy:false};
const all=Object.fromEntries(Object.keys(none).map(k=>[k,true]));

test('release version edits do not expand a frontend-only change',()=>{
 const fixtures=[
  ['VERSION','0.3.16\n','0.3.17\n'],
  ['internal/api/routes.go','package api\nvar Version = "0.3.16"\nfunc Route() {}\n','package api\nvar Version = "0.3.17"\nfunc Route() {}\n'],
  ['sshguard.sh','version=${SSHGUARD_VERSION:-v0.3.16}\nrun\n','version=${SSHGUARD_VERSION:-v0.3.17}\nrun\n'],
 ];
 for(const [path,before,after] of fixtures){
  assert.equal(isVersionOnlyChange(path,before,after),true,path);
  assert.equal(isVersionOnlyChange(path,before,after+'extra logic\n'),false,path);
  assert.equal(isVersionOnlyChange(path,'',after),false,path);
 }
 const paths=fixtures.map(([path])=>path);
 assert.deepEqual(planCI([...paths,'web/src/App.vue'],false,paths),{...none,web:true});
 assert.deepEqual(planCI(paths,true,paths),all);
 assert.deepEqual(planCI(['internal/api/routes.go']),{...none,go:true,database:true});
 assert.equal(isVersionOnlyChange('internal/api/routes.go','var AgentVersion = "0.3.16"','var AgentVersion = "0.3.17"'),false);
});
test('component checks stay isolated',()=>{
 for(const p of ['README.md','docs/guides/AGENTS.md','docs/.vitepress/config.mts','.github/workflows/docs.yml','scripts/checks/ci-plan.mjs','scripts/tests/ci-plan.test.mjs'])assert.deepEqual(planCI([p]),none,p);
 assert.deepEqual(planCI(['web/src/App.vue']),{...none,web:true});
 assert.deepEqual(planCI(['internal/agent/run.go']),{...none,agent:true});
 assert.deepEqual(planCI(['internal/api/auth.go']),{...none,go:true,database:true});
 assert.deepEqual(planCI(['internal/storage/migrations/sqlite/012.sql']),{...none,go:true,database:true});
 assert.deepEqual(planCI(['internal/notify/send.go']),{...none,go:true});
 assert.deepEqual(planCI(['scripts/uninstall-agent.sh']),{...none,agent:true,deploy:true});
 assert.deepEqual(planCI(['deploy/nginx.conf']),{...none,deploy:true});
});
test('shared dependencies, unknown paths and explicit full validation fail safe',()=>{
 for(const p of ['go.mod','go.sum','.github/workflows/ci.yml','unknown'])assert.deepEqual(planCI([p]),all);
 assert.deepEqual(planCI(null),all);assert.deepEqual(planCI([],true),all);
});
test('only production Agent inputs advance Agent version',()=>{
 for(const p of ['internal/agent/run.go','internal/ipaddress/public.go','cmd/agent/main.go','go.mod','scripts/uninstall-agent.ps1'])assert.equal(affectsAgent(p),true,p);
 for(const p of ['internal/api/auth.go','web/src/App.vue','VERSION','docs/index.md','internal/agent/run_test.go'])assert.equal(affectsAgent(p),false,p);
});
