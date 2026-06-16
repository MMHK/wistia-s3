<template>
  <span
    class="inline-flex items-center gap-1 px-2 py-0.5 text-xs font-medium rounded-full"
    :class="statusClass"
  >
    <svg v-if="status === 'running'" class="w-3 h-3 animate-spin" fill="none" viewBox="0 0 24 24">
      <circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"/>
      <path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4z"/>
    </svg>
    {{ label }}
  </span>
</template>

<script setup>
import { computed } from 'vue'

const props = defineProps({
  status: { type: String, required: true },
})

const label = computed(() => {
  const map = { init: '待處理', running: '執行中', finished: '已完成', error: '錯誤' }
  return map[props.status] || props.status
})

const statusClass = computed(() => {
  const map = {
    init: 'bg-slate-100 text-slate-600',
    running: 'bg-amber-50 text-amber-600',
    finished: 'bg-emerald-50 text-emerald-600',
    error: 'bg-red-50 text-red-600',
  }
  return map[props.status] || 'bg-slate-100 text-slate-600'
})
</script>
