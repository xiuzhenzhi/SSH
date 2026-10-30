<script setup lang="ts">
import { computed } from 'vue';
const props=defineProps<{code?:string,name?:string}>();
// Resolve from the Vite project root so moving this component cannot drop flag assets.
const flags=import.meta.glob<string>('/node_modules/flag-icons/flags/4x3/*.svg',{query:'?url&no-inline',import:'default',eager:true});
const source=computed(()=>flags['/node_modules/flag-icons/flags/4x3/'+(props.code||'').toLowerCase()+'.svg']);
</script>
<template><img v-if="source" class="country-flag" :src="source" :alt="name||code" :title="name||code" width="24" height="18"/></template>
<style scoped>.country-flag{display:inline-block;width:24px;height:18px;object-fit:cover;border-radius:3px;box-shadow:0 0 0 1px #0000000a;flex-shrink:0;vertical-align:middle}</style>
