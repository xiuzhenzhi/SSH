import { execFileSync } from 'node:child_process';
import { fileURLToPath } from 'node:url';

process.chdir(fileURLToPath(new URL('../', import.meta.url)));
// VitePress keeps SSR intermediates whenever DEBUG is set, even to an unrelated
// application's debug namespace. Scope the build so temporary files are removed.
execFileSync(process.execPath, ['node_modules/vitepress/bin/vitepress.js', 'build', 'docs'], {
  stdio: 'inherit', env: { ...process.env, DEBUG: '' },
});
