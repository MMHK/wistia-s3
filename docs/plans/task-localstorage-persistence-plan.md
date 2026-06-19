# Task localStorage 持久化

**Created**: 2026-06-19
**Status**: 🟢 Complete
**Priority**: Medium

---

## Background & Goals

目前 `useTaskPolling.js` 的 `taskMap` 是純記憶體 `reactive(new Map())`，頁面刷新後所有任務紀錄消失。使用者啟動了一個長時間遷移任務後刷新頁面，會完全失去該任務的追蹤。

**目標**：
1. 刷新頁面後，恢復任務列表並繼續 poll running 任務
2. 不修改任何 call site（`MediaLibrary.vue`、`VideoDetail.vue` 等），所有邏輯封裝在 `useTaskPolling.js` 內
3. 避免 localStorage 無限膨脹

## Technical Approach

### 儲存策略

- **Key**: `wistia-s3-tasks`
- **Value**: JSON array of `{id, status, result}` objects（從 `taskMap` 轉出）
- **寫入時機**: 每次 `taskMap` 變化時寫入（`addTask`、poll 更新、stopPolling）
- **讀取時機**: 模組載入時 hydrate 一次，恢復 `taskMap` 並對 `running`/`init` 任務重啟 polling

### Hydration 流程

```
App 載入 → useTaskPolling 模組初始化
  → 讀取 localStorage['wistia-s3-tasks']
  → 解析 JSON → 逐筆寫入 taskMap
  → 對 status === 'running' || 'init' 的任務呼叫 startPolling()
  → 若 poll 回傳 404（後端重啟），標記為 error 並停止 polling
```

### Cleanup 策略

- **上限**: 最多保留 **50 筆**任務（按加入時間排序）
- **淘汰**: 超出上限時，優先移除最舊的 `finished`/`error` 任務
- **寫入前修剪**: 每次 `saveToStorage()` 時檢查數量，超限則截斷

### 錯誤處理

- `localStorage` 寫入失敗（quota exceeded）→ `console.warn`，不影響功能
- `localStorage` 讀取失敗（corrupt JSON）→ 清空該 key，從空狀態開始
- Poll 回傳 404（後端重啟，任務不存在）→ 標記為 `error`，result 設為 "伺服器重啟，任務已遺失"

## Affected Files

| File | Action | Description |
|------|--------|-------------|
| `web/src/ui/composables/useTaskPolling.js` | Modify | 加入 localStorage 讀寫 + hydration 邏輯 |

**不需要修改的檔案**：`main.js`、`App.vue`、所有 view/component — 所有邏輯封裝在 composable 內。

## Implementation Detail

### 新增的內部函式（不 export）

```js
const STORAGE_KEY = 'wistia-s3-tasks'
const MAX_TASKS = 50

// localStorage → taskMap，恢復 running 任務的 polling
function hydrate() {
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

// taskMap → localStorage，附帶 cleanup
function saveToStorage() {
  try {
    let arr = Array.from(taskMap.values())
    // 超過上限時，淘汰最舊的已完成/錯誤任務
    if (arr.length > MAX_TASKS) {
      const done = arr.filter(t => t.status === 'finished' || t.status === 'error')
      const active = arr.filter(t => t.status === 'running' || t.status === 'init')
      // 保留所有 active + 最新的 done
      const sorted = done.sort((a, b) => Number(BigInt(b.id) - BigInt(a.id)))
      arr = [...active, ...sorted].slice(0, MAX_TASKS)
    }
    localStorage.setItem(STORAGE_KEY, JSON.stringify(arr))
  } catch (e) {
    console.warn('task save failed:', e)
  }
}
```

### 修改點（在現有函式內插入 `saveToStorage()` 呼叫）

| 位置 | 插入點 |
|------|--------|
| `addTask()` | `taskMap.set(...)` 之後 |
| `startPolling()` 內的 poll 成功路徑 | `taskMap.set(taskId, ...)` 之後 |
| `startPolling()` 內的 catch | catch block 末尾（可選：不存，因為下次 poll 會再試） |

### 404 處理（在 poll 的 catch 中）

目前 `getTask()` API 在 404 時會 throw（`api/index.js` 的 `if (!json.status) throw json.error`）。需要在 poll 函式中區分 404 和其他錯誤：

```js
const poll = async () => {
  try {
    const data = await getTask(taskId)
    taskMap.set(taskId, { ...data })
    saveToStorage()
    if (data.status === 'finished' || data.status === 'error') {
      stopPolling(taskId)
    }
  } catch (e) {
    // 404 = 後端重啟，任務不存在
    if (e === 'task not found' || (e.message && e.message.includes('not found'))) {
      taskMap.set(taskId, { id: taskId, status: 'error', result: '伺服器重啟，任務已遺失' })
      saveToStorage()
      stopPolling(taskId)
    } else {
      console.error('poll task error:', e)
    }
  }
}
```

### 模組初始化

在模組底部（`export` 之前）呼叫 `hydrate()`：

```js
// 模組載入時恢復任務
hydrate()

export function useTaskPolling() { ... }
```

## Tasks

| # | Task | Status | Sub Agent | Notes |
|---|------|--------|-----------|-------|
| 1 | 修改 `useTaskPolling.js` 加入 localStorage 持久化 | ✅ Complete | general | hydrate + saveToStorage + 404 處理 |
| 2 | 驗證 `yarn build` 通過 | ✅ Complete | general | 通過 |

### Status Legend
- ⬜ Pending — not started
- 🔄 In Progress — sub agent working on it
- ✅ Complete — implemented and reviewed
- 🔴 Blocked — dependency or issue, needs resolution
- ⏭️ Skipped — not needed, with reason

## Testing Strategy

- 無單元測試（前端無 test framework）
- 手動驗證：啟動 dev server → 觸發遷移 → 刷新頁面 → 確認任務恢復且 polling 繼續
- `yarn build` 必須通過

## Review Log

| Date | Reviewer | Findings | Action Taken |
|------|----------|----------|--------------|

## Open Questions

None.
