// Exercise real shell entry points, archive extraction and checksum validation.
// Only GitHub, platform detection and privilege elevation are replaced.
import test from 'node:test';
import assert from 'node:assert/strict';
import { spawnSync } from 'node:child_process';
import { mkdtempSync, mkdirSync, writeFileSync, readFileSync, existsSync, rmSync } from 'node:fs';
import { createHash } from 'node:crypto';
import { tmpdir } from 'node:os';
import { join, resolve } from 'node:path';

const shell = process.env.SSHGUARD_TEST_SHELL || (process.platform === 'win32'
  ? join(process.env.ProgramFiles, 'Git/bin/sh.exe') : 'sh');
const entry = resolve('sshguard.sh');

function fixture(t, { arch = 'amd64', version = '0.3.11', unified = false, ...overrides } = {}) {
  const dir = mkdtempSync(join(tmpdir(), 'sshguard-update-'));
  t.after(() => rmSync(dir, { recursive: true, force: true }));
  const put = (name, content) => writeFileSync(join(dir, name), content, { mode: 0o755 });
  mkdirSync(join(dir, 'bin'));
  mkdirSync(join(dir, 'tmp'));
  const bundle = `sshguard-linux-${arch}`;
  mkdirSync(join(dir, bundle));
  put(`${bundle}/VERSION`, version + '\n');
  put(`${bundle}/deploy.sh`, '#!/bin/sh\nprintf "%s\\n" "$@" > "$FIXTURE/deployed"\nexit "${DEPLOY_STATUS:-0}"\n');
  if (unified) put(`${bundle}/sshguard.sh`, '#!/bin/sh\nprintf "%s\\n" "$@" > "$FIXTURE/deployed"\n');
  const env = { ...process.env, SSHGUARD_VERSION: '', TEST_ROOT: dir, GH_AUTH: '0',
    LOOKUP_FAIL: '0', CORRUPT: '0', TARGET_TAG: 'v' + version, TEST_UID: '0',
    TEST_ARCH: arch === 'arm64' ? 'aarch64' : 'x86_64', LATEST_URL: '', DEPLOY_STATUS: '0', ...overrides };
  function run(script, args = [], scriptEntry = entry) {
    return spawnSync(shell, ['-c', `FIXTURE=$(CDPATH='' cd "$TEST_ROOT" && pwd -P)
export FIXTURE
${script}`, 'test', ...args], { encoding: 'utf8', env: { ...env, ENTRY: scriptEntry }, timeout: 15000 });
  }
  const archive = `sshguard-linux-${arch}.tar.gz`;
  const packed = run(`cd "$FIXTURE" && tar -czf "${archive}" "${bundle}"`);
  assert.equal(packed.status, 0, packed.stderr);
  put('SHA256SUMS', createHash('sha256').update(readFileSync(join(dir, archive))).digest('hex') + `  ${archive}\n`);
  put('bin/uname', '#!/bin/sh\ncase "$1" in -m) echo "$TEST_ARCH" ;; -s) echo Linux ;; *) exit 1 ;; esac\n');
  put('bin/id', '#!/bin/sh\necho "$TEST_UID"\n');
  put('bin/sudo', '#!/bin/sh\necho sudo >> "$FIXTURE/elevated"\nexec "$@"\n');
  put('bin/curl', `#!/bin/sh
set -eu
printf 'curl %s\n' "$*" >> "$FIXTURE/network"
case "$*" in *'/releases/latest'*)
  [ "$LOOKUP_FAIL" = 0 ] || exit 22
  printf '%s' "\${LATEST_URL:-https://github.com/xiuzhenzhi/SSHGuard/releases/tag/$TARGET_TAG}"
  exit 0 ;;
esac
url=''; output=''
while [ "$#" -gt 0 ]; do
  case "$1" in https://*) url=$1; shift ;; -o) output=$2; shift 2 ;; *) shift ;; esac
done
case "$url" in "https://github.com/xiuzhenzhi/SSHGuard/releases/download/$TARGET_TAG/"*) ;; *) exit 22 ;; esac
cp "$FIXTURE/\${url##*/}" "$output"
if [ "$CORRUPT" = 1 ] && [ "\${url##*/}" != SHA256SUMS ]; then printf tampered > "$output"; fi
`);
  put('bin/gh', `#!/bin/sh
set -eu
case "$1 $2" in
  'auth status') [ "$GH_AUTH" = 1 ]; exit $? ;;
  'release view')
    echo 'gh lookup' >> "$FIXTURE/network"
    [ "$LOOKUP_FAIL" = 0 ] || exit 1
    printf '%s\n' "$TARGET_TAG" ;;
  'release download')
    printf 'gh download %s\n' "$3" >> "$FIXTURE/network"
    [ "$3" = "$TARGET_TAG" ] || exit 1
    while [ "$#" -gt 0 ]; do
      case "$1" in --dir) output=$2; break ;; *) shift ;; esac
    done
    cp "$FIXTURE"/*.tar.gz "$FIXTURE/SHA256SUMS" "$output/" ;;
  *) exit 1 ;;
esac
`);
  return {
    run: (args = ['--update'], scriptEntry = entry) => run('export PATH="$FIXTURE/bin:$PATH" TMPDIR="$FIXTURE/tmp"\nexec sh "$ENTRY" "$@"', args, scriptEntry),
    read: name => existsSync(join(dir, name)) ? readFileSync(join(dir, name), 'utf8') : '',
    dir,
  };
}

