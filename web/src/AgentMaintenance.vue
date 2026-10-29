<script setup lang="ts">
import { computed, ref } from 'vue';
import { copyText } from './clipboard';
import { ElButton, ElDialog, ElAlert, ElInput, ElSelect, ElOption, ElMessage, ElMessageBox, ElDropdown, ElDropdownMenu, ElDropdownItem } from 'element-plus';
import { MoreFilled } from '@element-plus/icons-vue';
import 'element-plus/es/components/dropdown/style/css';
import 'element-plus/es/components/dropdown-menu/style/css';
import 'element-plus/es/components/dropdown-item/style/css';
const props = defineProps<{server: Record<string, any>}>();
const emit = defineEmits<{changed: [], manual: [], revoke: []}>();
const busy = ref(false), visible = ref(false), platform = ref('ubuntu');
const commands = ref<Record<string,string>>({});
const activeTask = computed(() => ['pending','running'].includes(props.server.update_status) && new Date(props.server.update_expires_at).getTime()>Date.now());
const state = computed(() => {
  if (['pending','running'].includes(props.server.update_status) && !activeTask.value) return '更新任务已超时，请检查维护服务';
  return {idle:'更新服务就绪',pending:'等待更新',running:'正在更新',success:'更新成功',failed:'更新失败'}[props.server.update_status as string] || '需手动更新以启用远程更新';
});
const ready = computed(() => props.server.updater_seen_at && Date.now()-new Date(props.server.updater_seen_at).getTime()<180000);
async function request(path:string,method='GET') {
  const response=await fetch('/api/v1'+path,{method,credentials:'same-origin',headers:{'Content-Type':'application/json'},...(method==='POST'?{body:'{}'}:{})});
  const body=await response.json(); if(!response.ok)throw Error(body.error||'请求失败'); return body.data;
}
async function update() {
  try { await ElMessageBox.confirm(`将 ${props.server.name} 的 Agent 更新到平台提供的版本。更新期间采集会短暂重启，失败时尝试恢复原程序。`, '更新 Agent', {confirmButtonText:'开始更新',cancelButtonText:'取消',type:'warning'}); } catch {return;}
  busy.value=true;
  try {await request(`/servers/${props.server.id}/update`,'POST');ElMessage.success('更新任务已创建，目标机器将在下一次检查时处理');emit('changed');} catch(e){ElMessage.error((e as Error).message);}finally{busy.value=false;}
}
async function uninstall() {
  busy.value=true;
  try {commands.value=(await request('/agent/uninstall-commands')).commands;platform.value=/windows/i.test(props.server.distribution)?'windows':/alpine/i.test(props.server.distribution)?'alpine':'ubuntu';visible.value=true;}catch(e){ElMessage.error((e as Error).message);}finally{busy.value=false;}
}
async function copy() {try{await copyText(commands.value[platform.value]||'');ElMessage.success('卸载命令已复制');}catch{ElMessage.warning('请手动复制命令');}}
</script>
<template>
  <el-dropdown trigger="click" @command="(key:string)=>key==='update'?update():key==='manual'?emit('manual'):key==='revoke'?emit('revoke'):uninstall()">
   <el-button text :icon="MoreFilled" aria-label="更多维护操作" :disabled="busy"/>
   <template #dropdown><el-dropdown-menu>
    <el-dropdown-item disabled>{{state}}</el-dropdown-item>
    <el-dropdown-item command="update" :disabled="server.status==='revoked'||!ready||activeTask">平台更新</el-dropdown-item>
    <el-dropdown-item command="manual">更新命令</el-dropdown-item>
    <el-dropdown-item command="uninstall" divided>卸载命令</el-dropdown-item>
    <el-dropdown-item command="revoke" :disabled="server.status==='revoked'">撤销凭证</el-dropdown-item>
   </el-dropdown-menu></template>
  </el-dropdown>
  <el-dialog v-model="visible" title="卸载 Agent" width="min(780px, 94vw)" append-to-body @closed="commands = {}">
    <el-alert title="只生成命令，不会远程卸载。请在目标机器以管理员身份执行。" type="warning" :closable="false"/>
    <p>停止并移除 Agent 和更新维护服务，保留配置、采集队列与历史数据。卸载后请在平台撤销该机器的凭证；不会删除中央平台或其他机器。</p>
    <el-select v-model="platform" aria-label="目标操作系统"><el-option label="Ubuntu / Debian（sudo）" value="ubuntu"/><el-option label="Alpine（root）" value="alpine"/><el-option label="Windows（管理员 PowerShell）" value="windows"/></el-select>
    <el-input :model-value="commands[platform] || ''" type="textarea" :rows="7" readonly aria-label="卸载命令" style="margin:16px 0"/>
    <el-button type="primary" @click="copy">复制卸载命令</el-button>
  </el-dialog>
</template>
