<template>
  <div>
    <div class="flex flex-wrap items-center gap-2 mb-3">
      <button
        class="px-3 py-1.5 text-sm font-medium rounded border border-slate-200 text-slate-600 hover:bg-slate-50 min-h-9 transition-colors"
        @click="addSubtitle"
      >
        + 新增字幕
      </button>
      <button
        v-if="hasChanges"
        class="px-3 py-1.5 text-sm font-medium rounded border border-blue-600 bg-blue-600 text-white hover:bg-blue-700 min-h-9 transition-colors"
        :disabled="saving"
        @click="save"
      >
        {{ saving ? '儲存中...' : '儲存' }}
      </button>
      <button
        v-if="hasChanges"
        class="px-3 py-1.5 text-sm font-medium rounded border border-slate-200 text-slate-600 hover:bg-slate-50 min-h-9 transition-colors"
        :disabled="saving"
        @click="cancel"
      >
        取消
      </button>
      <button
        class="px-3 py-1.5 text-sm font-medium rounded border border-slate-200 text-slate-600 hover:bg-slate-50 min-h-9 transition-colors ml-auto"
        @click="downloadVTT"
      >
        下載 VTT
      </button>
      <span v-if="hasChanges" class="text-xs text-amber-600">有未儲存的變更</span>
    </div>

    <div v-if="subtitles.length === 0" class="text-sm text-slate-500 py-4 text-center">
      尚無字幕資料
    </div>

    <div v-else class="border border-slate-200 rounded overflow-hidden">
      <div class="overflow-x-auto max-h-[500px] overflow-y-auto">
        <table class="w-full text-sm">
          <thead class="sticky top-0 bg-slate-50 z-10">
            <tr class="border-b border-slate-200">
              <th class="px-2 py-2 text-left w-10">#</th>
              <th class="px-2 py-2 text-left w-28">Start</th>
              <th class="px-2 py-2 text-left w-28">End</th>
              <th class="px-2 py-2 text-left">文字</th>
              <th class="px-2 py-2 text-center w-16">操作</th>
            </tr>
          </thead>
          <tbody>
            <tr
              v-for="(sub, i) in subtitles"
              :key="i"
              class="border-b border-slate-100"
              :class="{ 'bg-blue-50': currentIndex === i }"
            >
              <td class="px-2 py-1.5 text-slate-500 font-mono text-xs">{{ i + 1 }}</td>
              <td class="px-2 py-1.5">
                <div class="flex items-center gap-1">
                  <input
                    type="text"
                    :value="formatTimeInput(sub.start)"
                    class="w-20 px-1.5 py-1 text-xs font-mono border border-slate-200 rounded focus:border-blue-400 focus:outline-none"
                    @change="updateStart(i, $event.target.value)"
                  />
                  <button
                    class="p-1 text-slate-400 hover:text-blue-600 transition-colors"
                    title="擷取當前播放時間"
                    @click="captureTime(i, 'start')"
                  >
                    <svg class="w-3.5 h-3.5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                      <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M15 10l4.553-2.276A1 1 0 0121 8.618v6.764a1 1 0 01-1.447.894L15 14M5 18h8a2 2 0 002-2V8a2 2 0 00-2-2H5a2 2 0 00-2 2v8a2 2 0 002 2z"/>
                    </svg>
                  </button>
                </div>
              </td>
              <td class="px-2 py-1.5">
                <div class="flex items-center gap-1">
                  <input
                    type="text"
                    :value="formatTimeInput(sub.end)"
                    class="w-20 px-1.5 py-1 text-xs font-mono border border-slate-200 rounded focus:border-blue-400 focus:outline-none"
                    @change="updateEnd(i, $event.target.value)"
                  />
                  <button
                    class="p-1 text-slate-400 hover:text-blue-600 transition-colors"
                    title="擷取當前播放時間"
                    @click="captureTime(i, 'end')"
                  >
                    <svg class="w-3.5 h-3.5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                      <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M15 10l4.553-2.276A1 1 0 0121 8.618v6.764a1 1 0 01-1.447.894L15 14M5 18h8a2 2 0 002-2V8a2 2 0 00-2-2H5a2 2 0 002 2v8a2 2 0 002 2z"/>
                    </svg>
                  </button>
                </div>
              </td>
              <td class="px-2 py-1.5">
                <textarea
                  :value="sub.text"
                  class="w-full px-1.5 py-1 text-sm border border-slate-200 rounded focus:border-blue-400 focus:outline-none resize-none"
                  rows="1"
                  @input="updateText(i, $event.target.value)"
                />
              </td>
              <td class="px-2 py-1.5 text-center">
                <div class="flex items-center justify-center gap-1">
                  <button
                    class="p-1 text-slate-400 hover:text-red-600 transition-colors"
                    title="刪除"
                    @click="removeSubtitle(i)"
                  >
                    <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                      <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 7l-.867 12.142A2 2 0 0116.138 21H7.862a2 2 0 01-1.995-1.858L5 7m5 4v6m4-6v6m1-10V4a1 1 0 00-1-1h-4a1 1 0 00-1 1v3M4 7h16"/>
                    </svg>
                  </button>
                </div>
              </td>
            </tr>
          </tbody>
        </table>
      </div>
    </div>
  </div>
