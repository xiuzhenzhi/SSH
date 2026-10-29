<script setup lang="ts">
import { reactive, ref } from 'vue';
const emit = defineEmits<{ complete: [] }>();
const token = ref(new URLSearchParams(location.hash.slice(1)).get('setup') || '');
if (location.hash.startsWith('#setup=')) history.replaceState(null, '', location.pathname + location.search);
const form = reactive({driver:'sqlite',host:'127.0.0.1',port:3306,database:'sshguard',db_username:'sshguard',db_password:'',tls:false,username:'admin',password:''});
const confirmation = ref(''), busy = ref(false), error = ref(''), tested = ref(false), completed = ref(false);
async function request(action: string) {
  const response = await fetch('/api/v1/setup/'+action, {method:'POST',headers:{'Content-Type':'application/json','X-Setup-Token':token.value},body:JSON.stringify(form)});
  const result = await response.json();
  if (!response.ok) throw Error(result.error || '初始化失败，请重试');
}
async function run(finish: boolean) {
  busy.value = true; error.value = '';
  try {
    if (finish && form.password !== confirmation.value) throw Error('两次密码不一致');
    await request(finish ? 'complete' : 'test');
    if (!finish) { tested.value = true; return; }
    completed.value = true; form.password = ''; confirmation.value = ''; form.db_password = ''; token.value = '';
    await waitReady();
  } catch (e) { error.value = (e as Error).message; } finally { busy.value = false; }
}
async function waitReady() {
  for (let i=0;i<30;i++) {
    await new Promise(resolve=>setTimeout(resolve,1000));
    try { const r=await fetch('/api/v1/health'); const data=await r.json(); if(data.data?.status==='ok'){emit('complete');return;} } catch {}
  }
  throw Error('配置已保存，服务仍在启动。请稍后刷新页面。');
}
</script>

<template>
  <main class="setup-screen"><section class="setup-card">
    <div class="eyebrow">SSHGUARD · 欢迎使用</div><h1>初始化你的控制台</h1>
    <p>选择数据库，创建管理员，即可开始接入服务器。</p>
    <p v-if="completed" role="status">配置已保存，正在启动控制台…</p>
    <form v-else @submit.prevent="run(true)" @input="tested=false">
      <fieldset :disabled="busy"><legend>1 · 验证安装身份</legend>
        <label>初始化口令<input v-model="token" required type="password" autocomplete="off" /></label>
        <small>安装终端的初始化链接会自动填写；也可从服务器配置的 SETUP_TOKEN 获取。</small>
      </fieldset>
      <fieldset :disabled="busy"><legend>2 · 选择数据库</legend>
        <label>数据库<select v-model="form.driver" @change="form.port=form.driver==='postgres'?5432:3306;tested=false"><option value="sqlite">SQLite（推荐，无需安装）</option><option value="mysql">MySQL</option><option value="postgres">PostgreSQL</option></select></label>
        <p v-if="form.driver==='sqlite'">数据保存在本机，适合个人与小型部署。</p>
        <template v-else>
          <p>请先创建专用空数据库与账号，SSHGuard 不会覆盖已有数据库。</p>
          <label>主机<input v-model="form.host" required /></label><label>端口<input v-model.number="form.port" type="number" min="1" max="65535" required /></label>
          <label>数据库名<input v-model="form.database" required /></label><label>数据库用户名<input v-model="form.db_username" required autocomplete="off" /></label>
          <label>数据库密码<input v-model="form.db_password" type="password" autocomplete="new-password" /></label><label class="check"><input v-model="form.tls" type="checkbox" />启用 TLS 并校验证书</label>
        </template>
        <button type="button" :disabled="busy||!token" @click="run(false)">测试连接</button><span v-if="tested" role="status"> 连接成功</span>
      </fieldset>
      <fieldset :disabled="busy"><legend>3 · 创建管理员</legend>
        <label>用户名<input v-model="form.username" minlength="3" maxlength="64" autocomplete="username" required /></label>
        <label>密码<input v-model="form.password" type="password" minlength="12" maxlength="72" autocomplete="new-password" required /></label>
        <small>密码为 12–72 字节，建议使用字母、数字和符号。</small>
        <label>确认密码<input v-model="confirmation" type="password" autocomplete="new-password" required /></label>
      </fieldset>
      <button class="primary" :disabled="busy" type="submit">{{busy?'正在处理…':'完成初始化'}}</button>
    </form>
    <p v-if="error" class="setup-error" role="alert">{{error}}</p>
  </section></main>
</template>

<style scoped>
.setup-screen{min-height:100vh;padding:40px 16px;background:#f3f6fa}.setup-card{max-width:600px;margin:auto;background:white;padding:32px;border:1px solid #dce3ec;border-radius:16px}.setup-card h1{margin:12px 0}.setup-card p,.setup-card small{color:#526174;line-height:1.7}fieldset{border:0;border-top:1px solid #e3e8ef;padding:20px 0;margin:20px 0}legend{font-weight:600;padding-right:12px}label{display:flex;flex-direction:column;gap:8px;margin:12px 0}input,select,button{font:inherit;padding:10px 12px;border:1px solid #bcc8d7;border-radius:7px;max-width:100%;box-sizing:border-box}button{cursor:pointer;background:white}button:disabled{opacity:.6;cursor:wait}.primary{width:100%;background:#235ee8;color:white;border:0}.check{flex-direction:row;align-items:center}.setup-error{color:#b42318!important}input:focus,select:focus,button:focus-visible{outline:2px solid #235ee8;outline-offset:2px}@media(max-width:480px){.setup-card{padding:20px}}
</style>
