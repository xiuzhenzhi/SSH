<script setup lang="ts">
defineProps<{error?:Error}>();
function retry(){window.location.reload();}
</script>
<template>
  <div class="page-loading panel" role="status" :aria-busy="!error">
    <template v-if="error"><p>页面加载失败，请检查网络后重试。</p><button class="text-action" @click="retry">重新加载</button></template>
    <template v-else><span>正在加载…</span><div class="loading-line"></div><div class="loading-line short"></div></template>
  </div>
</template>
<style scoped>
.page-loading{padding:24px;color:var(--text-muted);font-size:13px;min-height:140px}.loading-line{height:14px;border-radius:6px;background:var(--surface-soft);margin-top:20px;animation:page-pulse 1.2s ease-in-out infinite}.loading-line.short{width:65%;margin-top:12px}@keyframes page-pulse{50%{opacity:.4}}@media(prefers-reduced-motion:reduce){.loading-line{animation:none}}
</style>
