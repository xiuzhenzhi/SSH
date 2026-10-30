import { readFileSync } from 'node:fs';
import { createHash } from 'node:crypto';
import { resolve } from 'node:path';
const dir = resolve(process.argv[2] || 'dist/sshguard-linux-amd64/web/downloads');
for (const os of ['linux','windows']) for (const arch of ['amd64','arm64']) {
  const name = `sshguard-agent-${os}-${arch}${os === 'windows' ? '.exe' : ''}`;
  const data = readFileSync(resolve(dir,name));
  const expected = readFileSync(resolve(dir,`${name}.sha256`),'utf8').trim().split(/\s+/)[0];
  if (createHash('sha256').update(data).digest('hex') !== expected) throw Error(`${name}: checksum mismatch`);
  if (os === 'linux') {
    if (data.subarray(0,4).toString('hex') !== '7f454c46' || data[4] !== 2 || data[5] !== 1) throw Error(`${name}: not ELF64 little endian`);
    if (data.readUInt16LE(18) !== (arch === 'amd64' ? 62 : 183)) throw Error(`${name}: wrong ELF architecture`);
    const offset = Number(data.readBigUInt64LE(32)), size = data.readUInt16LE(54), count = data.readUInt16LE(56);
    for(let i=0;i<count;i++) if(data.readUInt32LE(offset+i*size) === 3) throw Error(`${name}: dynamic interpreter dependency; not suitable for glibc/musl portability`);
  } else {
    const pe = data.readUInt32LE(60);
    if (data.subarray(0,2).toString() !== 'MZ' || data.readUInt32LE(pe) !== 0x4550 || data.readUInt16LE(pe+4) !== (arch === 'amd64' ? 0x8664 : 0xaa64)) throw Error(`${name}: wrong PE architecture`);
  }
  console.log(`${name}: checksum and ${os === 'linux' ? 'static ELF' : 'Windows PE'} architecture OK`);
}
