# Video Indexing Feature — Gemini 2.5 Flash-Lite

## Goal / Background

Wistia 影片遷移到 S3 後，需要將影片內容轉為可索引的文本資料（字幕、摘要、章節），以支援全文搜尋和內容理解。

使用 **Gemini 2.5 Flash-Lite**（$0.10/1M input tokens, $0.40/1M output tokens）直接處理影片，一次 API call 產出 subtitle + summary + chapters，無需多階段 pipeline。

## Gemini Video Input 方案分析

Gemini API 提供三種影片輸入方式（2026/01 更新）：

### 方案 A: External URL（推薦）

```
POST /v1beta/models/gemini-2.5-flash-lite:generateContent
{
  "contents": [{
    "parts": [
      {"file_data": {"mime_type": "video/mp4", "file_uri": "https://demo.static.mixmedia.com/.../224.mp4"}},
      {"text": "...prompt..."}
    ]
  }]
}
```

- Gemini 2026/01 起支援公開 HTTPS URL 和 Signed URL
- **不需要上傳檔案**，Gemini 自行從 URL 抓取
- S3 上的影片已設 PublicRead → 直接可用
- 最簡單、最少步驟、無 temporary storage 管理
- 限制：URL 必須 publicly accessible

### 方案 B: File API（大檔案 / 私有檔案）

```
# Step 1: Start resumable upload
POST https://generativelanguage.googleapis.com/upload/v1beta/files
  X-Goog-Upload-Protocol: resumable
  X-Goog-Upload-Command: start
  {"file": {"display_name": "video"}}

# Step 2: Upload bytes
PUT ${upload_url}
  X-Goog-Upload-Offset: 0
  X-Goog-Upload-Command: upload, finalize
  (binary data)

# Step 3: Poll until ACTIVE
GET https://generativelanguage.googleapis.com/v1beta/files/{file_name}

# Step 4: Use in generateContent
{"file_data": {"mime_type": "video/mp4", "file_uri": "${file.uri}"}}
```

- 支援最大 **2GB/檔案**，project quota 20GB
- 檔案 temporary 存储 **48 小時**後自動刪除
- 適合：檔案 > 20MB、private 檔案、需要重複使用同一檔案
- 需要額外管理 upload + polling state

### 方案 C: Inline Data（小檔案）

```json
{"inline_data": {"mime_type": "video/mp4", "data": "<base64>"}}
```

- 總 request 限制 **20MB**（base64 增加 ~33% overhead → 實際 ~14MB 原始檔）
- 最簡單但限制最多

### 本專案採用：方案 A (External URL)

理由：
- Wistia assets 已上傳 S3，URL 為 public
- 使用最小影片（224.mp4, ~4.8MB）遠低於任何限制
- 零額外上傳步驟，一個 HTTP call 搞定
- File API 作為 future reference 記錄，暫不實作

## Technical Approach

### 架構

全部使用 **Gemini Batch API**（50% cost off），搭配 async task model。

```
POST /index/{hash}  →  建立 async task (status: running)
  → 從 BoltDB 讀取 video metadata
  → 取得 assets，選最小 video file
  → 組合 S3 public URL
  → 提交 Gemini Batch API request
  → 儲存 batch operation name 到 task metadata

Background Poller (每 5 分鐘):
  → 檢查所有 pending batch operations
  → 完成的 batch → 下載結果
  → 解析 response → 結構化 JSON
  → 上傳 index-ai.json + subtitles.vtt 到 S3（+ CloudFront mirror）
  → 寫入 BoltDB "index" bucket
  → task status = FINISHED

GET /tasks/{id}  →  輪詢 task 狀態
```

### API Routes

| Method | Path | Body | Response |
|--------|------|------|----------|
| POST | `/index/{hash}` | (none) | `{"status": true, "data": {"taskId": "..."}}` |
| POST | `/index` | `{"media": ["hash1","hash2"]}` | `{"status": true, "data": {"taskId": "..."}}` |
| GET | `/index/{hash}` | — | `{"status": true, "data": {subtitles, summary, chapters}}` |

