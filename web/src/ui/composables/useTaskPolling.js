import { reactive } from 'vue'
import { getTask } from '../api'

const STORAGE_KEY = 'wistia-s3-tasks'
const MAX_TASKS = 50

const taskMap = reactive(new Map())
const pollingIntervals = new Map()

const stopPolling = (taskId) => {
  const interval = pollingIntervals.get(taskId)
  if (interval) {
    clearInterval(interval)
    pollingIntervals.delete(taskId)
  }
}

const saveToStorage = () => {
  try {
    let arr = Array.from(taskMap.values())
    if (arr.length > MAX_TASKS) {
      const active = arr.filter(t => t.status === 'running' || t.status === 'init')
      const done = arr.filter(t => t.status === 'finished' || t.status === 'error')
      const sorted = done.sort((a, b) => Number(BigInt(b.id) - BigInt(a.id)))
      arr = [...active, ...sorted].slice(0, MAX_TASKS)
    }
    localStorage.setItem(STORAGE_KEY, JSON.stringify(arr))
  } catch (e) {
    console.warn('task save failed:', e)
  }
}

const startPolling = (taskId) => {
  if (pollingIntervals.has(taskId)) return
  const poll = async () => {
    try {
      const data = await getTask(taskId)
      taskMap.set(taskId, { ...data })
      saveToStorage()
      if (data.status === 'finished' || data.status === 'error') {
        stopPolling(taskId)
      }
    } catch (e) {
      if (e.message && e.message.includes('task not found')) {
        taskMap.set(taskId, { id: taskId, status: 'error', result: '伺服器重啟，任務已遺失' })
        saveToStorage()
        stopPolling(taskId)
      } else {
        console.error('poll task error:', e)
      }
    }
  }
  const interval = setInterval(poll, 3000)
  pollingIntervals.set(taskId, interval)
}

const hydrate = () => {
  try {
    const raw = localStorage.getItem(STORAGE_KEY)
    if (!raw) return
    const saved = JSON.parse(raw)
    if (!Array.isArray(saved)) return
    for (const task of saved) {
      taskMap.set(task.id, { ...task })
      if (task.status === 'running' || task.status === 'init') {
        startPolling(task.id)
      }
    }
  } catch (e) {
    console.warn('task hydrate failed, clearing storage:', e)
    localStorage.removeItem(STORAGE_KEY)
  }
}

hydrate()

export function useTaskPolling() {
  const addTask = (task) => {
    taskMap.set(task.id, { ...task })
    saveToStorage()
    if (task.status === 'running' || task.status === 'init') {
      startPolling(task.id)
    }
  }

  const stopAll = () => {
    pollingIntervals.forEach((interval) => clearInterval(interval))
    pollingIntervals.clear()
  }

  const getAllTasks = () => Array.from(taskMap.values()).reverse()

  const getTaskData = (taskId) => taskMap.get(taskId)

  return { taskMap, addTask, startPolling, stopPolling, stopAll, getAllTasks, getTaskData }
}
