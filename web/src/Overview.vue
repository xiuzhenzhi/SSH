<script setup lang="ts">
import { computed, defineAsyncComponent } from 'vue';
import AnimatedNumber from './AnimatedNumber.vue';
const LoginTrend=defineAsyncComponent(()=>import('./LoginTrend.vue'));
const props = defineProps<{summary: Record<string,any>, overview: Record<string,any>, admin: boolean, range:string}>();
const emit = defineEmits<{navigate: [page: string],range:[value:string]}>();
const stamp = (value: string) => new Date(value).toLocaleTimeString('zh-CN',{hour:'2-digit',minute:'2-digit'});
const healthy = computed(() => props.overview.healthy_collectors || 0);
const percent = computed(() => props.summary.servers ? Math.min(100,healthy.value/props.summary.servers*100) : 0);
</script>
<template>
  <div class="overview">
    <div class="metrics">
      <article><div>在线服务器 <span class="metric-icon">◉</span></div><strong><AnimatedNumber :value="summary.online"/><em> / {{ summary.servers || 0 }}</em></strong></article>
      <article><div>24 小时成功登录 <span class="metric-icon">↗</span></div><strong><AnimatedNumber :value="summary.success_24h"/></strong></article>
      <article><div>24 小时认证失败 <span class="metric-icon warm">↘</span></div><strong><AnimatedNumber :value="summary.failure_24h"/></strong></article>
      <article :class="{attention:overview.unacknowledged_alerts}"><div>待处理告警 <span class="metric-icon warm">!</span></div><strong><AnimatedNumber :value="overview.unacknowledged_alerts"/></strong></article>
    </div>
    <div class="overview-grid">
      <LoginTrend :buckets="overview.trend||[]" :range="range" @range="value=>emit('range',value)"/>
      <section class="panel health-panel"><div class="panel-heading"><h3>采集与投递</h3><span class="subtle">运行状态</span></div><div class="health-content"><div class="health-ring" :style="{'--progress':percent+'%'}"><div><strong><AnimatedNumber :value="healthy"/></strong><small>采集正常</small></div></div><div class="health-caption">{{ summary.servers || 0 }} 台服务器中，{{ healthy }} 台正在正常采集</div><div class="health-item"><span>待投递通知</span><b>{{ overview.pending_deliveries || 0 }}</b></div><div class="health-item"><span>需检查的投递</span><b :class="{warning:overview.failed_deliveries}">{{ overview.failed_deliveries || 0 }}</b></div><button class="text-action" @click="emit('navigate',admin ? 'notifications' : 'servers')">{{ admin ? '查看投递记录' : '检查服务器' }} →</button></div></section>
    </div>
    <section class="panel recent-alerts"><div class="panel-heading"><h3>最近告警</h3><button class="text-action" @click="emit('navigate','alerts')">查看全部 →</button></div><div v-if="!overview.recent_alerts?.length" class="empty-state"><b>暂无告警记录</b></div><button v-for="item in overview.recent_alerts" :key="item.id" class="alert-preview" @click="emit('navigate','alerts')"><span :class="['alert-indicator',item.severity]"></span><div><b>{{ item.title }}</b><small>{{ item.server_name || '系统测试' }}</small></div><span :class="['status',item.resolved_at ? 'online' : item.acknowledged_at ? '' : 'pending']">{{ item.resolved_at ? '已恢复' : item.acknowledged_at ? '已确认' : '待处理' }}</span><time>{{ stamp(item.created_at) }}</time></button></section>
  </div>
</template>
