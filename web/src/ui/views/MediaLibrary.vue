<template>
  <div>
    <div class="flex flex-col gap-3 mb-4">
      <div class="flex flex-col sm:flex-row sm:items-center gap-2">
        <div class="relative flex-1">
          <svg class="absolute left-3 top-1/2 -translate-y-1/2 w-4 h-4 text-slate-400" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M21 21l-6-6m2-5a7 7 0 11-14 0 7 7 0 0114 0z"/>
          </svg>
          <input
            v-model="search"
            type="text"
            placeholder="搜尋 hash 或名稱..."
            class="w-full pl-9 pr-3 py-2 text-sm border border-slate-200 rounded min-h-11 focus:outline-none focus:border-blue-600 transition-colors"
          />
        </div>
        <div class="flex gap-2 flex-wrap">
          <button
            class="px-3 py-1.5 text-sm font-medium rounded border border-slate-200 text-slate-600 hover:bg-slate-50 min-h-11 transition-colors"
            @click="doRefreshS3"
            :disabled="loading"
          >
            刷新S3
          </button>
          <button
            v-if="selected.size > 0"
            class="px-3 py-1.5 text-sm font-medium rounded border border-blue-600 bg-blue-600 text-white hover:bg-blue-700 min-h-11 transition-colors"
            @click="doBatchMigrate"
          >
            批量遷移 ({{ selected.size }})
          </button>
          <button
            v-if="selected.size > 0"
            class="px-3 py-1.5 text-sm font-medium rounded border border-slate-200 text-slate-600 hover:bg-slate-50 min-h-11 transition-colors"
            @click="doBatchIndex"
          >
            批量AI索引 ({{ selected.size }})
          </button>
        </div>
      </div>
      <div v-if="selected.size > 0" class="text-xs text-slate-500">
        已選 {{ selected.size }} 項目
      </div>
    </div>

    <div v-if="loading" class="flex items-center justify-center py-12">
      <svg class="w-6 h-6 animate-spin text-blue-600" fill="none" viewBox="0 0 24 24">
        <circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"/>
        <path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4z"/>
      </svg>
    </div>

    <div v-else-if="paginatedItems.length === 0" class="text-center py-12 text-sm text-slate-500">
      {{ search ? '沒有符合的結果' : '尚無視頻資料，請先刷新S3' }}
    </div>

    <div class="hidden md:block overflow-x-auto border border-slate-200 rounded">
      <table class="w-full text-sm">
        <thead>
          <tr class="border-b border-slate-200 bg-slate-50">
            <th class="px-3 py-2 text-left w-10">
              <input type="checkbox" :checked="allSelected" @change="toggleSelectAll" class="min-h-11 min-w-11 cursor-pointer" />
            </th>
            <th class="px-3 py-2 text-left w-16">縮圖</th>
            <th class="px-3 py-2 text-left">名稱</th>
            <th class="px-3 py-2 text-left w-28">Hash</th>
            <th class="px-3 py-2 text-left w-16">時長</th>
            <th class="px-3 py-2 text-left w-36">操作</th>
          </tr>
        </thead>
        <tbody>
          <tr
            v-for="item in paginatedItems"
            :key="item.hashed_id"
            class="border-b border-slate-100 hover:bg-slate-50 transition-colors"
          >
            <td class="px-3 py-2">
              <input
                type="checkbox"
                :checked="selected.has(item.hashed_id)"
                @change="toggleSelect(item.hashed_id)"
                class="min-h-11 min-w-11 cursor-pointer"
              />
            </td>
            <td class="px-3 py-2">
              <img
                v-if="getCover(item)"
                :src="getCover(item)"
                class="w-12 h-8 object-cover rounded"
                @error="$event.target.style.display='none'"
              />
            </td>
            <td class="px-3 py-2">
              <router-link :to="`/video/${item.hashed_id}`" class="text-blue-600 hover:underline font-medium">
                {{ item.name || '(未命名)' }}
              </router-link>
            </td>
            <td class="px-3 py-2 font-mono text-xs text-slate-500">{{ item.hashed_id }}</td>
            <td class="px-3 py-2 text-slate-500">{{ formatDuration(item.duration) }}</td>
            <td class="px-3 py-2">
              <div class="flex gap-1">
                <button
                  class="px-2 py-1 text-xs font-medium rounded border border-blue-600 text-blue-600 hover:bg-blue-50 min-h-8 transition-colors"
                  @click.stop="doMigrate(item.hashed_id)"
                >
                  遷移
                </button>
                <button
                  class="px-2 py-1 text-xs font-medium rounded border border-slate-200 text-slate-600 hover:bg-slate-50 min-h-8 transition-colors"
                  @click.stop="doIndex(item.hashed_id)"
                >
                  AI索引
                </button>
              </div>
            </td>
          </tr>
        </tbody>
      </table>
    </div>

    <div class="md:hidden flex flex-col gap-3">
      <div
        v-for="item in paginatedItems"
        :key="item.hashed_id"
        class="border border-slate-200 rounded p-3"
      >
        <div class="flex items-start gap-3">
          <input
            type="checkbox"
            :checked="selected.has(item.hashed_id)"
            @change="toggleSelect(item.hashed_id)"
            class="mt-1 min-h-11 min-w-11 cursor-pointer flex-shrink-0"
          />
          <img
            v-if="getCover(item)"
            :src="getCover(item)"
            class="w-20 h-14 object-cover rounded flex-shrink-0"
            @error="$event.target.style.display='none'"
          />
          <div class="flex-1 min-w-0">
            <router-link :to="`/video/${item.hashed_id}`" class="text-sm font-medium text-blue-600 hover:underline block truncate">
              {{ item.name || '(未命名)' }}
            </router-link>
            <div class="text-xs text-slate-500 font-mono mt-0.5">{{ item.hashed_id }}</div>
            <div class="text-xs text-slate-500 mt-0.5">{{ formatDuration(item.duration) }}</div>
          </div>
        </div>
        <div class="flex gap-2 mt-3">
          <button
            class="flex-1 px-2 py-1.5 text-xs font-medium rounded border border-blue-600 text-blue-600 hover:bg-blue-50 min-h-11 transition-colors"
            @click="doMigrate(item.hashed_id)"
          >
            遷移
          </button>
          <button
            class="flex-1 px-2 py-1.5 text-xs font-medium rounded border border-slate-200 text-slate-600 hover:bg-slate-50 min-h-11 transition-colors"
            @click="doIndex(item.hashed_id)"
          >
            AI索引
          </button>
        </div>
      </div>
    </div>

    <div v-if="totalPages > 1" class="flex items-center justify-center gap-1 mt-4">
      <button
        class="px-3 py-1.5 text-sm rounded border border-slate-200 text-slate-600 hover:bg-slate-50 min-h-11 disabled:opacity-40 transition-colors"
        :disabled="page <= 1"
        @click="page--"
      >
        &lt;
      </button>
      <template v-for="p in visiblePages" :key="p">
        <button
          v-if="p !== '...'"
          class="px-3 py-1.5 text-sm rounded border min-h-11 transition-colors"
          :class="p === page ? 'border-blue-600 bg-blue-600 text-white' : 'border-slate-200 text-slate-600 hover:bg-slate-50'"
          @click="page = p"
        >
          {{ p }}
        </button>
        <span v-else class="px-2 text-slate-400">...</span>
      </template>
      <button
        class="px-3 py-1.5 text-sm rounded border border-slate-200 text-slate-600 hover:bg-slate-50 min-h-11 disabled:opacity-40 transition-colors"
        :disabled="page >= totalPages"
        @click="page++"
      >
        &gt;
      </button>
    </div>

    <ConfirmDialog
      :show="confirm.show"
      :title="confirm.title"
      :message="confirm.message"
      @confirm="confirm.onConfirm"
      @cancel="confirm.show = false"
    />
  </div>
