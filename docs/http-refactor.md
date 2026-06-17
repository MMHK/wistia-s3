# HTTP Handler Refactor & Bug Fixes

## Goal / Background

`pkg/http.go` 已膨脹到 802 行，職責混雜（路由、media、move、index 全塞一起）。同時有兩個 bug：

1. `RefreshVideoInfo` 只拉 `index.json` 到 `"media"` bucket，**沒有**拉 `index-ai.json` 到 `"index"` bucket
2. `GetAllVideo` 只返回 `WistiaRespVideo`，**沒有**附帶對應的 `index-ai.json` 內容（即使 localDB 有存）

## Technical Approach

### Bug 1: `RefreshVideoInfo` 同步 index-ai.json

在現有的 `for _, row := range files` 迴圈裡，除了 `index.json` 的分支，新增一個 `index-ai.json` 的分支：

```go
if strings.HasSuffix(row, "index.json") {
    // 現有邏輯：fetch → SaveVideoInfo → "media" bucket
} else if strings.HasSuffix(row, "index-ai.json") {
    // 新增：fetch → json.Unmarshal → SaveVideoIndex → "index" bucket
    // 失敗只 log warning，不中斷
}
```

注意：`filepath.Base(strings.Replace(row, "/index-ai.json", "", 1))` 取 hashId。

### Bug 2: `GetAllVideo` 返回 index-ai 資訊

前端（MediaLibrary.vue、VideoDetail.vue）直接存取 `item.hashed_id`、`item.name`、`item.assets` 等，如果用 wrapper struct 會 breaking change。

改用 **flat merge** 方案：將 `WistiaRespVideo` marshal 成 `map[string]interface{}`，若有對應的 index 則加上 `"index"` 欄位。前端不需修改。

```go
func (s *HTTPService) videoWithIndex(v *WistiaRespVideo) map[string]interface{} {
    bin, _ := json.Marshal(v)
    var m map[string]interface{}
    json.Unmarshal(bin, &m)
    dbHelper := NewDBHelper(s.config.DBConf)
    idx, err := dbHelper.FindVideoIndex(v.HashId)
    if err == nil && idx != nil {
        m["index"] = idx
    }
    return m
}
```

- 單筆（`?hash=xxx`）：`FindVideoInfo` → `videoWithIndex` → 返回 `[]map`
- 全部：`GetAllVideoInfo` → 遍歷每個 video 呼叫 `videoWithIndex` → 返回 `[]map`

### Refactor: 分拆 http.go

| 檔案 | 內容 |
|------|------|
| `http.go` | 類型定義、struct、middleware、路由註冊、Start()、ResponseJSON/ResponseJSONError、NotFoundHandle、RedirectSwagger、GetTask |
| `handler_media.go` | `GetAllVideo`（含 VideoWithIndex）、`RefreshVideoInfo`（含 index-ai.json fix）、`SaveVideoInfo`、`FindVideoInfo` |
| `handler_move.go` | `VideoToS3`、`MoveVideoToS3` |
| `handler_index.go` | `IndexVideo`、`IndexAllVideo`、`GetIndex`、`indexVideoToS3` |

所有檔案都是 `package pkg`，methods on `*HTTPService`。共用變數 `tasks`、`tasksMu`、`generateID` 留在 `http.go`。

## Affected Files

- `pkg/http.go` — 瘦身為核心 + 路由
- `pkg/handler_media.go` — 新建
- `pkg/handler_move.go` — 新建
- `pkg/handler_index.go` — 新建
- `docs/http-refactor.md` — 本文件
- 前端 **不需修改**（flat merge 方案保持 response shape 相容）

## Tasks

- [x] 1. 修改 `GetAllVideo`：flat merge 方案，返回 `[]map[string]interface{}`，附帶 index 資訊
- [x] 2. 修改 `RefreshVideoInfo`：新增 `index-ai.json` 分支，呼叫 `SaveVideoIndex`
- [x] 3. 建立 `handler_media.go`，搬移 media 相關 handler（含上述 fixes）
- [x] 4. 建立 `handler_move.go`，搬移 move 相關 handler
- [x] 5. 建立 `handler_index.go`，搬移 index 相關 handler
- [x] 6. 瘦身 `http.go`：只留 struct 定義、路由、Start()、共用方法
- [x] 7. 驗證：`docker build` + `yarn build` ✅

## Testing Strategy

- 所有測試都是整合測試，需真實憑證
- `docker run --rm -v "${PWD}:/app" -w /app --env-file .env golang:1.19 go test ./pkg/...`
- 手動驗證：
  - `POST /refresh/media` → 確認 `"index"` bucket 有寫入
  - `GET /media` → 確認 response 包含 `index` 欄位（有值時）
  - `GET /media?hash=xxx` → 同上

## Open Questions

- ~~前端 `web/src/ui/` 是否有依賴 `GET /media` 的 response shape？~~ → 已確認：用 flat merge 方案，前端不需修改
