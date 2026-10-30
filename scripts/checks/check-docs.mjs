import { readdirSync, readFileSync, existsSync } from 'node:fs';
import { dirname, resolve } from 'node:path';
const files=['README.md','CHANGELOG.md','CONTRIBUTING.md','SECURITY.md',...readdirSync('docs', {recursive:true}).filter(n=>n.endsWith('.md') && !n.startsWith('.vitepress')).map(n=>`docs/${n}`)];
let broken=0;
for (const file of files) {
  const text=readFileSync(file,'utf8').replace(/```[\s\S]*?```/g,'');
  for (const match of text.matchAll(/!?\[[^\]]*\]\(([^)]+)\)/g)) {
    const target=match[1].split('#')[0];
    if (!target || /^https?:\/\//.test(target)) continue;
    if (!existsSync(resolve(dirname(file),target))) {console.error(`${file}: missing ${target}`);broken++;}
  }
}
// Homepage YAML links bypass Markdown link parsing; validate their canonical pages too.
for (const match of readFileSync('docs/index.md','utf8').matchAll(/^\s+link: (\/[^\s#]+)(?:#[^\s]+)?$/gm)) {
 const path=match[1].slice(1)+'.md';
 if(!existsSync(resolve(path))) {console.error('docs/index.md: missing '+path);broken++;}
}
if(broken)process.exit(1);
console.log(`Relative links verified in ${files.length} Markdown documents.`);
