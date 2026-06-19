# WebUI 佈局重構 + 列表增強

**Created**: 2026-06-19
**Status**: 🟢 Complete
**Priority**: High

---

## Background & Goals

三項 UI 改善：

1. **去掉 sidebar，改為頂部 header**：品牌名放頂部，導航連結和 TaskProgress 整合到 header
2. **分頁引入 route query**：`page` 同步到 `?page=N`，支援書籤/分享/重新整理保留頁碼
3. **AI 索引標籤**：列表中有 AI 索引的項目顯示 "AI" badge（後端 `GET /media` 已回傳 `item.index` 欄位，前端只需檢查）
4. **按創建時間排序**：視頻列表按 `created` 欄位升序排列（最早創建的最前面）

## Technical Approach

### 1. 佈局：Sidebar → Top Header

**現狀**：桌面固定左 sidebar（`lg:fixed w-56`）+ 手機頂部 hamburger header
**目標**：統一頂部 header，包含品牌名 + 導航連結 + TaskProgress indicator

Header 佈局（桌面 & 手機統一）：
```
┌─────────────────────────────────────────────────────────┐
│ Wistia-S3    視頻管理  任務監控          ⚡ 3  (任務進度) │
└─────────────────────────────────────────────────────────┘
```

- 桌面：品牌名左側，導航連結中間/靠左，TaskProgress 收合為右側 badge（點擊展開下拉或跳轉 /tasks）
- 手機：品牌名左側，hamburger menu 展開導航，TaskProgress badge 右側
- `TaskProgress.vue` 重構為頂部 compact 模式：顯示 running count badge，點擊跳轉 /tasks

### 2. 分頁 route query

**現狀**：`page` 是 `ref(1)`，純前端狀態
**目標**：`page` 從 `route.query.page` 讀取，切換頁碼時 `router.push({ query: { ...route.query, page: N } })`

- 初始化：`const page = computed({ get: () => Number(route.query.page) || 1, set: ... })`
- 搜尋改變時：清除 page query（`router.push({ query: { ...route.query, page: undefined } })`）
- watch `filteredItems` 防止 page 超出範圍時同步到 query

### 3. AI 索引標籤

**現狀**：`GET /media` 回傳的每個 item 在存在 AI 索引時包含 `item.index` 物件
**目標**：在列表項目的名稱旁顯示 "AI" badge

- 桌面 table：名稱欄位旁加一個小 badge
- 手機 card：名稱下方加 badge
- Badge 樣式：`bg-purple-50 text-purple-600 text-xs px-1.5 py-0.5 rounded`，文字 "AI"

## Affected Files

| File | Action | Description |
|------|--------|-------------|
| `web/src/ui/components/AppLayout.vue` | Rewrite | Sidebar → top header，整合導航 + TaskProgress |
| `web/src/ui/components/TaskProgress.vue` | Modify | 改為 compact header badge 模式（running count + 點擊跳轉） |
| `web/src/ui/views/MediaLibrary.vue` | Modify | 分頁 sync route query + AI index badge + 按 created 升序排序 |
| `pkg/wistia.go` | Modify | 加入 `Created` 欄位到 `WistiaRespVideo` struct |

## Tasks

| # | Task | Status | Sub Agent | Notes |
|---|------|--------|-----------|-------|
| 1 | 重構 `AppLayout.vue`：sidebar → top header | ✅ Complete | general | 品牌名 + 導航 + TaskProgress badge |
| 2 | 重構 `TaskProgress.vue`：compact header badge | ✅ Complete | general | running count badge + 點擊跳轉 /tasks |
| 3 | `MediaLibrary.vue`：分頁 sync route query | ✅ Complete | general | page ↔ `?page=N` |
| 4 | `MediaLibrary.vue`：AI 索引 badge | ✅ Complete | general | 檢查 `item.index` 顯示 "AI" badge |
| 5 | `MediaLibrary.vue`：按 created 升序排序 | ✅ Complete | general | 加入 `Created` 欄位到 struct + 前端排序 |
| 6 | 驗證 `yarn build` + `docker build` | ✅ Complete | general | 通過 |

### Status Legend
- ⬜ Pending — not started
- 🔄 In Progress — sub agent working on it
- ✅ Complete — implemented and reviewed
- 🔴 Blocked — dependency or issue, needs resolution
- ⏭️ Skipped — not needed, with reason

## Testing Strategy

- `yarn build` 必須通過
- 手動驗證：分頁切換時 URL 更新、書籤可恢復頁碼、AI badge 正確顯示

## Review Log

| Date | Reviewer | Findings | Action Taken |
|------|----------|----------|--------------|
| 2026-06-19 | main | 全部任務完成，yarn build 通過 | 更新所有 task status 為 ✅ Complete |

## Open Questions

None.
