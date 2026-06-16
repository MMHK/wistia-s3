import { reactive } from 'vue'
import { getTask } from '../api'

const taskMap = reactive(new Map())
const pollingIntervals = new Map()

export function useTaskPolling() {
  const addTask = (task) => {
    taskMap.set(task.id, { ...task })
    if (task.status === 'running' || task.status === 'init') {
      startPolling(task.id)
    }
  }

  const startPolling = (taskId) => {
    if (pollingIntervals.has(taskId)) return
    const poll = async () => {
      try {
        const data = await getTask(taskId)
        taskMap.set(taskId, { ...data })
        if (data.status === 'finished' || data.status === 'error') {
          stopPolling(taskId)
        }
      } catch (e) {
        console.error('poll task error:', e)
      }
    }
    const interval = setInterval(poll, 3000)
    pollingIntervals.set(taskId, interval)
  }

  const stopPolling = (taskId) => {
    const interval = pollingIntervals.get(taskId)
    if (interval) {
      clearInterval(interval)
      pollingIntervals.delete(taskId)
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
