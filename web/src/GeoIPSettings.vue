<script setup lang="ts">
import { computed,onMounted,onUnmounted,reactive,ref } from 'vue';
import { ElButton,ElSelect,ElOption,ElInput,ElMessage } from 'element-plus';
import CountryFlag from './CountryFlag.vue';
const props=defineProps<{api:(path:string,method?:string,body?:any,signal?:AbortSignal)=>Promise<any>}>();
type Row=Record<string,any>;
const providers=[{value:'none',label:'无'},{value:'maxmind',label:'MaxMind'},{value:'ip-api',label:'ip-api.com'},{value:'geojs',label:'geojs.io'},{value:'ipinfo',label:'ipinfo.io'}];
const form=reactive({provider:'geojs',token:'',clear_token:false});
const saved=ref(''),configured=ref(false),ready=ref(false),updated=ref(''),loading=ref(true),busy=ref(false),error=ref(''),testIP=ref('8.8.8.8'),result=ref<Row|null>(null);
const dirty=computed(()=>form.provider!==saved.value||!!form.token||form.clear_token);
let controller:AbortController|undefined,alive=true;
function apply(value:Row){saved.value=form.provider=value.provider;configured.value=value.token_configured;ready.value=value.database_ready;updated.value=value.database_updated_at||'';form.token='';form.clear_token=false;result.value=null;}
async function load(){controller?.abort();controller=new AbortController();const signal=controller.signal;loading.value=true;error.value='';try{const value=await props.api('/settings/geoip','GET',undefined,signal);if(alive&&!signal.aborted)apply(value);}catch(e){if(alive&&!signal.aborted)error.value=(e as Error).message}finally{if(alive&&!signal.aborted)loading.value=false}}
async function save(){busy.value=true;error.value='';try{const value=await props.api('/settings/geoip','PATCH',form);if(!alive)return;apply(value);ElMessage.success('地理位置设置已保存');}catch(e){error.value=(e as Error).message}finally{busy.value=false}}
async function update(){busy.value=true;error.value='';try{const value=await props.api('/settings/geoip/update','POST',{});if(!alive)return;ready.value=value.database_ready;updated.value=value.database_updated_at;result.value=null;ElMessage.success('GeoIP 数据库已更新');}catch(e){error.value=(e as Error).message}finally{busy.value=false}}
async function test(){busy.value=true;error.value='';result.value=null;try{const value=await props.api('/settings/geoip/test','POST',{ip:testIP.value.trim()});if(alive)result.value=value;}catch(e){error.value=(e as Error).message}finally{busy.value=false}}
defineExpose({refresh:load});
onMounted(()=>void load());onUnmounted(()=>{alive=false;controller?.abort();form.token=''});
</script>
<template>
 <div class="geoip-settings" :aria-busy="loading||busy">
  <p v-if="error" class="captcha-error" role="alert">{{error}} <button v-if="!saved" class="text-action" @click="load">重试</button></p>
  <form class="panel geoip-config" @submit.prevent="save">
   <div class="geoip-row"><div><h3>地理位置数据提供商</h3><p>选择提供地理位置信息的服务商，在服务器名称前显示国家或地区旗帜</p></div><el-select v-model="form.provider" aria-label="地理位置数据提供商" :disabled="loading||busy" @change="form.token='';form.clear_token=false;result=null"><el-option v-for="provider in providers" :key="provider.value" :value="provider.value" :label="provider.label"/></el-select></div>
   <div v-if="['ipinfo','ip-api'].includes(form.provider)" class="geoip-row geoip-credentials"><div><h3>{{form.provider==='ipinfo'?'IPinfo Token（可选）':'ip-api Pro Key（可选）'}}</h3><p>{{form.provider==='ipinfo'?'填写后使用 IPinfo Lite；留空使用免 Token 接口，受服务商配额限制。':'留空使用免费 HTTP 接口（每分钟 45 次，仅限非商业用途）；填写 Pro Key 后使用 HTTPS。'}}</p></div><div class="geoip-token"><el-input v-model="form.token" type="password" show-password autocomplete="new-password" maxlength="512" :disabled="loading||busy||form.clear_token" :placeholder="configured&&form.provider===saved?'已配置，留空保留':'可选凭证'" aria-label="GeoIP 服务凭证"/><label v-if="configured&&form.provider===saved"><input v-model="form.clear_token" type="checkbox" :disabled="busy"/>清除已保存凭证</label></div></div>
   <div class="geoip-row geoip-save"><p>{{dirty?'设置已更改，保存后生效。':form.provider==='none'?'已关闭地理位置识别。':'只查询服务器公网 IP，查询失败时保留服务器列表正常显示。'}}</p><el-button type="primary" native-type="submit" :loading="busy" :disabled="loading||!saved||!dirty">保存设置</el-button></div>
  </form>
  <section class="panel geoip-row"><div><h3>更新 GeoIP 数据库</h3><p v-if="form.provider==='maxmind'">{{ready?'上次更新：'+new Date(updated).toLocaleString('zh-CN',{hour12:false}):'首次使用 MaxMind，请先下载国家或地区数据库。'}}</p><p v-else>MaxMind 使用本地数据库；在线服务由提供商维护，无需手动下载。</p><p v-if="form.provider==='maxmind'" class="geoip-attribution">数据库来源：<a href="https://github.com/Loyalsoldier/geoip" target="_blank" rel="noopener noreferrer">Loyalsoldier GeoLite2-Country</a></p></div><el-button :loading="busy" :disabled="loading||!saved||dirty||form.provider!=='maxmind'" @click="update">更新数据库</el-button></section>
  <section class="panel"><form class="geoip-row" @submit.prevent="test"><div><h3>测试 GeoIP</h3><p>测试已保存的数据提供商，支持公网 IPv4 和 IPv6</p></div><div class="geoip-test"><el-input v-model="testIP" aria-label="测试公网 IP" placeholder="输入公网 IPv4 或 IPv6" :disabled="loading||busy"/><el-button native-type="submit" :loading="busy" :disabled="loading||!saved||dirty||form.provider==='none'||!testIP.trim()||(form.provider==='maxmind'&&!ready)">测试</el-button></div></form><div v-if="result" class="geoip-result" role="status"><CountryFlag :code="result.country_code" :name="result.country_name"/><b>{{result.country_name}}</b><span>{{result.country_code}} · {{result.ip}}</span></div></section>
 </div>
