import { ref, computed } from 'vue';

const key='sshguard-theme';
const system=window.matchMedia('(prefers-color-scheme: dark)');
function preference():string|null {try{return localStorage.getItem(key)}catch{return null}}
const saved=preference();
export const theme=ref<'light'|'dark'>(saved==='dark'||saved==='light'?saved:system.matches?'dark':'light');
export const isDark=computed(()=>theme.value==='dark');
function apply(){document.documentElement.classList.toggle('dark',isDark.value);document.documentElement.dataset.theme=theme.value;document.documentElement.style.colorScheme=theme.value;}
apply();
export function toggleTheme(){theme.value=isDark.value?'light':'dark';try{localStorage.setItem(key,theme.value)}catch{}apply();}
export function observeTheme(){
 const update=()=>{const choice=preference();theme.value=choice==='dark'||choice==='light'?choice:system.matches?'dark':'light';apply();};
 const storage=(event:StorageEvent)=>{if(event.key===key||event.key===null)update();};
 system.addEventListener('change',update);window.addEventListener('storage',storage);
 return ()=>{system.removeEventListener('change',update);window.removeEventListener('storage',storage);};
}
