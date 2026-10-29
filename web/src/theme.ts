import { ref, computed, nextTick } from 'vue';

const key='sshguard-theme';
const system=window.matchMedia('(prefers-color-scheme: dark)');
function preference():string|null {try{return localStorage.getItem(key)}catch{return null}}
const saved=preference();
export const theme=ref<'light'|'dark'>(saved==='dark'||saved==='light'?saved:system.matches?'dark':'light');
export const isDark=computed(()=>theme.value==='dark');
function apply(){document.documentElement.classList.toggle('dark',isDark.value);document.documentElement.dataset.theme=theme.value;document.documentElement.style.colorScheme=theme.value;}
apply();
type ThemeTransition={ready:Promise<void>,finished:Promise<void>,skipTransition:()=>void};
let transition:ThemeTransition|undefined;
export async function toggleTheme(event?:MouseEvent){
 if(transition)return;
 const next=isDark.value?'light':'dark';
 const change=async()=>{theme.value=next;try{localStorage.setItem(key,next)}catch{}apply();await nextTick();};
 const reduced=window.matchMedia('(prefers-reduced-motion: reduce)').matches;
 const start=(document as Document&{startViewTransition?:(callback:()=>Promise<void>)=>ThemeTransition}).startViewTransition;
 if(!start||reduced){await change();return;}
 const root=document.documentElement;
 const button=event?.currentTarget instanceof Element?event.currentTarget.getBoundingClientRect():null;
 const x=button?button.left+button.width/2:innerWidth/2,y=button?button.top+button.height/2:innerHeight/2;
 // Include the furthest viewport corner; no unpainted strip at the last frame.
 const radius=Math.ceil(Math.hypot(Math.max(x,innerWidth-x),Math.max(y,innerHeight-y)))+4;
 root.style.setProperty('--theme-origin-x',`${x}px`);root.style.setProperty('--theme-origin-y',`${y}px`);root.style.setProperty('--theme-radius',`${radius}px`);
 root.classList.add('theme-transition');
 let current:ThemeTransition|undefined;
 try{
  current=start.call(document,change);transition=current;
  await current.ready;
 }catch{current?.skipTransition();await change();}
 finally{await current?.finished.catch(()=>{});transition=undefined;root.classList.remove('theme-transition');for(const name of ['--theme-origin-x','--theme-origin-y','--theme-radius'])root.style.removeProperty(name);}
}
export function observeTheme(){
 const update=()=>{transition?.skipTransition();const choice=preference();theme.value=choice==='dark'||choice==='light'?choice:system.matches?'dark':'light';apply();};
 const storage=(event:StorageEvent)=>{if(event.key===key||event.key===null)update();};
 system.addEventListener('change',update);window.addEventListener('storage',storage);
 return ()=>{system.removeEventListener('change',update);window.removeEventListener('storage',storage);};
}
