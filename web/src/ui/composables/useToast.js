import { reactive } from 'vue'

const toasts = reactive([])
let nextId = 0

export function useToast() {
  const addToast = (message, type = 'info', duration = 3000) => {
    const id = nextId++
    toasts.push({ id, message, type })
    if (duration > 0) {
      setTimeout(() => {
        const idx = toasts.findIndex(t => t.id === id)
        if (idx !== -1) toasts.splice(idx, 1)
      }, duration)
    }
  }

  const removeToast = (id) => {
    const idx = toasts.findIndex(t => t.id === id)
    if (idx !== -1) toasts.splice(idx, 1)
  }

  return { toasts, addToast, removeToast }
}
