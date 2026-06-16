<template>
  <div v-if="show" class="fixed inset-0 z-50 flex items-center justify-center p-4">
    <div class="fixed inset-0 bg-black/30" @click="$emit('cancel')"></div>
    <div class="relative bg-white rounded border border-slate-200 shadow-sm max-w-sm w-full p-5">
      <h3 class="text-base font-semibold text-slate-900 mb-2">{{ title }}</h3>
      <p class="text-sm text-slate-500 mb-5">{{ message }}</p>
      <div class="flex justify-end gap-2">
        <button
          class="px-3 py-1.5 text-sm font-medium rounded border border-slate-200 text-slate-600 hover:bg-slate-50 min-h-11 min-w-11 transition-colors"
          @click="$emit('cancel')"
        >
          取消
        </button>
        <button
          class="px-3 py-1.5 text-sm font-medium rounded border border-blue-600 bg-blue-600 text-white hover:bg-blue-700 min-h-11 min-w-11 transition-colors"
          @click="$emit('confirm')"
        >
          確認
        </button>
      </div>
    </div>
  </div>
</template>

<script setup>
defineProps({
  show: { type: Boolean, default: false },
  title: { type: String, default: '確認操作' },
  message: { type: String, default: '' },
})

defineEmits(['confirm', 'cancel'])
</script>
