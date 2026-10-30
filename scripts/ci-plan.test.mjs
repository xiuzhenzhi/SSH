import test from 'node:test';
import assert from 'node:assert/strict';
import { planCI } from './ci-plan.mjs';

test('documentation does not start application builds or database containers', () => {
  assert.deepEqual(planCI(['README.md', 'docs/API.md', 'docs/overview-preview.png', 'docs/.vitepress/config.mts', 'package.json', 'package-lock.json', 'scripts/build-docs.mjs', '.github/workflows/docs.yml']), {go:false,web:false});
});
test('frontend changes check the frontend without running backend databases', () => {
  assert.deepEqual(planCI(['web/package-lock.json', 'web/src/App.vue', 'README.md']), {go:false,web:true});
});
test('backend, migrations, installers, workflow and unknown changes keep all checks', () => {
  for (const file of ['cmd/agent/main.go','internal/api/auth.go','migrations/mysql/001_init.sql','scripts/deploy.sh','.github/workflows/ci.yml','Dockerfile']) {
    assert.deepEqual(planCI([file]), {go:true,web:true}, file);
  }
});
test('full validation and an unavailable comparison always run all checks', () => {
  assert.deepEqual(planCI(['README.md'], true), {go:true,web:true});
  assert.deepEqual(planCI(null), {go:true,web:true});
  assert.deepEqual(planCI([]), {go:false,web:false});
});
