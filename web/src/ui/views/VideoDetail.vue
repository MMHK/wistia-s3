<template>
  <div>
    <router-link to="/media" class="inline-flex items-center gap-1 text-sm text-blue-600 hover:underline mb-4 min-h-11">
      <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M15 19l-7-7 7-7"/>
      </svg>
      返回視頻列表
    </router-link>

    <div v-if="loading" class="flex items-center justify-center py-12">
      <svg class="w-6 h-6 animate-spin text-blue-600" fill="none" viewBox="0 0 24 24">
        <circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"/>
        <path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4z"/>
      </svg>
    </div>

    <div v-else-if="!video" class="text-center py-12 text-sm text-slate-500">找不到視頻資料</div>

    <template v-else>
      <div class="mb-4">
        <h1 class="text-lg font-semibold text-slate-900">{{ video.name || '(未命名)' }}</h1>
        <div class="flex flex-wrap gap-x-4 gap-y-1 text-sm text-slate-500 mt-1">
          <span class="font-mono">{{ video.hashId }}</span>
          <span>時長: {{ formatDuration(video.duration) }}</span>
          <span v-if="video.id">ID: {{ video.id }}</span>
        </div>
      </div>

      <div class="mb-6 rounded overflow-hidden bg-black">
        <VideoPlayer
          ref="playerRef"
          :hash-id="props.hash"
          @timeupdate="onPlayerTimeUpdate"
        />
      </div>

      <div class="flex flex-wrap gap-2 mb-6">
        <button
          class="px-3 py-1.5 text-sm font-medium rounded border border-blue-600 bg-blue-600 text-white hover:bg-blue-700 min-h-11 transition-colors"
          @click="doMigrate"
        >
          遷移到S3
        </button>
        <button
          class="px-3 py-1.5 text-sm font-medium rounded border border-blue-600 text-blue-600 hover:bg-blue-50 min-h-11 transition-colors"
          @click="doForceMigrate"
        >
          強制重新遷移
        </button>
        <button
          class="px-3 py-1.5 text-sm font-medium rounded border border-slate-200 text-slate-600 hover:bg-slate-50 min-h-11 transition-colors"
          @click="doIndex"
        >
          AI索引
        </button>
        <button
          class="px-3 py-1.5 text-sm font-medium rounded border border-slate-200 text-slate-600 hover:bg-slate-50 min-h-11 transition-colors"
          @click="doForceIndex"
        >
          強制重建索引
        </button>
      </div>

      <div class="lg:grid lg:grid-cols-2 lg:gap-6">
        <div class="mb-6 lg:mb-0">
          <button
            class="flex items-center gap-2 text-sm font-semibold text-slate-900 mb-3 w-full text-left min-h-11 lg:cursor-default"
            @click="assetsOpen = !assetsOpen"
          >
            <svg class="w-4 h-4 transition-transform lg:hidden" :class="{ 'rotate-90': assetsOpen }" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 5l7 7-7 7"/>
            </svg>
            素材列表
            <span class="text-xs font-normal text-slate-500 ml-1">({{ assetCount }})</span>
          </button>
          <div :class="{ 'hidden lg:block': !assetsOpen }">
            <div v-if="assets.length === 0" class="text-sm text-slate-500 py-4">尚無素材</div>
            <div v-else class="border border-slate-200 rounded overflow-x-auto">
              <table class="w-full text-sm">
                <thead>
                  <tr class="border-b border-slate-200 bg-slate-50">
                    <th class="px-3 py-2 text-left">類型</th>
                    <th class="px-3 py-2 text-left">大小</th>
                    <th class="px-3 py-2 text-left">尺寸</th>
                  </tr>
                </thead>
                <tbody>
                  <tr v-for="(asset, i) in assets" :key="i" class="border-b border-slate-100">
                    <td class="px-3 py-2 text-slate-600">{{ asset.type }}</td>
                    <td class="px-3 py-2 text-slate-500">{{ formatSize(asset.fileSize) }}</td>
                    <td class="px-3 py-2 text-slate-500">{{ asset.width }}x{{ asset.height }}</td>
                  </tr>
                </tbody>
              </table>
            </div>
          </div>
        </div>

        <div>
          <button
            class="flex items-center gap-2 text-sm font-semibold text-slate-900 mb-3 w-full text-left min-h-11 lg:cursor-default"
            @click="aiOpen = !aiOpen"
          >
            <svg class="w-4 h-4 transition-transform lg:hidden" :class="{ 'rotate-90': aiOpen }" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 5l7 7-7 7"/>
            </svg>
            AI 索引結果
          </button>
          <div :class="{ 'hidden lg:block': !aiOpen }">
            <div v-if="indexLoading" class="flex items-center gap-2 py-4">
              <svg class="w-4 h-4 animate-spin text-blue-600" fill="none" viewBox="0 0 24 24">
                <circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"/>
                <path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4z"/>
              </svg>
              <span class="text-sm text-slate-500">載入中...</span>
            </div>
            <div v-else-if="!aiIndex" class="text-sm text-slate-500 py-4">
              尚無 AI 索引
              <button
                class="ml-2 px-2 py-1 text-xs font-medium rounded border border-blue-600 text-blue-600 hover:bg-blue-50 min-h-8 transition-colors"
                @click="doIndex"
              >
                生成索引
              </button>
            </div>
            <div v-else class="space-y-4">
              <div>
                <h4 class="text-xs font-medium text-slate-500 uppercase mb-1">摘要</h4>
                <p class="text-sm text-slate-700 leading-relaxed">{{ aiIndex.summary }}</p>
              </div>

              <div v-if="aiIndex.chapters && aiIndex.chapters.length > 0">
                <h4 class="text-xs font-medium text-slate-500 uppercase mb-1">
                  章節 ({{ aiIndex.chapters.length }})
                </h4>
                <div class="border border-slate-200 rounded divide-y divide-slate-100">
                  <div
                    v-for="(ch, i) in aiIndex.chapters"
                    :key="i"
                    class="flex gap-3 px-3 py-2 text-sm"
                  >
                    <span class="text-slate-500 font-mono flex-shrink-0 w-14">{{ formatTime(ch.start) }}</span>
                    <span class="text-slate-700">{{ ch.title }}</span>
                  </div>
                </div>
              </div>

              <div v-if="aiIndex.subtitles && aiIndex.subtitles.length > 0">
                <h4 class="text-xs font-medium text-slate-500 uppercase mb-1">
                  字幕 ({{ aiIndex.subtitles.length }} 筆)
                </h4>
                <div class="border border-slate-200 rounded max-h-64 overflow-y-auto divide-y divide-slate-100">
                  <div
                    v-for="(sub, i) in aiIndex.subtitles"
                    :key="i"
                    class="flex gap-3 px-3 py-1.5 text-sm"
                  >
                    <span class="text-slate-500 font-mono flex-shrink-0 w-28">
                      {{ formatTime(sub.start) }}-{{ formatTime(sub.end) }}
                    </span>
                    <span class="text-slate-700">{{ sub.text }}</span>
                  </div>
                </div>
              </div>

              <div v-if="aiIndex.tokenUsage" class="text-xs text-slate-500">
                Token 用量:
                {{ aiIndex.tokenUsage.inputK.toFixed(1) }}K 輸入 /
                {{ aiIndex.tokenUsage.outputK.toFixed(1) }}K 輸出 /
                {{ aiIndex.tokenUsage.totalK.toFixed(1) }}K 總計
              </div>
            </div>
          </div>
        </div>
      </div>

      <div v-if="subtitles.length > 0" class="mt-6">
        <button
          class="flex items-center gap-2 text-sm font-semibold text-slate-900 mb-3 w-full text-left min-h-11 lg:cursor-default"
          @click="subtitleEditorOpen = !subtitleEditorOpen"
        >
          <svg class="w-4 h-4 transition-transform lg:hidden" :class="{ 'rotate-90': subtitleEditorOpen }" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 5l7 7-7 7"/>
          </svg>
          字幕編輯器
          <span class="text-xs font-normal text-slate-500 ml-1">({{ subtitles.length }} 筆)</span>
        </button>
        <div :class="{ 'hidden lg:block': !subtitleEditorOpen }">
          <SubtitleEditor
            ref="subtitleEditorRef"
            :hash-id="props.hash"
            :initial-subtitles="subtitles"
            :get-current-time="getPlayerCurrentTime"
            @saved="onSubtitlesSaved"
          />
        </div>
      </div>
    </template>

    <ConfirmDialog
      :show="confirmDlg.show"
      :title="confirmDlg.title"
      :message="confirmDlg.message"
      @confirm="confirmDlg.onConfirm"
      @cancel="confirmDlg.show = false"
    />
  </div>
