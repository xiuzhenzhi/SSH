<script setup lang="ts">
import { onUnmounted, ref, watch } from 'vue';
const props = defineProps<{value?: number}>();
const shown = ref(0);
let frame = 0;
watch(() => props.value, value => {
  cancelAnimationFrame(frame);
  const end = Number(value || 0), start = shown.value, at = performance.now();
  if (matchMedia('(prefers-reduced-motion: reduce)').matches) {shown.value = end; return;}
  const step = (now: number) => {
    const p = Math.min(1, (now-at)/650);
    shown.value = Math.round(start+(end-start)*(1-Math.pow(1-p,3)));
    if (p < 1) frame = requestAnimationFrame(step);
  };
  frame = requestAnimationFrame(step);
}, {immediate: true});
onUnmounted(() => cancelAnimationFrame(frame));
</script>
<template><span :aria-label="String(value ?? 0)">{{ shown.toLocaleString('zh-CN') }}</span></template>
