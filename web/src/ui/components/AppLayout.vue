<template>
  <div class="min-h-screen bg-white">
    <header class="sticky top-0 z-20 flex items-center justify-between px-4 py-3 border-b border-slate-200 bg-white">
      <div class="flex items-center gap-6">
        <router-link to="/media" class="text-base font-semibold text-slate-900">Wistia-S3</router-link>
        <nav class="hidden lg:flex items-center gap-1">
          <router-link
            v-for="item in navItems"
            :key="item.path"
            :to="item.path"
            class="px-3 py-1.5 text-sm font-medium rounded transition-colors"
            :class="$route.path === item.path
              ? 'bg-blue-50 text-blue-600'
              : 'text-slate-600 hover:bg-slate-100 hover:text-slate-900'"
          >
            {{ item.label }}
          </router-link>
        </nav>
      </div>
      <div class="flex items-center gap-3">
        <TaskProgress />
        <button
          class="lg:hidden min-h-11 min-w-11 flex items-center justify-center rounded text-slate-500 hover:text-slate-900 hover:bg-slate-100 transition-colors"
          @click="menuOpen = !menuOpen"
        >
          <svg class="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path v-if="!menuOpen" stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 6h16M4 12h16M4 18h16"/>
            <path v-else stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12"/>
          </svg>
        </button>
      </div>
    </header>
    <div v-if="menuOpen" class="lg:hidden border-b border-slate-200 bg-white px-4 py-2">
      <router-link
        v-for="item in navItems"
        :key="item.path"
        :to="item.path"
        class="block px-3 py-2 text-sm font-medium rounded transition-colors"
        :class="$route.path === item.path
          ? 'bg-blue-50 text-blue-600'
          : 'text-slate-600 hover:bg-slate-100 hover:text-slate-900'"
        @click="menuOpen = false"
      >
        {{ item.label }}
      </router-link>
    </div>
    <main class="p-4 lg:p-6">
      <slot></slot>
    </main>
    <Toast />
  </div>
</template>

<script setup>
import { ref } from 'vue'
import Toast from './Toast.vue'
import TaskProgress from './TaskProgress.vue'

const menuOpen = ref(false)

const navItems = [
  { path: '/media', label: '視頻管理' },
  { path: '/tasks', label: '任務監控' },
]
</script>