</template>

<script setup>
import { ref, computed, onMounted } from 'vue'
import { getMedia, moveVideo, indexVideo, getIndex } from '../api'
import { useTaskPolling } from '../composables/useTaskPolling'
import { useToast } from '../composables/useToast'
import ConfirmDialog from '../components/ConfirmDialog.vue'
import VideoPlayer from '../components/VideoPlayer.vue'
import SubtitleEditor from '../components/SubtitleEditor.vue'

const props = defineProps({ hash: { type: String, required: true } })

const { addTask } = useTaskPolling()
const { addToast } = useToast()

const video = ref(null)
const aiIndex = ref(null)
const loading = ref(false)
const indexLoading = ref(false)
const assetsOpen = ref(true)
const aiOpen = ref(true)
const subtitleEditorOpen = ref(true)
const playerRef = ref(null)
const subtitleEditorRef = ref(null)

const confirmDlg = ref({ show: false, title: '', message: '', onConfirm: () => {} })

const showConfirm = (title, message, onConfirm) => {
  confirmDlg.value = { show: true, title, message, onConfirm }
}

const assets = computed(() => (video.value && video.value.assets) ? video.value.assets : [])
const assetCount = computed(() => assets.value.length)

const subtitles = computed(() => {
  if (aiIndex.value && aiIndex.value.subtitles) {
    return aiIndex.value.subtitles
  }
  if (video.value && video.value.index && video.value.index.subtitles) {
    return video.value.index.subtitles
  }
  return []
})

