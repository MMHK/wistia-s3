# Wistia Full Video Sync

**Created**: 2026-06-19
**Status**: 🟢 Complete
**Priority**: High

---

## Background & Goals

目前系統只透過 `GET /v1/medias/{hashId}.json` pull 單一視頻資訊。需要新增功能：從 Wistia API 拉取**所有** active + archived 視頻的 metadata，並快取到新的 BoltDB bucket，為後續 diff 功能做準備。

## Technical Approach

### API 選擇
- 繼續使用 **v1 API**：`GET https://api.wistia.com/v1/medias`
- Offset pagination：`?page={n}&per_page=50`
- 不設 `archived` 參數 → 預設返回全部（active + archived）
- 分頁直到返回空 array

### BoltDB 新 Bucket
- Bucket 名稱：`"wistia_catalog"`
- Key：`hashId`（string）
- Value：`WistiaRespVideo` JSON（與 `"media"` bucket 相同 schema）
- 另存 sync metadata 於 key `"__sync_meta"`：記錄最後 sync 時間、總數

### API Endpoint
- `POST /sync/wistia` — 觸發全量 sync（async task 模式）
- 狀態查詢：沿用現有 `GET /tasks/{id}` 輪詢

### 資料流
```
POST /sync/wistia → goroutine:
  1. page=1, per_page=50 → GET /v1/medias
  2. 解析 response → []*WistiaRespVideo
  3. 逐筆存入 BoltDB "wistia_catalog" bucket
  4. page++ 直到空 array
  5. 寫入 sync meta（timestamp, total count）
  6. Task status → FINISHED
```

## Affected Files / Routes / S3 Paths

### 新增/修改
| File | Change |
|------|--------|
| `pkg/wistia.go` | 新增 `ListAllVideos() ([]*WistiaRespVideo, error)` |
| `pkg/db.go` | 新增 bucket `"wistia_catalog"` 相關方法 |
| `pkg/handler_sync.go` | **新檔案** — `SyncWistiaVideos` + `GetWistiaMedia` handlers |
| `pkg/http.go` | 註冊新 routes `POST /sync/wistia`, `GET /wistia/media` |

### 不受影響
- S3 key layout — 不變
- 現有 `"media"` / `"index"` buckets — 不變
- 現有 API routes — 不變

## Tasks

| # | Task | Status | Sub Agent | Notes |
|---|------|--------|-----------|-------|
| 1 | 新增 `WistiaHelper.ListAllVideos()` — v1 API pagination | ✅ Complete | general | 分頁取所有 active + archived, lines 165-244 |
| 2 | 新增 BoltDB `"wistia_catalog"` bucket 方法 | ✅ Complete | — | Save/Find/GetAll + SyncMeta |
| 3 | 新增 `handler_sync.go` — `POST /sync/wistia` | ✅ Complete | general | handler_sync.go:1-80 |
| 4 | 註冊 route 到 `http.go` | ✅ Complete | general | http.go:109 |
| 5 | Code review | ✅ Complete | general | 無 critical issues，已修復 2 個 medium |
| 6 | Verify: docker build | ✅ Complete | — | docker build 通過 |
| 7 | 添加測試案例 | ✅ Complete | general | wistia_test.go + db_test.go |
| 8 | 新增 `GET /wistia/media` 查詢 API | ✅ Complete | general | handler_sync.go:88-189, 支援 hash/archived/pagination |
| 9 | 更新 swagger.json | ✅ Complete | general | /wistia/media path 已添加 |

### Status Legend
- ⬜ Pending — not started
- 🔄 In Progress — sub agent working on it
- ✅ Complete — implemented and reviewed
- 🔴 Blocked — dependency or issue, needs resolution
- ⏭️ Skipped — not needed, with reason

## Testing Strategy

所有測試皆為 integration test，需要真實 Wistia API credentials：

1. `TestWistiaHelper_ListAllVideos` — 確認能分頁取回所有視頻
2. `TestDBHelper_WistiaCatalog` — 確認新 bucket 的 save/find/getAll 正確
3. 手動測試：`POST /sync/wistia` → poll `GET /tasks/{id}` 確認完成

## Review Log

| Date | Reviewer | Findings | Action Taken |
|------|----------|----------|--------------|
| 2026-06-19 | general | 2 medium: partial failure discards results; 4 low | Fixed: return partial results on error, save partial in handler, init slice with make |

## Open Questions

- v1 list API 的 response 是否完全相容於現有 `WistiaRespVideo` struct？需要驗證 `assets` 欄位是否完整。
- `per_page` 最大值？Wistia 文檔未明確說明，先用 50 測試。