</template>
<style scoped>
.geoip-settings{display:grid;gap:14px}.geoip-row{padding:20px 24px;display:flex;align-items:center;justify-content:space-between;gap:28px}.geoip-row h3{color:var(--text-strong)}.geoip-row p{margin:5px 0 0;color:var(--text-muted)}.geoip-row .el-select{width:170px;flex-shrink:0}.geoip-config>.geoip-row+.geoip-row{border-top:1px solid var(--line)}.geoip-save{padding-top:14px;padding-bottom:14px}.geoip-save p{margin:0}.geoip-token{width:280px;flex-shrink:0}.geoip-token label{display:flex;gap:6px;align-items:center;font-size:12px;margin-top:8px;color:var(--text-muted)}.geoip-test{display:flex;gap:10px;width:390px;flex-shrink:0}.geoip-result{margin:0 24px 20px;padding:14px 16px;border-radius:8px;background:var(--surface-soft);display:flex;gap:12px;align-items:center;font-size:13px;color:var(--text-strong)}.geoip-result span{color:var(--text-muted)}.geoip-attribution a{color:var(--el-color-primary)}@media(max-width:760px){.geoip-row{align-items:flex-start;flex-direction:column;gap:14px;padding:18px}.geoip-row .el-select,.geoip-token,.geoip-test{width:100%}.geoip-result{margin:0 18px 18px;flex-wrap:wrap}}
</style>