</template>

<script setup>
import { ref, computed, onMounted, watch } from 'vue'
import { getMedia, moveVideo, moveBatch, refreshMedia, indexVideo, indexBatch } from '../api'
import { useTaskPolling } from '../composables/useTaskPolling'
import { useToast } from '../composables/useToast'
import ConfirmDialog from '../components/ConfirmDialog.vue'

const { addTask } = useTaskPolling()
const { addToast } = useToast()

const media = ref([])
const loading = ref(false)
const search = ref('')
const page = ref(1)
const selected = ref(new Set())
const perPage = 20

const confirm = ref({ show: false, title: '', message: '', onConfirm: () => {} })

const showConfirm = (title, message, onConfirm) => {
  confirm.value = { show: true, title, message, onConfirm }
}

const filteredItems = computed(() => {
  const q = search.value.toLowerCase().trim()
  if (!q) return media.value
  return media.value.filter(
    (m) => (m.name || '').toLowerCase().includes(q) || (m.hashed_id || '').toLowerCase().includes(q)
  )
})

const totalPages = computed(() => Math.ceil(filteredItems.value.length / perPage) || 1)

const paginatedItems = computed(() => {
  const start = (page.value - 1) * perPage
  return filteredItems.value.slice(start, start + perPage)
})

const visiblePages = computed(() => {
  const total = totalPages.value
  const current = page.value
  if (total <= 7) return Array.from({ length: total }, (_, i) => i + 1)
  const pages = []
  pages.push(1)
  if (current > 3) pages.push('...')
  for (let i = Math.max(2, current - 1); i <= Math.min(total - 1, current + 1); i++) {
    pages.push(i)
  }
  if (current < total - 2) pages.push('...')
  pages.push(total)
  return pages
})

