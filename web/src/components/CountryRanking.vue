<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref, watch } from 'vue';
import AnimatedNumber from './AnimatedNumber.vue';
import CountryFlag from './CountryFlag.vue';

type Country={country_code:string,country_name:string,count:number};
type Ranking={items:Country[],country_count:number,total:number,unlocated:number,geoip_status:string};
const props=defineProps<{api:(path:string,method?:string,body?:any,signal?:AbortSignal)=>Promise<any>,refreshKey?:object}>();
const data=ref<Ranking>({items:[],country_count:0,total:0,unlocated:0,geoip_status:'ready'});
const loading=ref(true),error=ref(''),ready=ref(false);
const names=new Intl.DisplayNames(['zh-CN'],{type:'region'});
const name=(country:Country)=>names.of(country.country_code)||country.country_name||country.country_code;
const maximum=computed(()=>Math.max(1,...data.value.items.map(country=>country.count)));
const emptyText=computed(()=>data.value.geoip_status==='disabled'?'GeoIP 已关闭':data.value.geoip_status==='unavailable'?'地理位置服务暂不可用':data.value.total?'暂未识别到来源国家':'暂无认证失败记录');
let request:AbortController|undefined,frame=0,destroyed=false;

async function load(){
 if(request)return;
 const current=new AbortController();request=current;error.value='';
 try{
  const result=await props.api('/dashboard/country-ranking','GET',undefined,current.signal);
  if(current.signal.aborted||destroyed)return;
  data.value=result;
  if(!ready.value){cancelAnimationFrame(frame);frame=requestAnimationFrame(()=>{frame=requestAnimationFrame(()=>{ready.value=true;});});}
 }catch(e){if(!current.signal.aborted&&!destroyed)error.value=(e as Error).message;}
 finally{if(request===current){request=undefined;if(!destroyed)loading.value=false;}}
}
watch(()=>props.refreshKey,()=>void load());
onMounted(()=>void load());
onUnmounted(()=>{destroyed=true;request?.abort();cancelAnimationFrame(frame);});
</script>

<template>
 <section class="country-ranking" aria-label="来源国家前五名，按近24小时认证失败次数排序" :aria-busy="loading">
  <div class="country-ranking-heading"><h4>来源国家 <span>TOP 5</span></h4><button v-if="error&&data.items.length" class="text-action" type="button" aria-label="国家统计更新失败，点击重试" @click="load()">重试更新</button><small v-else>近 24 小时</small></div>
  <div v-if="loading" class="country-ranking-skeleton" role="status" aria-label="正在加载国家统计"><div v-for="index in 5" :key="index"><i/><span/></div></div>
  <TransitionGroup v-else-if="data.items.length" name="country-rank" tag="ol" class="country-ranking-list" appear>
   <li v-for="(country,index) in data.items" :key="country.country_code" class="country-rank-row" :style="{'--rank-delay':index*55+'ms','--rank-color':['#3b78ff','#598dff','#75a1ff','#8cb2ff','#a4c2ff'][index]}" tabindex="0" :aria-label="`${name(country)}，认证失败 ${country.count.toLocaleString('zh-CN')} 次`" :title="`${name(country)}：${country.count.toLocaleString('zh-CN')} 次认证失败`">
    <span class="country-rank-number" aria-hidden="true">{{index+1}}</span><span class="country-rank-name"><CountryFlag :code="country.country_code" :name="name(country)"/><span>{{name(country)}}</span></span><span class="country-rank-track" aria-hidden="true"><i :style="{transform:`scaleX(${ready?country.count/maximum:0})`}"/></span><b><AnimatedNumber :value="country.count"/></b>
   </li>
  </TransitionGroup>
  <div v-else class="country-ranking-empty" role="status">{{error?'国家统计加载失败':emptyText}}<button v-if="error" class="text-action" @click="loading=true;load()">重试</button></div>
 </section>
</template>

