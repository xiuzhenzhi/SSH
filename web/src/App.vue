<script setup lang="ts">
import { computed, onMounted, onUnmounted, reactive, ref } from 'vue';
import { ElMessage, ElMessageBox, ElIcon, ElForm, ElFormItem, ElInput, ElButton, ElAlert, ElSelect, ElOption, ElTable, ElTableColumn, ElDialog } from 'element-plus';
import 'element-plus/es/components/icon/style/css';
import 'element-plus/es/components/form/style/css';
import 'element-plus/es/components/form-item/style/css';
import 'element-plus/es/components/input/style/css';
import 'element-plus/es/components/button/style/css';
import 'element-plus/es/components/alert/style/css';
import 'element-plus/es/components/select/style/css';
import 'element-plus/es/components/option/style/css';
import 'element-plus/es/components/table/style/css';
import 'element-plus/es/components/table-column/style/css';
import 'element-plus/es/components/dialog/style/css';
import 'element-plus/es/components/message/style/css';
import 'element-plus/es/components/message-box/style/css';
import { Monitor, Grid, List, Lock, Setting, Plus, Refresh, Search, ArrowRight, SwitchButton } from '@element-plus/icons-vue';
type Row = Record<string, any>;
const user = ref<Row | null>(null), booting = ref(true), busy = ref(false), error = ref('');
const active = ref('dashboard'), rows = ref<Row[]>([]), summary = ref<Row>({}), page = ref(1), more = ref(false);
const search = ref(''), outcome = ref(''), username = ref(''), ip = ref(''), serverID = ref('');
const login = reactive({username: 'admin', password: ''});
const serverDialog = ref(false), userDialog = ref(false), commandDialog = ref(false), detail = ref<Row | null>(null);
const passwordDialog = ref(false), updating = ref(false);
const passwords = reactive({current_password: '', new_password: ''});
const commands = ref<Record<string,string>>({}), platform = ref('ubuntu'), expires = ref('');
const command = computed(() => commands.value[platform.value] || '');
const platformHint = computed(() => platform.value === 'windows' ? '在目标 Windows 的管理员 PowerShell 中执行。服务以 LocalService 身份运行。' : platform.value === 'alpine' ? '在目标 Alpine 的 root shell 中执行；使用 OpenRC，不需要 Bash 或 sudo。先安装 curl、ca-certificates。' : '在目标 Ubuntu / Debian 终端执行，需要 sudo、systemd、curl 和 ca-certificates。');
const server = reactive({id: '', name: '', group_name: '', notes: ''});
const newUser = reactive({username: '', password: '', role: 'viewer'});
const admin = computed(() => user.value?.role === 'admin');
const writer = computed(() => ['admin', 'operator'].includes(user.value?.role));
const nav = computed(() => [
  {id: 'dashboard', title: '安全总览', icon: Grid}, {id: 'servers', title: '服务器管理', icon: Monitor},
  {id: 'events', title: 'SSH 事件', icon: List}, {id: 'roadmap', title: '告警与通知', icon: Lock},
  ...(admin.value ? [{id: 'audit-logs', title: '操作审计', icon: List}, {id: 'users', title: '用户与权限', icon: Setting}] : [])
]);
const title = computed(() => nav.value.find(x => x.id === active.value)?.title || '安全总览');
const statusName: Row = {online: '在线', offline: '离线', pending: '待接入', revoked: '已撤销'};
const time = (s: any) => s ? new Date(s).toLocaleString('zh-CN', {hour12: false}) : '尚无记录';
let generation = 0;
let controller: AbortController | undefined;
let loading = false;
async function api(path: string, method = 'GET', body?: any, signal?: AbortSignal) {
  const response = await fetch('/api/v1' + path, {method, signal, credentials: 'same-origin', headers: {'Content-Type': 'application/json'}, body: body === undefined ? undefined : JSON.stringify(body)});
  const data = await response.json().catch(() => ({error: `HTTP ${response.status}`}));
  if (!response.ok) { if (response.status === 401) {user.value = null; commands.value = {}; commandDialog.value = false;} throw Error(data.error || '请求失败'); }
  return data.data;
}
async function perform(fn: () => Promise<void>) {busy.value = true; try {await fn();} catch(e) {ElMessage.error((e as Error).message);} finally {busy.value = false;}}
async function signIn() {await perform(async () => {user.value = await api('/auth/login', 'POST', login); login.password = ''; await load();});}
async function signOut() {controller?.abort(); generation++; await perform(async () => {await api('/auth/logout','POST');user.value = null;rows.value = [];commands.value = {};commandDialog.value = false;});}
async function load() {
  controller?.abort(); controller = new AbortController(); const signal = controller.signal;
  const request = ++generation; error.value = ''; loading = true;
  try {
    if (active.value === 'dashboard' || !summary.value.version) {const s = await api('/dashboard/summary','GET',undefined,signal); if (request !== generation) return; summary.value = s;}
    if (active.value === 'roadmap') {rows.value = [];return;}
    const params = new URLSearchParams({page: String(page.value), page_size: active.value === 'dashboard' ? '8' : '20'});
    if (active.value === 'servers' && search.value) params.set('q',search.value);
    if (active.value === 'events') {if (outcome.value) params.set('outcome',outcome.value);if (username.value) params.set('username',username.value);if (ip.value) params.set('source_ip',ip.value);if (serverID.value) params.set('server_id',serverID.value);}
    const result = await api(`/${active.value === 'dashboard' ? 'events' : active.value}?${params}`, 'GET', undefined, signal);
    if (request !== generation) return; rows.value = result.items; more.value = result.has_more;
  } catch (e) {if (request === generation && !signal.aborted) error.value = (e as Error).message;}
  finally {if (request === generation) loading = false;}
}
function navigate(id: string) {active.value = id;page.value = 1;rows.value = [];void load();}
function filter() {page.value = 1;void load();}
function editServer(row?: Row) {Object.assign(server, {id: row?.id || '',name: row?.name || '',group_name: row?.group_name || '',notes: row?.notes || ''});serverDialog.value = true;}
async function saveServer() {await perform(async () => {await api('/servers' + (server.id ? '/' + server.id : ''),server.id ? 'PATCH' : 'POST',server);serverDialog.value = false;ElMessage.success('服务器已保存');await load();});}
async function enrollment(row: Row) {updating.value = false; await perform(async () => {const result = await api(`/servers/${row.id}/enrollment`,'POST',{});commands.value = result.commands;platform.value = /windows/i.test(row.distribution) ? 'windows' : /alpine/i.test(row.distribution) ? 'alpine' : /debian/i.test(row.distribution) ? 'debian' : 'ubuntu';expires.value = result.expires_at;commandDialog.value = true;});}
async function updateAgent(row: Row) {updating.value = true; await perform(async () => {const result = await api('/agent/update-commands'); commands.value = result.commands; platform.value = /windows/i.test(row.distribution) ? 'windows' : /alpine/i.test(row.distribution) ? 'alpine' : 'ubuntu'; commandDialog.value = true;});}
async function changePassword() {await perform(async () => {await api('/auth/password','POST',passwords); passwords.current_password = ''; passwords.new_password = ''; passwordDialog.value = false; commands.value = {}; commandDialog.value = false; user.value = null; ElMessage.success('密码已修改，所有旧会话已失效，请重新登录');});}
async function revoke(row: Row) {try {await ElMessageBox.confirm(`撤销 ${row.name} 的 Agent 凭证后将拒绝其后续上报。`, '撤销凭证', {type:'warning',confirmButtonText:'撤销',cancelButtonText:'取消'});}catch{return;}await perform(async () => {await api(`/servers/${row.id}/revoke`,'POST',{});await load();});}
async function saveUser() {await perform(async () => {await api('/users','POST',newUser);userDialog.value = false;newUser.password = '';await load();});}
async function copyCommand() {try {await navigator.clipboard.writeText(command.value);ElMessage.success('命令已复制');}catch{ElMessage.warning('请选中下方命令手动复制');}}
function history(row: Row) {serverID.value = row.id;navigate('events');}
let timer: ReturnType<typeof setInterval>;
onMounted(async () => {try {user.value = await api('/auth/me');await load();}catch{}finally{booting.value = false;}timer = setInterval(() => {if (user.value && !document.hidden && !busy.value && !loading && ['dashboard','servers','events'].includes(active.value)) void load();},30000);});
onUnmounted(() => {clearInterval(timer); controller?.abort();});
</script>