Query parameters:
- `?force=true` — 覆蓋已有的 index-ai.json

### Gemini API 規格

| 項目 | 值 |
|------|-----|
| Model | `gemini-2.5-flash-lite` |
| Endpoint | `https://generativelanguage.googleapis.com/v1beta/models/gemini-2.5-flash-lite:generateContent?key={API_KEY}` |
| Input $/1M tokens | $0.10 (text/image/video), $0.30 (audio) |
| Output $/1M tokens | $0.40 |
| Context window | 1,048,576 tokens |
| Max output | 65,535 tokens |
| Input size limit | 500 MB |
| Video 長度上限 | ~45 min（含音訊） |
| Cache | cache_read $0.01/1M tokens |
| Batch API | 50% off, 全部走 Batch |

### Gemini Batch API 流程

```
1. 建立 batch request (JSONL 格式)
   POST https://generativelanguage.googleapis.com/v1beta/batches
   {
     "model": "gemini-2.5-flash-lite",
     "inputConfig": {
       "gcsUri": "..."  // 或用 inline requests
     }
   }

   實際做法（無需 GCS）：用 inline requests 直接提交
   POST https://generativelanguage.googleapis.com/v1beta/models/gemini-2.5-flash-lite/batchRequests
   Body: JSONL (每行一個 generateContent request)

2. Response 包含 operation name
   {
     "name": "operations/...",
     "metadata": {"state": "RUNNING"},
     "done": false
   }

3. Poll operation 直到 done
   GET https://generativelanguage.googleapis.com/v1beta/{operation_name}

4. 完成後下載結果
   Response.outputConfig.gcsDestination.outputUri
   (或使用 inline response 直接在 operation 中)
```

Batch API 特性：
- 所有 token 費用 **50% off**
- 處理時間最長 24 小時
- 非即時，適合 async task model
- 每支影片成本從 ~$0.11 降到 **~$0.055**

### 影片來源選擇邏輯

從 `WistiaRespVideo.Assets` 中：
1. 過濾 `GetVideoFiles()`（type contains "VideoFile"）
2. 排除 `OriginalFile` 和 `IphoneVideoFile`（可能品質不佳或過大）
3. 取 `FileSize` 最小的 asset

以範例資料為例：
- `Mp4VideoFile` 224.mp4 — 4.8MB ✅ 選這個
- `IphoneVideoFile` 360.mp4 — 6.7MB
- `MdMp4VideoFile` 540.mp4 — 9.3MB
- `HdMp4VideoFile` 720.mp4 — 12.5MB
- `HdMp4VideoFile` 1080.mp4 — 19.7MB
- `OriginalFile` original.mp4 — 112MB

### S3 URL 組合

```
https://{bucket}.s3.{region}.amazonaws.com/{prefix}/media/{hash}/{height}.mp4
```

或使用 CloudFront domain（若已設定）：
```
https://{cloudfront_domain}/{prefix}/media/{hash}/{height}.mp4
```

### Prompt 設計

```
Analyze this video and return ONLY a valid JSON object (no markdown, no explanation) with:
1. "summary": A concise summary of the video content in 2-4 sentences.
2. "subtitles": An array of subtitle entries with "start" (float, seconds), "end" (float, seconds), and "text" (the spoken words).
3. "chapters": An array of chapter markers with "start" (float, seconds), "end" (float, seconds), and "title" (a descriptive chapter title).
```

### 輸出 JSON 結構 (`index-ai.json`)

```json
{
  "hashId": "u7k1cgyjy0",
  "model": "gemini-2.5-flash-lite",
  "source": {
    "type": "Mp4VideoFile",
    "url": "https://demo.static.mixmedia.com/wistia-backup/media/u7k1cgyjy0/224.mp4",
    "fileSize": 4861043,
    "width": 400,
    "height": 224
  },
  "generatedAt": "2026-06-15T10:30:00Z",
  "summary": "This video covers...",
  "subtitles": [
    {"start": 0.0, "end": 3.5, "text": "Hello everyone"},
    {"start": 3.5, "end": 7.2, "text": "Today we will discuss..."}
  ],
  "chapters": [
    {"start": 0.0, "end": 45.0, "title": "Introduction"},
    {"start": 45.0, "end": 180.0, "title": "Main Topic"}
  ]
}
```