<style scoped>
.country-ranking{border-top:1px solid var(--line);padding-top:12px;min-width:0}.country-ranking-heading{display:flex;align-items:center;justify-content:space-between;gap:8px;margin-bottom:10px}.country-ranking-heading h4{margin:0;color:var(--text-strong);font-size:11px;font-weight:500;white-space:nowrap}.country-ranking-heading h4 span{margin-left:4px;color:#3b78ff;font-size:9px;font-weight:600;letter-spacing:.3px}.country-ranking-heading small{font-size:9px;color:var(--text-muted);white-space:nowrap}.country-ranking-list{list-style:none;padding:0;margin:0;display:flex;flex-direction:column;gap:7px}.country-rank-row{display:grid;grid-template-columns:10px 68px minmax(20px,1fr) minmax(22px,max-content);align-items:center;gap:7px;min-height:16px;border-radius:4px;transition:background-color .18s ease;position:relative}.country-rank-row:hover,.country-rank-row:focus-visible{background:var(--surface-soft);outline:2px solid var(--line);outline-offset:3px}.country-rank-number{font-size:9px;color:var(--text-muted);font-variant-numeric:tabular-nums}.country-rank-row:first-child .country-rank-number{color:#3b78ff;font-weight:600}.country-rank-name{display:flex;align-items:center;gap:5px;min-width:0;color:var(--text-muted);font-size:10px}.country-rank-name>span{overflow:hidden;white-space:nowrap;text-overflow:ellipsis}.country-rank-name :deep(.country-flag){width:16px;height:12px;border-radius:2px}.country-rank-track{display:block;overflow:hidden;height:5px;background:var(--surface-soft);border-radius:3px}.country-rank-track i{display:block;width:100%;height:100%;border-radius:inherit;background:var(--rank-color);transform-origin:left center;transition:transform .8s cubic-bezier(.22,1,.36,1),filter .2s;will-change:transform}.country-rank-row:hover .country-rank-track i{filter:brightness(1.08)}.country-rank-row b{font-size:11px;color:var(--text-strong);font-weight:500;line-height:16px;text-align:right;font-variant-numeric:tabular-nums}.country-ranking-empty{min-height:110px;display:flex;align-items:center;justify-content:center;flex-direction:column;gap:8px;color:var(--text-muted);font-size:11px;text-align:center}.country-ranking-skeleton{display:grid;gap:7px}.country-ranking-skeleton>div{height:16px;display:flex;align-items:center;gap:12px;animation:country-skeleton-pulse 1s ease-in-out infinite alternate}.country-ranking-skeleton i,.country-ranking-skeleton span{display:block;height:6px;border-radius:3px;background:var(--line)}.country-ranking-skeleton i{width:62px}.country-ranking-skeleton span{flex:1}.country-rank-enter-active,.country-rank-leave-active{transition:opacity .35s ease,transform .4s cubic-bezier(.22,1,.36,1);transition-delay:var(--rank-delay,0ms)}.country-rank-enter-from,.country-rank-leave-to{opacity:0;transform:translateY(7px)}.country-rank-move{transition:transform .5s cubic-bezier(.22,1,.36,1)}.country-rank-leave-active{position:absolute;transition-delay:0ms}@keyframes country-skeleton-pulse{to{opacity:.4}}@media(max-width:1000px){.country-ranking{grid-column:2;grid-row:2/5}.country-rank-row{grid-template-columns:12px minmax(68px,95px) minmax(20px,1fr) minmax(24px,max-content)}}@media(max-width:500px){.country-ranking{grid-column:1/-1;grid-row:auto}.country-rank-row{grid-template-columns:12px 80px minmax(20px,1fr) minmax(24px,max-content)}}@media(prefers-reduced-motion:reduce){.country-rank-row,.country-rank-track i,.country-rank-enter-active,.country-rank-leave-active,.country-rank-move{transition:none!important}.country-ranking-skeleton>div{animation:none}}
</style>
