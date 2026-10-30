<script setup lang="ts">
import { onMounted, onUnmounted, ref, watch } from 'vue';
import { isDark } from '../lib/theme';
import { loadCaptchaScript } from '../lib/captchaScripts';
type Answer=Record<string,string>;
const props=defineProps<{provider:string,siteKey:string}>();
const emit=defineEmits<{answer:[value:Answer]}>();
const host=ref<HTMLElement|null>(null),error=ref(''),loading=ref(false);
let generation=0,widget:any,widgetID:string|number|undefined,live=true;
const globals=window as any;
function clear(){emit('answer',{});}
function destroy(){generation++;clear();try{if(props.provider==='turnstile'&&widgetID!==undefined)globals.turnstile?.remove(widgetID);else if(props.provider==='recaptcha'&&widgetID!==undefined)globals.grecaptcha?.reset(widgetID);else widget?.destroy?.();}catch{}widget=undefined;widgetID=undefined;host.value?.replaceChildren();}
async function render(){
 destroy();const request=generation;error.value='';loading.value=true;
 try{
  await loadCaptchaScript(props.provider);if(!live||request!==generation||!host.value)return;
  const valid=(value:Answer)=>{if(live&&request===generation){error.value='';loading.value=false;emit('answer',value);}};
  const failed=()=>{if(live&&request===generation){clear();error.value='验证失败，请重试';loading.value=false;}};
  if(props.provider==='turnstile'){
   widgetID=globals.turnstile.render(host.value,{sitekey:props.siteKey,size:host.value.clientWidth<304?'compact':'normal',action:'login',theme:isDark.value?'dark':'light',callback:(token:string)=>valid({token}),'expired-callback':clear,'error-callback':failed});loading.value=false;
  }else if(props.provider==='recaptcha'){
   await new Promise<void>(resolve=>globals.grecaptcha.ready(resolve));if(!live||request!==generation||!host.value)return;
   widgetID=globals.grecaptcha.render(host.value,{sitekey:props.siteKey,size:host.value.clientWidth<304?'compact':'normal',theme:isDark.value?'dark':'light',callback:(token:string)=>valid({token}),'expired-callback':clear,'error-callback':failed});loading.value=false;
  }else{
   globals.initGeetest4({captchaId:props.siteKey,product:'float',protocol:'https://',language:'zho',timeout:10000},(value:any)=>{
    if(!live||request!==generation){value.destroy?.();return;}widget=value;value.appendTo(host.value);value.onReady(()=>{loading.value=false});value.onError(failed);value.onSuccess(()=>{const answer=value.getValidate();if(answer)valid({lot_number:answer.lot_number,captcha_output:answer.captcha_output,pass_token:answer.pass_token,gen_time:answer.gen_time});});
   });
  }
 }catch(e){if(live&&request===generation){error.value=(e as Error).message;loading.value=false;}}
}
function reset(){clear();error.value='';try{if(props.provider==='turnstile'&&widgetID!==undefined)globals.turnstile.reset(widgetID);else if(props.provider==='recaptcha'&&widgetID!==undefined)globals.grecaptcha.reset(widgetID);else widget?.reset?.();}catch{void render();}}
defineExpose({reset});
watch(isDark,()=>{if(props.provider==='turnstile')void render();});
onMounted(()=>void render());onUnmounted(()=>{live=false;destroy();});
</script>
<template><div class="captcha-widget"><div ref="host"/><small v-if="loading">正在加载人机验证…</small><div v-if="error" class="captcha-error" role="alert">{{error}} <button type="button" class="text-action" @click="render">重试</button></div></div></template>