<template>
  <div v-if="booting" class="loading">正在连接 SSHGuard…</div>
  <main v-else-if="!user" class="login-screen">
    <section class="login-intro"><div class="brand"><el-icon><Lock /></el-icon> SSHGuard <span>CONSOLE</span></div><div class="intro-copy"><div class="eyebrow">INFRASTRUCTURE SECURITY</div><h1>每一次连接，<br>都有迹可循。</h1><p>集中管理 Windows 与 Linux 服务器，<br>建立清晰、可追溯的 SSH 安全视图。</p><div class="intro-line"></div><small>PHASE 01 · 身份认证 / 资产管理 / 审计基础</small></div><footer>SSHGuard · 自托管服务器安全平台</footer></section>
    <section class="login-form"><div class="eyebrow">WELCOME BACK</div><h2>登录管理控制台</h2><p>使用管理员分配的账号继续</p><el-form label-position="top" @submit.prevent="signIn"><el-form-item label="用户名"><el-input v-model="login.username" autocomplete="username" size="large" /></el-form-item><el-form-item label="密码"><el-input v-model="login.password" type="password" show-password autocomplete="current-password" size="large" /></el-form-item><el-button type="primary" size="large" native-type="submit" :loading="busy">安全登录 <el-icon><ArrowRight /></el-icon></el-button></el-form><small>初始管理员密码保存在部署目录的 .env 文件中。</small></section>
  </main>
  <div v-else class="shell">
    <aside><div class="brand"><el-icon><Lock /></el-icon> SSHGuard</div><div class="workspace-label">SECURITY WORKSPACE</div><nav><button v-for="item in nav" :key="item.id" :class="{selected: active === item.id}" @click="navigate(item.id)"><el-icon><component :is="item.icon" /></el-icon>{{ item.title }}<span v-if="item.id === 'roadmap'" class="later">规划</span></button></nav><div class="sidebar-bottom"><span class="dot"></span> {{ summary.version || 'SSHGuard' }}<p>注册与心跳已接入<br>SSH 采集尚未启用</p></div></aside>
    <div class="main"><header><div>控制台 <span>/</span> {{ title }}</div><div class="account"><span class="avatar">{{ user.username.slice(0,1).toUpperCase() }}</span><b>{{ user.username }}</b><span>{{ user.role }}</span><el-button text @click="passwordDialog = true">修改密码</el-button><el-button text :icon="SwitchButton" aria-label="退出登录" @click="signOut" /></div></header>
    <section class="content"><div class="page-title"><div><div class="eyebrow">SSHGUARD / {{ active.toUpperCase() }}</div><h1>{{ title }}</h1><p>统一掌握服务器接入状态与安全事件。</p></div><div class="actions"><span class="refresh-label">动态列表每 30 秒刷新</span><el-button :icon="Refresh" @click="load">刷新</el-button><el-button v-if="active === 'servers' && writer" type="primary" :icon="Plus" @click="editServer()">添加服务器</el-button><el-button v-if="active === 'users' && admin" type="primary" :icon="Plus" @click="userDialog = true">创建用户</el-button></div></div>
    <el-alert v-if="error" :title="error" type="error" show-icon :closable="false" class="notice" />
    <template v-if="active === 'dashboard'">
      <div class="metrics"><article><div>纳管服务器 <el-icon><Monitor /></el-icon></div><strong>{{ summary.servers ?? '—' }}</strong><small>{{ summary.revoked || 0 }} 台已撤销凭证</small></article><article><div>在线 Agent <span class="dot"></span></div><strong>{{ summary.online ?? '—' }}<em> / {{ summary.servers || 0 }}</em></strong><small>90 秒内收到心跳</small></article><article><div>24 小时成功认证</div><strong>{{ summary.success_24h ?? '—' }}</strong><small>来自已入库 SSH 事件</small></article><article><div>24 小时认证失败</div><strong class="amber">{{ summary.failure_24h ?? '—' }}</strong><small>规则与通知将在后续阶段启用</small></article></div>
      <div class="phase-banner"><div class="phase-mark">01</div><div><b>基础平台已就绪，接入你的第一台服务器</b><p>本阶段 Agent 仅注册和上报心跳；下方事件来自上报 API，不代表已启用真实日志采集。</p></div><el-button @click="navigate('servers')">管理服务器 <el-icon><ArrowRight /></el-icon></el-button></div>
    </template>
    <div v-if="active === 'roadmap'" class="panel roadmap"><el-icon :size="42"><Lock /></el-icon><h2>告警引擎 · 下一阶段</h2><p>当前版本尚不发送安全告警，请勿将“在线心跳”视为“SSH 采集正常”。</p><div class="roadmap-grid"><article><span>02 / 采集</span><h3>真实 SSH 事件</h3><p>journal、auth.log、secure 解析，持久化队列与断线补传。</p></article><article><span>03 / 通知</span><h3>规则与可靠投递</h3><p>Root 登录、失败阈值、IP 白名单；Telegram、Webhook 与邮件。</p></article><article><span>04 / 加固</span><h3>批量部署与运维</h3><p>采集停滞检测、MFA、备份恢复验证与 70 台容量验收。</p></article></div></div>
    <div v-else class="panel"><div class="panel-heading"><h3>{{ active === 'dashboard' ? '最近 SSH 事件' : title }} <span v-if="active === 'dashboard'" class="live">持久化记录</span></h3><el-button v-if="active === 'dashboard'" link type="primary" @click="navigate('events')">查看全部 →</el-button><div v-if="active === 'servers'" class="filters"><el-input v-model="search" placeholder="搜索名称、主机名或分组" :prefix-icon="Search" clearable @keyup.enter="filter" @clear="filter"/><el-button @click="filter">查询</el-button></div></div>
    <div v-if="active === 'events'" class="event-filters"><el-input v-model="username" placeholder="用户名" clearable/><el-input v-model="ip" placeholder="来源 IP" clearable/><el-input v-model="serverID" placeholder="服务器 ID" clearable/><el-select v-model="outcome" placeholder="全部结果" clearable><el-option label="成功" value="success"/><el-option label="失败" value="failure"/></el-select><el-button type="primary" @click="filter">筛选</el-button></div>
    <el-table v-if="active === 'servers'" :data="rows" empty-text="暂无服务器，点击“添加服务器”开始接入。"><el-table-column label="服务器" min-width="185"><template #default="{row}"><div class="cell-name">{{ row.name }}</div><small>{{ row.hostname || '等待 Agent 注册' }}</small></template></el-table-column><el-table-column prop="group_name" label="分组" min-width="110"/><el-table-column label="状态" width="110"><template #default="{row}"><span :class="['status',row.status]">{{ statusName[row.status] }}</span></template></el-table-column><el-table-column prop="distribution" label="发行版" min-width="150"/><el-table-column prop="agent_version" label="Agent 版本" min-width="140"/><el-table-column label="最后心跳" min-width="180"><template #default="{row}">{{ time(row.last_seen_at) }}</template></el-table-column><el-table-column label="操作" min-width="320"><template #default="{row}"><el-button link type="primary" @click="history(row)">事件</el-button><el-button v-if="writer" link @click="editServer(row)">编辑</el-button><el-button v-if="admin" link type="primary" :disabled="busy" @click="enrollment(row)">安装命令</el-button><el-button v-if="admin && row.status !== 'pending' && row.status !== 'revoked'" link type="primary" :disabled="busy" @click="updateAgent(row)">更新命令</el-button><el-button v-if="admin && row.status !== 'revoked'" link type="danger" :disabled="busy" @click="revoke(row)">撤销</el-button></template></el-table-column></el-table>
    <el-table v-else-if="active === 'events' || active === 'dashboard'" :data="rows" empty-text="暂无 SSH 事件。本阶段可通过事件上报 API 验证入库。" @row-click="(row: Row) => detail = row"><el-table-column label="发生时间" min-width="180"><template #default="{row}">{{ time(row.occurred_at) }}</template></el-table-column><el-table-column prop="server_name" label="服务器" min-width="150"/><el-table-column prop="username" label="用户" min-width="100"/><el-table-column prop="source_ip" label="来源 IP" min-width="160"/><el-table-column prop="auth_method" label="认证方式" min-width="110"/><el-table-column label="结果" width="100"><template #default="{row}"><span :class="['status', row.outcome === 'success' ? 'online' : 'failed']">{{ row.outcome === 'success' ? '成功' : '失败' }}</span></template></el-table-column></el-table>
    <el-table v-else-if="active === 'audit-logs'" :data="rows" empty-text="暂无审计记录"><el-table-column label="时间" min-width="185"><template #default="{row}">{{ time(row.created_at) }}</template></el-table-column><el-table-column label="操作人" min-width="120"><template #default="{row}">{{ row.actor || 'Agent / 系统' }}</template></el-table-column><el-table-column prop="action" label="动作" min-width="180"/><el-table-column prop="target" label="目标 ID" min-width="300"/></el-table>
    <el-table v-else-if="active === 'users'" :data="rows" empty-text="暂无用户"><el-table-column prop="username" label="用户名"/><el-table-column prop="role" label="角色"/><el-table-column label="创建时间"><template #default="{row}">{{ time(row.created_at) }}</template></el-table-column></el-table>
    <div v-if="active !== 'dashboard'" class="pagination"><span>第 {{ page }} 页 · 每页 20 条</span><el-button :disabled="page <= 1" @click="page--;load()">上一页</el-button><el-button :disabled="!more" @click="page++;load()">下一页</el-button></div></div>
    <footer class="main-footer">SSHGuard / Phase 1 <span>数据库：{{ summary.database || '连接中' }} · 时间以浏览器本地时区显示</span></footer>
    </section></div>
  </div>
  <el-dialog v-model="serverDialog" :title="server.id ? '编辑服务器' : '添加服务器'" width="min(520px, 94vw)"><el-form label-position="top" @submit.prevent="saveServer"><el-form-item label="服务器名称"><el-input v-model="server.name" maxlength="128"/></el-form-item><el-form-item label="分组"><el-input v-model="server.group_name" maxlength="64" placeholder="例如：生产环境 / 华东"/></el-form-item><el-form-item label="备注"><el-input v-model="server.notes" type="textarea" maxlength="1024"/></el-form-item><el-button type="primary" native-type="submit" :loading="busy">保存服务器</el-button></el-form></el-dialog>
  <el-dialog v-model="userDialog" title="创建用户" width="min(480px, 94vw)"><el-form label-position="top" @submit.prevent="saveUser"><el-form-item label="用户名（3–64 字符）"><el-input v-model="newUser.username"/></el-form-item><el-form-item label="密码（12–72 字节）"><el-input v-model="newUser.password" type="password" show-password/></el-form-item><el-form-item label="角色"><el-select v-model="newUser.role"><el-option label="Viewer · 只读" value="viewer"/><el-option label="Operator · 可编辑服务器" value="operator"/><el-option label="Admin · 用户和凭证管理" value="admin"/></el-select></el-form-item><el-button type="primary" native-type="submit" :loading="busy">创建用户</el-button></el-form></el-dialog>
  <el-dialog v-model="commandDialog" :title="updating ? '更新 Agent' : '安装 Agent'" width="min(780px, 94vw)" @closed="commands = {}"><el-alert title="支持 Windows / Ubuntu / Debian / Alpine，amd64 / arm64。第一阶段仅注册与心跳，尚不采集 SSH 日志。" type="info" :closable="false"/><el-form label-position="top" style="margin-top:20px"><el-form-item label="目标操作系统"><el-select v-model="platform"><el-option label="Windows · Windows 服务" value="windows"/><el-option label="Ubuntu · systemd" value="ubuntu"/><el-option label="Debian · systemd" value="debian"/><el-option label="Alpine · OpenRC" value="alpine"/></el-select></el-form-item></el-form><p>{{ platformHint }}</p><p v-if="!updating">凭证单次有效，过期时间：{{ time(expires) }}。切换系统使用同一个凭证，仅在一台目标服务器执行；重新生成会使旧安装命令失效。</p><el-input :model-value="command" readonly type="textarea" :rows="7"/><p v-if="!updating">命令含短期凭证，请勿公开转发。重装成功会替换该服务器的旧 Agent 凭证。</p><p v-else>更新到平台提供的版本，保留原凭证，不会重新注册。下载校验失败不停止服务；替换后启动或心跳检查失败会恢复旧二进制。请先升级平台，再更新 Agent。</p><el-button type="primary" @click="copyCommand">{{ updating ? '复制更新命令' : '复制安装命令' }}</el-button></el-dialog>
  <el-dialog v-model="passwordDialog" title="修改密码" width="min(480px, 94vw)" @closed="passwords.current_password = ''; passwords.new_password = ''"><el-form label-position="top" @submit.prevent="changePassword"><el-form-item label="当前密码"><el-input v-model="passwords.current_password" type="password" autocomplete="current-password"/></el-form-item><el-form-item label="新密码（12–72 字节）"><el-input v-model="passwords.new_password" type="password" autocomplete="new-password" show-password/></el-form-item><p>修改后所有设备上的旧会话都会失效。</p><el-button type="primary" native-type="submit" :loading="busy">修改并重新登录</el-button></el-form></el-dialog>
  <el-dialog :model-value="!!detail" title="事件详情" width="min(700px, 94vw)" @close="detail = null"><pre class="detail">{{ JSON.stringify(detail,null,2) }}</pre></el-dialog>
</template>
