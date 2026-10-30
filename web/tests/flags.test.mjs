import test from 'node:test';
import assert from 'node:assert/strict';
import {readdir} from 'node:fs/promises';
import {fileURLToPath} from 'node:url';
import {build} from 'vite';
import vue from '@vitejs/plugin-vue';

test('production flag component includes every bundled country SVG', async () => {
  const root = fileURLToPath(new URL('../', import.meta.url));
  const result = await build({
    root, configFile:false, logLevel:'silent', plugins:[vue()],
    build:{write:false, rollupOptions:{input:fileURLToPath(new URL('../src/components/CountryFlag.vue',import.meta.url)),external:['vue']}},
  });
  const output = (Array.isArray(result) ? result : [result]).flatMap(bundle => bundle.output);
  const emitted = output.filter(item => item.type === 'asset' && item.fileName.endsWith('.svg'));
  const expected = (await readdir(new URL('../node_modules/flag-icons/flags/4x3/',import.meta.url))).filter(name=>name.endsWith('.svg'));
  assert.ok(expected.length > 200, 'country asset dependency is incomplete');
  for (const name of expected) {
    assert.ok(emitted.some(item => item.names.includes(name)), `missing country flag: ${name}`);
  }
});
