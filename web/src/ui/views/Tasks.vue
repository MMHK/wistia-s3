<template>
  <div>
    <div class="flex items-center justify-between mb-4">
      <h1 class="text-lg font-semibold text-slate-900">任務監控</h1>
      <span v-if="hasRunning" class="text-xs text-slate-500">自動 3s 刷新</span>
    </div>

    <div v-if="tasks.length === 0" class="text-center py-12 text-sm text-slate-500">
      尚無任務記錄
    </div>

    <div class="hidden md:block overflow-x-auto border border-slate-200 rounded">
      <table class="w-full text-sm">
        <thead>
          <tr class="border-b border-slate-200 bg-slate-50">
            <th class="px-3 py-2 text-left w-32">任務 ID</th>
            <th class="px-3 py-2 text-left w-24">狀態</th>
            <th class="px-3 py-2 text-left">結果</th>
          </tr>
        </thead>
        <tbody>
          <template v-for="task in tasks" :key="task.id">
            <tr
              class="border-b border-slate-100 hover:bg-slate-50 transition-colors cursor-pointer"
              @click="toggleExpand(task.id)"
            >
              <td class="px-3 py-2 font-mono text-xs text-slate-500">{{ shortId(task.id) }}</td>
              <td class="px-3 py-2"><TaskBadge :status="task.status" /></td>
              <td class="px-3 py-2 text-slate-600">{{ resultSummary(task) }}</td>
            </tr>
            <tr v-if="expandedId === task.id">
              <td colspan="3" class="px-3 py-3 bg-slate-50">
                <pre class="text-xs text-slate-600 whitespace-pre-wrap break-all max-h-64 overflow-y-auto">{{ JSON.stringify(task.result, null, 2) }}</pre>
              </td>
            </tr>
          </template>
        </tbody>
      </table>
    </div>

    <div class="md:hidden flex flex-col gap-3">
      <div
        v-for="task in tasks"
        :key="task.id"
        class="border border-slate-200 rounded p-3"
        @click="toggleExpand(task.id)"
      >
        <div class="flex items-center justify-between mb-1">
          <span class="font-mono text-xs text-slate-500">{{ shortId(task.id) }}</span>
          <TaskBadge :status="task.status" />
        </div>
        <div class="text-sm text-slate-600">{{ resultSummary(task) }}</div>
        <div v-if="expandedId === task.id" class="mt-2 pt-2 border-t border-slate-100">
          <pre class="text-xs text-slate-600 whitespace-pre-wrap break-all max-h-64 overflow-y-auto">{{ JSON.stringify(task.result, null, 2) }}</pre>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup>
import { ref, computed } from 'vue'
import { useTaskPolling } from '../composables/useTaskPolling'
import TaskBadge from '../components/TaskBadge.vue'

const { getAllTasks } = useTaskPolling()

const expandedId = ref(null)

const tasks = computed(() => getAllTasks())

const hasRunning = computed(() => tasks.value.some((t) => t.status === 'running' || t.status === 'init'))

const toggleExpand = (id) => {
  expandedId.value = expandedId.value === id ? null : id
}

const shortId = (id) => {
  if (!id) return ''
  return id.length > 8 ? id.slice(0, 8) + '..' : id
}

const resultSummary = (task) => {
  if (task.status === 'init') return '待處理'
  if (task.status === 'running') return '處理中...'
  if (task.status === 'error') {
    if (typeof task.result === 'string') return task.result
    return '發生錯誤'
  }
  if (task.status === 'finished') {
    if (Array.isArray(task.result)) {
      const ok = task.result.filter((r) => r.status).length
      return `${ok}/${task.result.length} 成功`
    }
    if (typeof task.result === 'object' && task.result !== null && task.result.hashId) {
      return `AI 索引完成 (${task.result.hashId})`
    }
    if (task.result === true) return '完成'
    return '已完成'
  }
  return ''
}
</script>