const onPlayerTimeUpdate = (time) => {
  if (subtitleEditorRef.value) {
    subtitleEditorRef.value.updateCurrentIndex(time)
  }
}

const getPlayerCurrentTime = () => {
  if (playerRef.value) {
    return playerRef.value.getCurrentTime()
  }
  return 0
}

const onSubtitlesSaved = () => {
  fetchIndex()
}

const formatDuration = (sec) => {
  if (!sec) return '--:--'
  const m = Math.floor(sec / 60)
  const s = Math.floor(sec % 60)
  return `${m}:${s.toString().padStart(2, '0')}`
}

const formatTime = (sec) => {
  if (sec == null) return '--:--'
  const m = Math.floor(sec / 60)
  const s = Math.floor(sec % 60)
  return `${m}:${s.toString().padStart(2, '0')}`
}

const formatSize = (bytes) => {
  if (!bytes) return '-'
  if (bytes < 1024) return bytes + 'B'
  if (bytes < 1024 * 1024) return (bytes / 1024).toFixed(1) + 'KB'
  return (bytes / (1024 * 1024)).toFixed(1) + 'MB'
}

const fetchVideo = async () => {
  loading.value = true
  try {
    const data = await getMedia(props.hash)
    video.value = Array.isArray(data) && data.length > 0 ? data[0] : null
  } catch (e) {
    addToast('載入失敗: ' + e.message, 'error')
  } finally {
    loading.value = false
  }
}

const fetchIndex = async () => {
  indexLoading.value = true
  try {
    aiIndex.value = await getIndex(props.hash)
  } catch {
    aiIndex.value = null
  } finally {
    indexLoading.value = false
  }
}

onMounted(() => {
  fetchVideo()
  fetchIndex()
})

const doMigrate = () => {
  showConfirm('遷移視頻', `確定要遷移 ${props.hash} 到 S3 嗎？`, async () => {
    confirmDlg.value.show = false
    try {
      const task = await moveVideo(props.hash)
      addTask(task)
      addToast('遷移任務已啟動', 'success')
    } catch (e) {
      addToast('遷移失敗: ' + e.message, 'error')
    }
  })
}

const doForceMigrate = () => {
  showConfirm('強制重新遷移', `確定要強制重新遷移 ${props.hash} 嗎？這會覆蓋現有檔案。`, async () => {
    confirmDlg.value.show = false
    try {
      const task = await moveVideo(props.hash, true)
      addTask(task)
      addToast('強制遷移任務已啟動', 'success')
    } catch (e) {
      addToast('強制遷移失敗: ' + e.message, 'error')
    }
  })
}

const doIndex = () => {
  showConfirm('AI 索引', `確定要為 ${props.hash} 生成 AI 索引嗎？`, async () => {
    confirmDlg.value.show = false
    try {
      const task = await indexVideo(props.hash)
      addTask(task)
      addToast('AI 索引任務已啟動', 'success')
    } catch (e) {
      addToast('AI 索引失敗: ' + e.message, 'error')
    }
  })
}

const doForceIndex = () => {
  showConfirm('強制重建索引', `確定要強制重建 ${props.hash} 的 AI 索引嗎？`, async () => {
    confirmDlg.value.show = false
    try {
      const task = await indexVideo(props.hash, true)
      addTask(task)
      addToast('強制重建索引任務已啟動', 'success')
    } catch (e) {
      addToast('強制重建索引失敗: ' + e.message, 'error')
    }
  })
}
</script>
