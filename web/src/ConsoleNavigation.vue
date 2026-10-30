<script setup lang="ts">
import type { Component } from 'vue';
import { ElIcon } from 'element-plus';
import { Lock } from '@element-plus/icons-vue';

defineProps<{
  items: {id: string; title: string; icon: Component}[];
  active: string;
  version?: string;
}>();
const emit = defineEmits<{navigate: [id: string]}>();
</script>

<template>
  <div class="console-navigation">
    <div class="sidebar-heading">
      <div class="brand"><el-icon><Lock/></el-icon> SSHGuard</div>
      <slot/>
    </div>
    <nav aria-label="主导航">
      <button v-for="item in items" :key="item.id" type="button"
        :class="{selected: active === item.id}"
        :aria-current="active === item.id ? 'page' : undefined"
        @click="emit('navigate', item.id)">
        <el-icon :class="['nav-icon', 'nav-icon-' + item.id]"><component :is="item.icon" :key="item.id"/></el-icon>
        {{ item.title }}
      </button>
    </nav>
    <div class="sidebar-bottom"><span class="dot"></span> {{ version || 'SSHGuard' }}</div>
  </div>
</template>
