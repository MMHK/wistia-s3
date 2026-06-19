<template>
  <span
    v-if="runningCount > 0"
    class="inline-flex items-center gap-1 px-2 py-1 text-xs font-medium rounded-full bg-amber-50 text-amber-600 cursor-pointer"
    @click="router.push('/tasks')"
  >
    <svg class="w-3 h-3 animate-spin" fill="none" viewBox="0 0 24 24">
      <circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"/>
      <path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4z"/>
    </svg>
    {{ runningCount }}
  </span>
  <span
    v-else-if="finishedCount > 0 || errorCount > 0"
    class="inline-flex items-center px-2 py-1 text-xs font-medium rounded-full bg-slate-100 text-slate-500 cursor-pointer"
    @click="router.push('/tasks')"
  >
    <span class="w-1.5 h-1.5 rounded-full bg-slate-400"></span>
  </span>
</template>

<script setup>
import { computed } from 'vue'
import { useRouter } from 'vue-router'
import { useTaskPolling } from '../composables/useTaskPolling'

const router = useRouter()
const { getAllTasks } = useTaskPolling()

const tasks = computed(() => getAllTasks())
const runningCount = computed(() => tasks.value.filter((t) => t.status === 'running' || t.status === 'init').length)
const finishedCount = computed(() => tasks.value.filter((t) => t.status === 'finished').length)
const errorCount = computed(() => tasks.value.filter((t) => t.status === 'error').length)
</script>
