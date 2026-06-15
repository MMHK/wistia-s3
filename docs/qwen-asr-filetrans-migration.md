# ASR 改用 qwen3-asr-flash-filetrans + 原生 API

## 狀態：✅ 已完成

## 目標
將 `Transcribe()` 從 compatible-mode（需 ffmpeg + base64 音頻）改為 DashScope 原生非實時轉寫 API：
- 模型：`qwen3-asr-flash-filetrans`
- 直接傳入影片 URL，無需 ffmpeg
- 異步模式（submit → poll → get result），有時間戳

## API 格式

### 提交任務
```
POST {BaseURL}/api/v1/services/audio/asr/transcription
Headers: X-DashScope-Async: enable  (必須)
Body: {"model": "qwen3-asr-flash-filetrans", "input": {"file_url": "https://..."}, "parameters": {"channel_id": [0]}}
Response: {"request_id": "...", "output": {"task_id": "...", "task_status": "PENDING"}}
```

### 查詢任務
```
GET {BaseURL}/api/v1/tasks/{task_id}
Headers: Authorization: Bearer {key}  (不需要 X-DashScope-Async)
Response: {"request_id": "...", "output": {"task_id": "...", "task_status": "SUCCEEDED", "result": {"transcription_url": "..."}}}
```

### 下載結果
GET transcription_url → 含時間戳的轉寫 JSON

### 轉寫結果結構
```json
{
  "file_url": "https://...",
  "audio_info": {"format": "mp3", "sample_rate": 22050},
  "transcripts": [
    {
      "channel_id": 0,
      "text": "完整識別文本...",
      "sentences": [
        {
          "sentence_id": 0,
          "begin_time": 0,
          "end_time": 1440,
          "language": "zh",
          "emotion": "neutral",
          "text": "句子文本",
          "words": [
            {"begin_time": 0, "end_time": 160, "text": "字", "punctuation": ""}
          ]
        }
      ]
    }
  ]
}
```

## MaaS Workspace 端點
如果使用 MaaS 私有端點（`{workspaceId}.ap-southeast-1.maas.aliyuncs.com`），所有 API 回應會包裹在 envelope 內：
```json
{"code": "", "message": "", "data": {標準 DashScope 回應}}
```
程式碼使用 `unwrapMaaSResponse()` 函數自動處理兩種格式。

## 任務清單
- [x] 更新 `dashscope.go`：重寫 `Transcribe()` 使用原生異步 API
- [x] 更新默認模型：`qwen3-asr-flash` → `qwen3-asr-flash-filetrans`
- [x] 移除 Dockerfile 的 ffmpeg
- [x] 更新 E2E 測試
- [x] 修正 `dashscopeFiletransTaskResponse` 結構對齊官方文檔
- [x] 支援 MaaS workspace 端點的 response envelope
- [x] 移除 poll GET 請求的 `X-DashScope-Async` header
- [x] `go build` + `go test` 驗證

## 受影響文件
- `pkg/dashscope.go`
- `pkg/dashscope_test.go`
- `Dockerfile`
