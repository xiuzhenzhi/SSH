<script setup lang="ts">
import { computed } from 'vue';
import AnimatedNumber from './AnimatedNumber.vue';
const props = defineProps<{summary: Record<string,any>, overview: Record<string,any>, admin: boolean}>();
const emit = defineEmits<{navigate: [page: string]}>();
const trend = computed(() => props.overview.trend || []);
const ceiling = computed(() => Math.max(4,...trend.value.map((v: any) => Math.max(v.success,v.failure))));
const points = (kind: string) => trend.value.map((v: any,i: number) => `${30+i*30},${170-v[kind]/ceiling.value*135}`).join(' ');
const stamp = (value: string) => new Date(value).toLocaleTimeString('zh-CN',{hour:'2-digit',minute:'2-digit'});
const healthy = computed(() => props.overview.healthy_collectors || 0);
const percent = computed(() => props.summary.servers ? Math.min(100,healthy.value/props.summary.servers*100) : 0);
</script>
<template>
  <div class="overview">
    <section class="overview-welcome"><div><span class="eyebrow">YOUR INFRASTRUCTURE, IN VIEW</span><h2>连接有迹，安全有数。</h2><p>从一次登录到一次告警，关注此刻真正需要处理的事。</p></div><div class="overview-clock"><span class="dot"></span>最近同步 <b>{{ overview.as_of ? stamp(overview.as_of) : '等待数据' }}</b></div></section>
    <div class="metrics">
      <article><div>在线服务器 <span class="metric-icon">◉</span></div><strong><AnimatedNumber :value="summary.online"/><em> / {{ summary.servers || 0 }}</em></strong><small>{{ summary.offline_or_pending || 0 }} 台离线或待接入</small></article>
      <article><div>24 小时成功登录 <span class="metric-icon">↗</span></div><strong><AnimatedNumber :value="summary.success_24h"/></strong><small>已持久化的 SSH 认证记录</small></article>
      <article><div>24 小时认证失败 <span class="metric-icon warm">↘</span></div><strong><AnimatedNumber :value="summary.failure_24h"/></strong><small>留意同一来源的连续失败</small></article>
      <article :class="{attention:overview.unacknowledged_alerts}"><div>待处理告警 <span class="metric-icon warm">!</span></div><strong><AnimatedNumber :value="overview.unacknowledged_alerts"/></strong><button class="text-action" @click="emit('navigate','alerts')">前往告警中心 →</button></article>
    </div>
    <div class="overview-grid">
      <section class="panel trend-panel"><div class="panel-heading"><div><h3>登录活动</h3><p>过去 24 个小时 · 按小时汇总</p></div><div class="legend"><span><i class="success-line"></i>成功</span><span><i class="failure-line"></i>失败</span></div></div><div class="chart-wrap">
        <svg viewBox="0 0 750 205" role="img" :aria-label="`24 小时登录趋势，成功 ${summary.success_24h || 0} 次，失败 ${summary.failure_24h || 0} 次`">
          <defs><linearGradient id="trend-fill" x1="0" y1="0" x2="0" y2="1"><stop offset="0%" stop-color="#6366f1" stop-opacity=".16"/><stop offset="100%" stop-color="#6366f1" stop-opacity="0"/></linearGradient></defs>
          <g v-for="n in 4" :key="n"><line x1="30" x2="720" :y1="170-(n-1)*45" :y2="170-(n-1)*45" stroke="#eef0f5" stroke-dasharray="4 5"/></g>
          <polygon v-if="trend.length" :points="`30,170 ${points('success')} 720,170`" fill="url(#trend-fill)"/>
          <polyline :points="points('success')" fill="none" stroke="#6366f1" stroke-width="2.5" stroke-linejoin="round" stroke-linecap="round"/>
          <polyline :points="points('failure')" fill="none" stroke="#df9c45" stroke-width="2" stroke-linejoin="round" stroke-linecap="round"/>
          <template v-for="(bucket,i) in trend" :key="bucket.at"><circle :cx="30+Number(i)*30" :cy="170-bucket.success/ceiling*135" r="3" fill="#6366f1"><title>{{ stamp(bucket.at) }} · 成功 {{ bucket.success }} · 失败 {{ bucket.failure }}</title></circle><text v-if="Number(i)%6===0 || Number(i)===23" :x="30+Number(i)*30" y="198" text-anchor="middle" fill="#9299ab" font-size="10">{{ stamp(bucket.at) }}</text></template>
        </svg><p v-if="!summary.events_24h" class="chart-empty">暂无登录活动。接入 Agent 后，真实事件将在这里呈现。</p>
      </div></section>
      <section class="panel health-panel"><div class="panel-heading"><h3>采集与投递</h3><span class="subtle">运行状态</span></div><div class="health-content"><div class="health-ring" :style="{'--progress':percent+'%'}"><div><strong>{{ healthy }}</strong><small>采集正常</small></div></div><div class="health-caption">{{ summary.servers || 0 }} 台服务器中，{{ healthy }} 台正在正常采集</div><div class="health-item"><span>待投递通知</span><b>{{ overview.pending_deliveries || 0 }}</b></div><div class="health-item"><span>需检查的投递</span><b :class="{warning:overview.failed_deliveries}">{{ overview.failed_deliveries || 0 }}</b></div><button class="text-action" @click="emit('navigate',admin ? 'notifications' : 'servers')">{{ admin ? '查看投递记录' : '检查服务器' }} →</button></div></section>
    </div>
    <section v-if="!summary.servers || !overview.enabled_channels" class="setup-card"><span class="setup-mark">↗</span><div><h3>{{ !summary.servers ? '从第一台服务器开始' : '让重要事件及时找到你' }}</h3><p>{{ !summary.servers ? '添加服务器，复制安装命令，随后检查采集状态。' : '配置通知渠道并发送测试通知，确认消息能够到达。' }}</p></div><button class="primary-action" @click="emit('navigate',!summary.servers || !admin ? 'servers' : 'channels')">{{ !summary.servers || !admin ? '管理服务器' : '配置通知渠道' }} →</button></section>
    <section class="panel recent-alerts"><div class="panel-heading"><h3>最近告警</h3><button class="text-action" @click="emit('navigate','alerts')">查看全部 →</button></div><div v-if="!overview.recent_alerts?.length" class="empty-state"><b>暂无告警记录</b><p>规则命中后会显示在这里；请同时确认采集和通知渠道正常。</p></div><button v-for="item in overview.recent_alerts" :key="item.id" class="alert-preview" @click="emit('navigate','alerts')"><span :class="['alert-indicator',item.severity]"></span><div><b>{{ item.title }}</b><small>{{ item.server_name || '系统测试' }}</small></div><span :class="['status',item.resolved_at ? 'online' : item.acknowledged_at ? '' : 'pending']">{{ item.resolved_at ? '已恢复' : item.acknowledged_at ? '已确认' : '待处理' }}</span><time>{{ stamp(item.created_at) }}</time></button></section>
  </div>
</template>
