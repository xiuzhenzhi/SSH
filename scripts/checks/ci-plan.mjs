import { execFileSync } from 'node:child_process';
import { appendFileSync } from 'node:fs';
import { pathToFileURL } from 'node:url';

export function planCI(files, full = false) {
  if (full || files === null) return { go: true, web: true };
  const docs = p => /\.md$/i.test(p) || p.startsWith('docs/') || ['LICENSE', 'package.json', 'package-lock.json', '.github/workflows/docs.yml', 'scripts/build/build-docs.mjs'].includes(p);
  const shared = files.some(p => !docs(p) && !p.startsWith('web/'));
  return { go: shared, web: shared || files.some(p => p.startsWith('web/')) };
}

if (process.argv[1] && import.meta.url === pathToFileURL(process.argv[1]).href) {
  const base = process.env.PR_BASE_SHA || process.env.BEFORE_SHA || '';
  const head = process.env.SOURCE_SHA || process.env.GITHUB_SHA || '';
  let files = null;
  if (/^[a-f0-9]{40}$/.test(base) && !/^0+$/.test(base) && /^[a-f0-9]{40}$/.test(head)) {
    try { files = execFileSync('git', ['diff', '--name-only', '-z', base, head], {encoding:'utf8'}).split('\0').filter(Boolean); }
    catch { /* A missing base must expand checks, never silently skip them. */ }
  }
  const plan = planCI(files, process.env.FULL_VALIDATION === 'true');
  appendFileSync(process.env.GITHUB_OUTPUT, `go=${plan.go}\nweb=${plan.web}\n`);
  console.log(JSON.stringify({files: files?.length ?? 'all', ...plan}));
}
