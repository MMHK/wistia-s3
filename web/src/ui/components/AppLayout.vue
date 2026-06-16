<template>
  <div class="min-h-screen bg-white">
    <div class="lg:fixed lg:inset-y-0 lg:flex lg:w-56 lg:flex-col">
      <div class="flex items-center justify-between px-4 py-3 border-b border-slate-200 lg:border-b-0 lg:border-r lg:h-14">
        <span class="text-base font-semibold text-slate-900">Wistia-S3</span>
        <button
          class="lg:hidden min-h-11 min-w-11 flex items-center justify-center rounded text-slate-500 hover:text-slate-900 hover:bg-slate-100 transition-colors"
          @click="sidebarOpen = !sidebarOpen"
        >
          <svg class="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path v-if="!sidebarOpen" stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 6h16M4 12h16M4 18h16"/>
            <path v-else stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12"/>
          </svg>
        </button>
      </div>
      <nav v-show="sidebarOpen" class="flex flex-col gap-1 p-3 lg:flex lg:border-r lg:border-slate-200 lg:h-full">
        <router-link
          v-for="item in navItems"
          :key="item.path"
          :to="item.path"
          class="flex items-center gap-2 px-3 py-2 text-sm font-medium rounded min-h-11 transition-colors"
          :class="$route.path === item.path
            ? 'bg-blue-50 text-blue-600'
            : 'text-slate-600 hover:bg-slate-100 hover:text-slate-900'"
          @click="sidebarOpen = false"
        >
          <span v-html="item.icon"></span>
          {{ item.label }}
        </router-link>
      </nav>
    </div>
    <div class="lg:pl-56">
      <header class="sticky top-0 z-10 flex items-center px-4 py-3 border-b border-slate-200 bg-white lg:hidden">
        <button
          class="min-h-11 min-w-11 flex items-center justify-center rounded text-slate-500 hover:text-slate-900 hover:bg-slate-100 transition-colors"
          @click="sidebarOpen = !sidebarOpen"
        >
          <svg class="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 6h16M4 12h16M4 18h16"/>
          </svg>
        </button>
        <span class="ml-3 text-base font-semibold text-slate-900">管理後台</span>
      </header>
      <main class="p-4 lg:p-6">
        <slot></slot>
      </main>
    </div>
    <Toast />
  </div>
</template>

<script setup>
import { ref } from 'vue'
import Toast from './Toast.vue'

const sidebarOpen = ref(false)

const navItems = [
  {
    path: '/media',
    label: '視頻管理',
    icon: '<svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M15 10l4.553-2.276A1 1 0 0121 8.618v6.764a1 1 0 01-1.447.894L15 14M5 18h8a2 2 0 002-2V8a2 2 0 00-2-2H5a2 2 0 00-2 2v8a2 2 0 002 2z"/></svg>',
  },
  {
    path: '/tasks',
    label: '任務監控',
    icon: '<svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 5H7a2 2 0 00-2 2v12a2 2 0 002 2h10a2 2 0 002-2V7a2 2 0 00-2-2h-2M9 5a2 2 0 002 2h2a2 2 0 002-2M9 5a2 2 0 012-2h2a2 2 0 012 2"/></svg>',
  },
]
</script>
