import { execFileSync } from 'node:child_process';
import { appendFileSync } from 'node:fs';
import { pathToFileURL } from 'node:url';

const versionPatterns = {
 VERSION: /^[0-9]+\.[0-9]+\.[0-9]+(?:-[\w.-]+)?\r?\n?$/,
 'internal/api/routes.go': /^var Version = "[0-9]+\.[0-9]+\.[0-9]+(?:-[\w.-]+)?"$/m,
 'sshguard.sh': /^version=\$\{SSHGUARD_VERSION:-v[0-9]+\.[0-9]+\.[0-9]+(?:-[\w.-]+)?\}$/m,
};

// Ignore only a changed release value, never another edit in the same file.
export function isVersionOnlyChange(path, before, after) {
 const pattern=versionPatterns[path];
 if (!pattern || before===after || !pattern.test(before) || !pattern.test(after)) return false;
 return before.replace(pattern,'<release-version>')===after.replace(pattern,'<release-version>');
}

export function planCI(files, full = false, versionOnly = []) {
 const plan = {go:false, agent:false, database:false, web:false, deploy:false};
 if (full || files === null) return Object.fromEntries(Object.keys(plan).map(k=>[k,true]));
 for (const p of files) {
  if (versionOnly.includes(p) && Object.hasOwn(versionPatterns,p)) continue;
  // The mandatory scope job runs these tests on every push and release.
  if (['scripts/checks/ci-plan.mjs','scripts/tests/ci-plan.test.mjs'].includes(p)) continue;
  if (/\.md$/i.test(p) || p.startsWith('docs/') || ['LICENSE','package.json','package-lock.json','.github/workflows/docs.yml','scripts/build/build-docs.mjs'].includes(p)) continue;
  if (p.startsWith('web/')) { plan.web=true; continue; }
  if (/^(cmd\/agent|internal\/agent)\//.test(p)) {plan.agent=true;continue;}
  if (/^internal\/(ipaddress|originurl)\//.test(p)) {plan.go=true;plan.agent=true;continue;}
  if (/^(internal\/(api|storage|server)|cmd\/api)\//.test(p)) {plan.go=true;plan.database=true;continue;}
  if (/^internal\/(geoip|notify)\//.test(p)) {plan.go=true;continue;}
  if (/^scripts\/(install|uninstall)-agent\./.test(p) || p==='AGENT_VERSION' || p==='scripts/build/build-agents.mjs' || p==='scripts/release/agent-version.mjs') {plan.agent=true;plan.deploy=true;continue;}
  if (p==='VERSION') {plan.go=true;continue;}
  if (p.startsWith('deploy/') || p==='compose.yaml' || p==='sshguard.sh') {plan.deploy=true;continue;}
  // Unknown files, dependency or workflow changes conservatively check all components.
  return Object.fromEntries(Object.keys(plan).map(k=>[k,true]));
 }
 return plan;
}

if (process.argv[1] && import.meta.url === pathToFileURL(process.argv[1]).href) {
  let base = process.env.PR_BASE_SHA || process.env.BEFORE_SHA || '';
  const head = process.env.SOURCE_SHA || process.env.GITHUB_SHA || '';
  if (process.env.SELECTIVE_RELEASE === 'true') {
    try { base = execFileSync('git',['rev-list','-n','1',execFileSync('git',['describe','--tags','--abbrev=0',head+'^'],{encoding:'utf8'}).trim()],{encoding:'utf8'}).trim(); } catch { base=''; }
  }
  let files = null;
  if (/^[a-f0-9]{40}$/.test(base) && !/^0+$/.test(base) && /^[a-f0-9]{40}$/.test(head)) {
    try { files = execFileSync('git', ['diff', '--name-only', '-z', base, head], {encoding:'utf8'}).split('\0').filter(Boolean); }
    catch { /* A missing base must expand checks, never silently skip them. */ }
  }
  const versionOnly=[];
  for (const path of files || []) {
    if (!Object.hasOwn(versionPatterns,path)) continue;
    try {
      const before=execFileSync('git',['show',`${base}:${path}`],{encoding:'utf8',stdio:['ignore','pipe','ignore']});
      const after=execFileSync('git',['show',`${head}:${path}`],{encoding:'utf8',stdio:['ignore','pipe','ignore']});
      if (isVersionOnlyChange(path,before,after)) versionOnly.push(path);
    } catch { /* Missing or unreadable content keeps its normal checks. */ }
  }
  const plan = planCI(files, process.env.FULL_VALIDATION === 'true' && process.env.SELECTIVE_RELEASE !== 'true',versionOnly);
  appendFileSync(process.env.GITHUB_OUTPUT, Object.entries(plan).map(([k,v])=>`${k}=${v}\n`).join(''));
  const packages = plan.go && plan.agent ? './...' : plan.agent ? './cmd/agent ./internal/agent ./internal/ipaddress ./internal/originurl' : './cmd/api ./internal/api ./internal/server ./internal/storage ./internal/geoip ./internal/notify ./internal/ipaddress ./internal/originurl';
  appendFileSync(process.env.GITHUB_OUTPUT, `packages=${packages}\n`);
  console.log(JSON.stringify({files: files?.length ?? 'all', versionOnly, ...plan}));
}