test('update resolves latest once and downloads both assets from the selected tag', t => {
  const f = fixture(t);
  const result = f.run();
  assert.equal(result.status, 0, result.stderr);
  assert.equal(f.read('deployed'), '--update\n');
  const network = f.read('network');
  assert.equal((network.match(/\/releases\/latest/g) || []).length, 1);
  assert.match(network, /download\/v0\.3\.11\/sshguard-linux-amd64.tar.gz/);
  assert.match(network, /download\/v0\.3\.11\/SHA256SUMS/);
});

test('explicit versions skip discovery, and ARM64 selects its own archive', t => {
  const f = fixture(t, { arch: 'arm64', version: '0.4.0-rc.1', SSHGUARD_VERSION: 'v0.4.0-rc.1' });
  const result = f.run();
  assert.equal(result.status, 0, result.stderr);
  assert.doesNotMatch(f.read('network'), /releases\/latest/);
  assert.match(f.read('network'), /download\/v0\.4\.0-rc\.1\/sshguard-linux-arm64/);
});

test('first installation keeps the pinned release and passes installation options', t => {
  const version = readFileSync(entry, 'utf8').match(/SSHGUARD_VERSION:-v([^}]+)/)[1];
  const f = fixture(t, { version });
  const result = f.run(['--port', '8081']);
  assert.equal(result.status, 0, result.stderr);
  assert.doesNotMatch(f.read('network'), /releases\/latest/);
  assert.equal(f.read('deployed'), '--port\n8081\n');
});

test('private repository uses authenticated gh for discovery and downloads', t => {
  const f = fixture(t, { GH_AUTH: '1', TEST_UID: '1000' });
  const result = f.run();
  assert.equal(result.status, 0, result.stderr);
  assert.equal(f.read('network'), 'gh lookup\ngh download v0.3.11\n');
  assert.equal(f.read('elevated'), 'sudo\n');
  assert.equal(f.read('deployed'), '--update\n');
});

for (const options of [
  { LOOKUP_FAIL: '1' },
  { LOOKUP_FAIL: '1', GH_AUTH: '1' },
  { LATEST_URL: 'https://github.com/login' },
  { TARGET_TAG: 'v1;touch bad' },
  { TARGET_TAG: 'v0.3.11\nv0.3.12' },
  { CORRUPT: '1', TEST_UID: '1000' },
  { SSHGUARD_VERSION: 'v0.3.12' },
]) {
  test(`failed update leaves deployment untouched: ${JSON.stringify(options)}`, t => {
    const f = fixture(t, options);
    const result = f.run();
    assert.notEqual(result.status, 0, result.stdout);
    assert.equal(f.read('deployed'), '');
    assert.equal(f.read('elevated'), '');
  });
}

test('a valid archive with a mismatched manifest is rejected before deployment', t => {
  const f = fixture(t, { TARGET_TAG: 'v0.3.12', SSHGUARD_VERSION: 'v0.3.12' });
  const result = f.run();
  assert.notEqual(result.status, 0);
  assert.match(result.stderr, /VERSION does not match/);
  assert.equal(f.read('deployed'), '');
});

test('standalone update entry invokes the updater from any working directory', t => {
  const f = fixture(t);
  const result = f.run([], resolve('scripts/update.sh'));
  assert.equal(result.status, 0, result.stderr);
  assert.equal(f.read('deployed'), '--update\n');
  assert.match(f.read('network'), /releases\/latest/);
});

test('deploy update-latest delegates to the same download path from an extracted directory', t => {
  const f = fixture(t);
  const result = f.run(['--update-latest'], resolve('scripts/deploy.sh'));
  assert.equal(result.status, 0, result.stderr);
  assert.equal(f.read('deployed'), '--update\n');
  assert.match(f.read('network'), /releases\/latest/);
});

test('a freshly pulled updater runs as a single renamed file without local helpers', t => {
  const f = fixture(t);
  const downloaded = join(f.dir, 'sshguard-update.sh');
  writeFileSync(downloaded, readFileSync(entry));
  const result = f.run(['--update'], downloaded);
  assert.equal(result.status, 0, result.stderr);
  assert.equal(f.read('deployed'), '--update\n');
  assert.match(f.read('network'), /releases\/latest/);
});

test('deployment failures propagate to the caller', t => {
  const f = fixture(t, { DEPLOY_STATUS: '17' });
  assert.equal(f.run().status, 17);
});

test('new release invokes its unified entry with local deployment mode', t => {
  const f = fixture(t, {unified: true});
  const result = f.run();
  assert.equal(result.status, 0, result.stderr);
  assert.equal(f.read('deployed'), '--deploy\n--update\n');
});