### 存放位置

**S3:**
- `media/{hashId}/index-ai.json` (PublicRead, ContentType: application/json) — 完整結果
- `media/{hashId}/subtitles.vtt` (PublicRead, ContentType: text/vtt) — VTT 字幕檔，video player 可直接引用
- `cloudfront/media/{hashId}/index-ai.json` (若 CloudFront 已設定)
- `cloudfront/media/{hashId}/subtitles.vtt` (若 CloudFront 已設定)

**BoltDB:**
- Bucket: `"index"`
- Key: `{hashId}`
- Value: JSON-serialized `GeminiIndexResult`

### VTT 字幕輸出

`GeminiIndexResult` 提供 `ToVTT()` 方法，從 subtitles 陣列產生 WebVTT 格式字串：

```
WEBVTT

00:00:00.000 --> 00:00:03.500
Hello everyone

00:00:03.500 --> 00:00:07.200
Today we will discuss...
```

時間戳轉換：`start`/`end` 為 float 秒數 → 格式化為 `HH:MM:SS.mmm`。
VTT 檔與 JSON 同時上傳到 S3，前端 `<track src="subtitles.vtt">` 即可使用。

## Task Breakdown

- [ ] 1. 新增 `GeminiConf` config struct（`pkg/gemini.go`）
  - `GeminiApiKey string`
  - `Model string` (default: `gemini-2.5-flash-lite`)
  - `MarginWithENV()` 讀取 `GEMINI_API_KEY`, `GEMINI_MODEL`
- [ ] 2. 新增 `Config.GeminiConf` field（`pkg/conf.go`）
- [ ] 3. 定義 `GeminiIndexResult` struct + `ToVTT()` method（`pkg/gemini.go`）
- [ ] 4. 新增 `GeminiHelper`（`pkg/gemini.go`）
  - `SubmitBatch(videoUrl, videoName string) (*BatchSubmitResult, error)` — 提交 Batch API
  - `CheckBatch(operationName string) (*BatchStatusResult, error)` — 檢查 batch 狀態
  - `ParseBatchResult(raw string) (*GeminiIndexResult, error)` — 解析結果
  - 使用 net/http 直接呼叫 REST API（不引入外部 SDK，保持 Go 1.19 vendor 乾淨）
- [ ] 5. 新增 BoltDB methods（`pkg/db.go`）
  - `SaveVideoIndex(hashId string, data *GeminiIndexResult) error`
  - `FindVideoIndex(hashId string) (*GeminiIndexResult, error)`
- [ ] 6. 新增 HTTP handlers（`pkg/http.go`）
  - `IndexVideo(w, r)` — POST `/index/{hash}`
  - `IndexAllVideo(w, r)` — POST `/index`
  - `GetIndex(w, r)` — GET `/index/{hash}`
  - 使用現有 async task pattern（generateID + tasks map + goroutine）
- [ ] 7. 新增 `IndexVideoToS3()` orchestrator（`pkg/http.go`）
  - 從 BoltDB 讀取 video metadata
  - 選擇最小 video file
  - 組合 S3 public URL
  - 提交 Gemini Batch API
  - 儲存 operation name 到 task
- [ ] 8. 新增 Batch result processor（`pkg/http.go`）
  - 處理 batch 完成後的結果
  - 解析 JSON → 產生 VTT
  - 上傳 `index-ai.json` + `subtitles.vtt` 到 S3
  - 寫入 BoltDB `"index"` bucket
  - task status = FINISHED
- [ ] 9. 註冊 routes（`pkg/http.go`）
  - `r.HandleFunc("/index/{hash}", s.IndexVideo).Methods("POST")`
  - `r.HandleFunc("/index", s.IndexAllVideo).Methods("POST")`
  - `r.HandleFunc("/index/{hash}", s.GetIndex).Methods("GET")`
