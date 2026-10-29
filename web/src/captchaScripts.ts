const pending=new Map<string,Promise<void>>();
const sources:Record<string,string>={turnstile:'https://challenges.cloudflare.com/turnstile/v0/api.js?render=explicit',recaptcha:'https://www.google.com/recaptcha/api.js?render=explicit&hl=zh-CN',geetest:'https://static.geetest.com/v4/gt4.js'};

// At most one loader and timeout per supported provider, shared across remounts.
export function loadCaptchaScript(provider:string):Promise<void>{
 if(pending.has(provider))return pending.get(provider)!;
 if(!sources[provider])return Promise.reject(Error('不支持的验证服务'));
 const promise=new Promise<void>((resolve,reject)=>{
  const script=document.createElement('script');let timer=0;
  const cleanup=()=>{clearTimeout(timer);script.onload=null;script.onerror=null;};
  script.id='sshguard-captcha-'+provider;script.src=sources[provider];script.async=true;
  script.onload=()=>{cleanup();resolve();};
  const failed=()=>{cleanup();script.remove();reject(Error('验证组件加载失败，请检查网络或反代策略后重试'));};
  script.onerror=failed;timer=window.setTimeout(failed,15000);document.head.appendChild(script);
 });
 pending.set(provider,promise);promise.catch(()=>pending.delete(provider));return promise;
}