</template>

<script setup>
import { ref, computed, watch } from 'vue'
import { saveSubtitles } from '../api'
import { useToast } from '../composables/useToast'

const props = defineProps({
  hashId: { type: String, required: true },
  initialSubtitles: { type: Array, default: () => [] },
  getCurrentTime: { type: Function, default: () => 0 },
})

const emit = defineEmits(['saved'])

const { addToast } = useToast()

const subtitles = ref([])
const originalSubtitles = ref([])
const saving = ref(false)
const currentIndex = ref(-1)

const hasChanges = computed(() => {
  return JSON.stringify(subtitles.value) !== JSON.stringify(originalSubtitles.value)
})

watch(() => props.initialSubtitles, (val) => {
  subtitles.value = val ? val.map(s => ({ ...s })) : []
  originalSubtitles.value = val ? val.map(s => ({ ...s })) : []
}, { immediate: true, deep: true })

const formatTimeInput = (sec) => {
  if (sec == null) return '0:00.000'
  const m = Math.floor(sec / 60)
  const s = Math.floor(sec % 60)
  const ms = Math.round((sec - Math.floor(sec)) * 1000)
  return `${m}:${s.toString().padStart(2, '0')}.${ms.toString().padStart(3, '0')}`
}

const parseTimeInput = (str) => {
  str = str.trim()
  let parts = str.split(':')
  let m = 0, s = 0, ms = 0

  if (parts.length === 2) {
    m = parseInt(parts[0]) || 0
    const secParts = parts[1].split('.')
    s = parseInt(secParts[0]) || 0
    ms = parseInt(secParts[1] || '0')
    if (secParts[1] && secParts[1].length === 1) ms *= 100
    if (secParts[1] && secParts[1].length === 2) ms *= 10
  } else if (parts.length === 1) {
    const secParts = parts[0].split('.')
    s = parseInt(secParts[0]) || 0
    ms = parseInt(secParts[1] || '0')
  }

  return m * 60 + s + ms / 1000
}

const updateStart = (i, val) => {
  subtitles.value[i].start = parseTimeInput(val)
}

const updateEnd = (i, val) => {
  subtitles.value[i].end = parseTimeInput(val)
}

const updateText = (i, val) => {
  subtitles.value[i].text = val
}

const captureTime = (i, field) => {
  const time = props.getCurrentTime()
  subtitles.value[i][field] = time
}

const removeSubtitle = (i) => {
  subtitles.value.splice(i, 1)
}

const addSubtitle = () => {
  const lastEnd = subtitles.value.length > 0
    ? subtitles.value[subtitles.value.length - 1].end
    : 0
  subtitles.value.push({
    start: lastEnd,
    end: lastEnd + 3,
    text: '',
  })
}

const save = async () => {
  saving.value = true
  try {
    await saveSubtitles(props.hashId, subtitles.value)
    originalSubtitles.value = subtitles.value.map(s => ({ ...s }))
    addToast('字幕已儲存', 'success')
    emit('saved')
  } catch (e) {
    addToast('儲存失敗: ' + e.message, 'error')
  } finally {
    saving.value = false
  }
}

const cancel = () => {
  subtitles.value = originalSubtitles.value.map(s => ({ ...s }))
}

const downloadVTT = () => {
  let vtt = 'WEBVTT\n\n'
  subtitles.value.forEach((sub, i) => {
    if (i > 0) vtt += '\n'
    vtt += `${formatVTTTime(sub.start)} --> ${formatVTTTime(sub.end)}\n`
    vtt += `${sub.text}\n`
  })

  const blob = new Blob([vtt], { type: 'text/vtt' })
  const url = URL.createObjectURL(blob)
  const a = document.createElement('a')
  a.href = url
  a.download = `${props.hashId}.vtt`
  a.click()
  URL.revokeObjectURL(url)
}

const formatVTTTime = (sec) => {
  const h = Math.floor(sec / 3600)
  const m = Math.floor((sec % 3600) / 60)
  const s = Math.floor(sec % 60)
  const ms = Math.round((sec - Math.floor(sec)) * 1000)
  return `${h.toString().padStart(2, '0')}:${m.toString().padStart(2, '0')}:${s.toString().padStart(2, '0')}.${ms.toString().padStart(3, '0')}`
}

const updateCurrentIndex = (time) => {
  const idx = subtitles.value.findIndex(s => time >= s.start && time <= s.end)
  currentIndex.value = idx
}

defineExpose({ updateCurrentIndex })
</script>