- [ ] 10. 更新 `.env.example` + `docker-compose.yml`
  - `GEMINI_API_KEY=`
  - `GEMINI_MODEL=gemini-2.5-flash-lite`
- [ ] 11. Verification
  - `docker build`
  - `docker test ./pkg/...`（需要 GEMINI_API_KEY in .env）

## Affected Files

| File | Change |
|------|--------|
| `pkg/gemini.go` | **新增** — GeminiConf, GeminiHelper, GeminiIndexResult, ToVTT() |
| `pkg/conf.go` | **修改** — Config 加 GeminiConf field |
| `pkg/db.go` | **修改** — 新增 SaveVideoIndex, FindVideoIndex |
| `pkg/http.go` | **修改** — 新增 routes, handlers, orchestrator, batch result processor |
| `.env.example` | **修改** — 新增 GEMINI_API_KEY, GEMINI_MODEL |
| `docker-compose.yml` | **修改** — 新增 GEMINI_API_KEY env |
| `go.mod` / `vendor/` | **不變** — 使用 net/http，不引入新 dependency |

## Cost Estimation

全部使用 **Batch API**（50% off）：

### Per-video cost（10 分鐘影片）

| 項目 | Tokens | Cost (Sync) | Cost (Batch) |
|------|--------|------------|-------------|
| Video input (224.mp4) | ~1M | $0.10 | $0.05 |
| Output (subtitle + summary + chapters) | ~2K | $0.001 | $0.0005 |
| **Total** | | **~$0.11** | **~$0.055** |

### Batch cost

| 影片數 | 成本 (Batch) |
|--------|-------------|
| 10 支 | ~$0.55 |
| 100 支 | ~$5.50 |
| 1,000 支 | ~$55.00 |

### 成本優化

- **Batch API**: 全部走 Batch API，50% off → ~$0.055/支
- **選擇最小影片**: 224.mp4 (4.8MB) 比 1080.mp4 (19.7MB) 節省 ~75% input tokens

## Error Handling

| Error Case | Behavior |
|------------|----------|
| S3 URL 無效（影片未遷移） | 回 400: "video not found, run /move/{hash} first" |
| Gemini API 失敗 | Task status = error, message 含 Gemini error detail |
| Gemini response JSON parse 失敗 | 嘗試 recovery，失敗則存 raw response 供 debug |
| Gemini response 缺欄位 | 存已有欄位，缺的設為 null |
| BoltDB 寫入失敗 | S3 已成功則 log warning，不回退 |
| S3 上傳失敗 | Task status = error |
| 重複 index（無 ?force=true） | 回 200 + 既有結果，不重新呼叫 Gemini |

## Open Questions

- [ ] Gemini Free tier rate limit 是否足夠？（1500 RPD for Flash-Lite）
- [x] ~~是否需要支援 Batch API（50% off, 非即時）？~~ → 決定全部走 Batch API
- [x] ~~Subtitle 是否需要同時輸出 .vtt 格式供播放器使用？~~ → 決定同時輸出 JSON + VTT
- [ ] 是否需要支援多語言字幕輸出？
- [x] ~~是否需要 Gemini File API 支援（處理非公開影片）？~~ → 暫不實作，用 External URL

## Future: Gemini File API（暫不實作）

如果未來需要處理**非公開影片**或**超大檔案**，需引入 File API：

```
流程：
1. POST /upload/v1beta/files (start resumable upload)
   → 取得 upload_url
2. PUT upload_url (upload bytes, X-Goog-Upload-Command: upload, finalize)
   → 取得 file URI
3. Poll GET /v1beta/files/{name} 直到 state = ACTIVE
4. POST /v1beta/models/{model}:generateContent (file_data.file_uri)
5. 檔案 48 小時後自動刪除，不需手動清理
```

限制：
- 最大 2GB/檔案
- Project quota 20GB
- 檔案 48 小時後自動刪除
- 需要實作 multipart upload + polling state machine
