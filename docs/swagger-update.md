# Swagger JSON 全面更新

## 目標 / 背景

`webroot/swagger/js/swagger.json` 缺少 3 個 `/index` 路由，且所有 route 的 response 都沒有定義 schema（僅空的 `content: {}`）。本次更新：

1. **補齊缺失的 3 個路由**：`POST /index/{hash}`、`POST /index`、`GET /index/{hash}`
2. **全面補齊所有 route 的 response/request schema**，包括定義 `components/schemas`
3. **tags 維持空陣列**，與現有風格一致

## 技術方案

### 單一修改檔案

`webroot/swagger/js/swagger.json` — 純 JSON 文件，不涉及 Go 代碼或前端構建。

### 路由對照表（http.go vs swagger.json）

| Method | Path | Handler | 當前 swagger | 行動 |
|--------|------|---------|:---:|------|
| GET | `/media` | `GetAllVideo` | ✓ 存在 | 補 response schema |
| POST | `/move/{hash}` | `VideoToS3` | ✓ 存在 | 補 response schema |
| POST | `/move` | `VideoToS3` | ✓ 存在 | 補 response schema |
| POST | `/refresh/media` | `RefreshVideoInfo` | ✓ 存在 | 補 response schema |
| GET | `/tasks/{id}` | `GetTask` | ✓ 存在 | 補 response schema |
| POST | `/index/{hash}` | `IndexVideo` | **缺失** | 新增路由 + schema |
| POST | `/index` | `IndexAllVideo` | **缺失** | 新增路由 + schema |
| GET | `/index/{hash}` | `GetIndex` | **缺失** | 新增路由 + schema |

### components/schemas 定義清單

所有 route 的 response 統一包在 `APIResponse`（`{status, data}`）或 `APIStandardError`（`{status, error}`）中。

#### 通用 wrappers

| Schema | 來源 (Go struct) | 用途 |
|--------|------------------|------|
| `APIResponse` | `APIResponse` | 成功回應 wrapper，`data` 為 oneOf 各類型 |
| `APIStandardError` | `APIStandardError` | 錯誤回應 wrapper |

#### Task 相關

| Schema | 來源 | 說明 |
|--------|------|------|
| `Task` | `Task` | `{id, status, result?}` — 非同步任務。`status` enum: `init`, `running`, `finished`, `error` |
| `MoveToS3Result` | `MoveToS3Result` | `/move` task 的 result 陣列元素 |

#### Media 相關

| Schema | 來源 | 說明 |
|--------|------|------|
| `WistiaRespVideo` | `WistiaRespVideo` | `/media` response data 元素 |
| `WistiaRespVideoAsset` | `WistiaRespVideoAsset` | WistiaRespVideo.assets 元素 |
| `WistiaRespVideoThumbnail` | `WistiaRespVideoThumbnail` | WistiaRespVideo.thumbnail |
| `WistiaRespVideoProject` | `WistiaRespVideoProject` | WistiaRespVideo.project |

#### Index 相關

| Schema | 來源 | 說明 |
|--------|------|------|
| `DashScopeIndexResult` | `DashScopeIndexResult` | `/index` 的同步返回或 task.result |
| `DashScopeSubtitleEntry` | `DashScopeSubtitleEntry` | `{start, end, text}` |
| `DashScopeChapterEntry` | `DashScopeChapterEntry` | `{start, end, title}` |
| `DashScopeTokenUsage` | `DashScopeTokenUsage` | `{inputK, outputK, totalK}` |

#### Request body

| Schema | 來源 | 說明 |
|--------|------|------|
| `MultipleMediaBody` | `MultipleMediaBody` | `{media: string[]}` — `/move` 和 `/index` batch 的 body |

### POST /index/{hash} 雙行為說明

在 `description` 中說明：
- **cache hit**（BoltDB 已有 index 且 `force` 非 `true`）：直接同步回傳 `DashScopeIndexResult`
- **cache miss**：建立非同步任務，回傳 `Task`，需輪詢 `GET /tasks/{id}`

response 200 使用 `oneOf` 指向 `APIResponse`（data 為 `DashScopeIndexResult`）或 `Task`。

### Query 參數補充

- `POST /index/{hash}`：補 `force` query param（`string`, enum: `[true, false]`）
- `POST /move/{hash}`：已有 `forceRefresh` ✓
- `POST /move`：已有 `forceRefresh` ✓

## 任務清單

- [ ] 1. 在 `components.schemas` 中定義所有 schema（APIResponse、APIStandardError、Task、MoveToS3Result、WistiaRespVideo、WistiaRespVideoAsset、WistiaRespVideoThumbnail、WistiaRespVideoProject、MultipleMediaBody、DashScopeIndexResult、DashScopeSubtitleEntry、DashScopeChapterEntry、DashScopeTokenUsage）
- [ ] 2. 為現有 5 個路由補上 response schema（`$ref` 到 components）
- [ ] 3. 新增 `POST /index/{hash}` 路由（path param + query param `force` + 雙行為 response）
- [ ] 4. 新增 `POST /index` 路由（request body `MultipleMediaBody` + async Task response）
- [ ] 5. 新增 `GET /index/{hash}` 路由（path param + sync `DashScopeIndexResult` response）
- [ ] 6. JSON 格式驗證（確保 swagger.json 為有效 JSON）

## 受影響檔案

| 檔案 | 變更類型 |
|------|---------|
| `webroot/swagger/js/swagger.json` | 修改 |

## 測試策略

- `yarn serve` 啟動前端 dev server → 打開 Swagger UI 確認所有路由正確顯示
- 或直接瀏覽器打開 `/swagger/index.html` 確認 swagger.json 載入成功、無格式錯誤
- 不需要跑 Go 測試（純靜態 JSON 文件修改）

## 開放問題

無（所有決策已與使用者確認）。