const allSelected = computed(() => {
  if (paginatedItems.value.length === 0) return false
  return paginatedItems.value.every((item) => selected.value.has(item.hashed_id))
})

watch(search, () => { page.value = 1 })

watch(filteredItems, () => {
  if (page.value > totalPages.value) page.value = totalPages.value
})

const fetchData = async () => {
  loading.value = true
  try {
    const data = await getMedia()
    media.value = Array.isArray(data) ? data : []
  } catch (e) {
    addToast('載入失敗: ' + e.message, 'error')
  } finally {
    loading.value = false
  }
}

onMounted(fetchData)

const getCover = (item) => {
  if (!item.assets) return null
  const cover = item.assets.find((a) => a.type === 'StillImageFile')
  return cover ? cover.url : null
}

const formatDuration = (sec) => {
  if (!sec) return '--:--'
  const m = Math.floor(sec / 60)
  const s = Math.floor(sec % 60)
  return `${m}:${s.toString().padStart(2, '0')}`
}

const toggleSelect = (hash) => {
  const s = new Set(selected.value)
  if (s.has(hash)) s.delete(hash)
  else s.add(hash)
  selected.value = s
}

const toggleSelectAll = () => {
  if (allSelected.value) {
    const s = new Set(selected.value)
    paginatedItems.value.forEach((item) => s.delete(item.hashed_id))
    selected.value = s
  } else {
    const s = new Set(selected.value)
    paginatedItems.value.forEach((item) => s.add(item.hashed_id))
    selected.value = s
  }
}

const doMigrate = (hash) => {
  showConfirm('遷移視頻', `確定要遷移 ${hash} 到 S3 嗎？`, async () => {
    confirm.value.show = false
    try {
      const task = await moveVideo(hash)
      addTask(task)
      addToast('遷移任務已啟動', 'success')
    } catch (e) {
      addToast('遷移失敗: ' + e.message, 'error')
    }
  })
}

const doIndex = (hash) => {
  showConfirm('AI 索引', `確定要為 ${hash} 生成 AI 索引嗎？`, async () => {
    confirm.value.show = false
    try {
      const task = await indexVideo(hash)
      addTask(task)
      addToast('AI 索引任務已啟動', 'success')
    } catch (e) {
      addToast('AI 索引失敗: ' + e.message, 'error')
    }
  })
}

const doBatchMigrate = () => {
  const hashes = Array.from(selected.value)
  showConfirm('批量遷移', `確定要遷移 ${hashes.length} 個視頻到 S3 嗎？`, async () => {
    confirm.value.show = false
    try {
      const task = await moveBatch(hashes)
      addTask(task)
      addToast(`批量遷移任務已啟動 (${hashes.length} 個)`, 'success')
      selected.value = new Set()
    } catch (e) {
      addToast('批量遷移失敗: ' + e.message, 'error')
    }
  })
}

const doBatchIndex = () => {
  const hashes = Array.from(selected.value)
  showConfirm('批量 AI 索引', `確定要為 ${hashes.length} 個視頻生成 AI 索引嗎？`, async () => {
    confirm.value.show = false
    try {
      const task = await indexBatch(hashes)
      addTask(task)
      addToast(`批量 AI 索引任務已啟動 (${hashes.length} 個)`, 'success')
      selected.value = new Set()
    } catch (e) {
      addToast('批量 AI 索引失敗: ' + e.message, 'error')
    }
  })
}

const doRefreshS3 = () => {
  showConfirm('刷新 S3', '確定要從 S3 重新索引所有視頻嗎？', async () => {
    confirm.value.show = false
    try {
      const task = await refreshMedia()
      addTask(task)
      addToast('刷新 S3 任務已啟動', 'success')
    } catch (e) {
      addToast('刷新失敗: ' + e.message, 'error')
    }
  })
}
</script>
