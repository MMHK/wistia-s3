# Qwen Migration: Gemini → DashScope (Qwen) Video Indexing Pipeline

## Goal

Replace the Gemini-based video indexing pipeline with DashScope (Qwen) APIs. The pipeline produces subtitles (via audio transcription) and summary+chapters (via video analysis) for each Wistia video, stored as `index-ai.json` and `subtitles.vtt` on S3 and BoltDB.

## Status: ✅ Implemented

The migration is complete. See [dashscope-struct-fix.md](./dashscope-struct-fix.md) and [qwen-asr-filetrans-migration.md](./qwen-asr-filetrans-migration.md) for the final implementation details.

## Background

The current pipeline (`pkg/gemini.go` + `pkg/http.go:indexVideoToS3`) works in two steps:

1. **Audio transcription**: ffmpeg extracts WAV → upload to Gemini File API → Gemini audio transcription → subtitle entries (start/end/text)
2. **Video analysis**: Video URL → Gemini video understanding → JSON with summary + chapter entries

This was replaced with:

1. **Audio transcription**: Video URL (mp4) → **qwen3-asr-flash-filetrans** (DashScope async API) → sentence-level subtitles with ms timestamps
2. **Video analysis**: Video URL → **Qwen3.5-Omni-Flash** (DashScope OpenAI-compatible streaming API) → summary + chapters

**Note**: The original plan was to use `fun-asr` for transcription. The implementation uses `qwen3-asr-flash-filetrans` instead, which has a slightly different API structure (single `file_url` instead of `file_urls` array, `result.transcription_url` instead of `results[]`, etc.).

## Architecture

### Current Pipeline (Gemini)

```
Video on S3
  → download to temp dir
  → ffmpeg extract WAV to temp dir
  → upload WAV to Gemini File API (resumable upload, poll for ACTIVE)
  → Gemini audio transcription (sync, returns JSON)
  → parse GeminiAudioTranscription → subtitles

  → Gemini video analysis (sync, returns JSON)
  → parse GeminiVideoAnalysis → summary + chapters

  → merge → GeminiIndexResult → upload index-ai.json + subtitles.vtt to S3
  → save to BoltDB "index" bucket
```

### New Pipeline (DashScope)

```
Video on S3 (public URL already available)
  → PASS VIDEO URL directly to qwen3-asr-flash-filetrans (no ffmpeg, no download!)
  → Submit task (async, returns task_id)
  → Poll task status until SUCCEEDED
  → Download transcription result JSON
  → convert sentences → subtitle entries (ms → seconds)

  → PASS VIDEO URL to Qwen3.5-Omni-Flash (OpenAI-compatible, streaming)
  → Collect SSE stream chunks → concatenate full text
  → parse JSON → summary + chapters

  → merge → DashScopeIndexResult → upload index-ai.json + subtitles.vtt to S3
  → save to BoltDB "index" bucket
```

### Key Improvement: No More ffmpeg

qwen3-asr-flash-filetrans supports mp4/mkv/mov/avi/flv/wav/mp3 and many other formats directly. Our video files are already on S3 as public URLs. We can pass the video URL directly, eliminating:
- Video download to temp dir
- ffmpeg extraction
- Gemini file upload (resumable upload + polling)

This removes ~80% of the pipeline latency and the ffmpeg binary dependency.

## API Mapping

| Step | Current (Gemini) | New (DashScope) |
|------|-----------------|-----------------|
| **Audio input** | ffmpeg extract WAV → upload to Gemini File API | Pass video URL directly (qwen3-asr-flash-filetrans accepts mp4) |
| **Transcription API** | `POST generativelanguage.googleapis.com/v1beta/models/{model}:generateContent` | `POST {BaseURL}/api/v1/services/audio/asr/transcription` |
| **Transcription mode** | Synchronous | Async (submit → poll → download) |
| **Transcription model** | gemini-2.5-flash-lite | qwen3-asr-flash-filetrans |
| **Video analysis API** | Same Gemini endpoint | `POST {BaseURL}/compatible-mode/v1/chat/completions` |
| **Video analysis mode** | Synchronous | SSE streaming (mandatory) |
| **Video model** | gemini-2.5-flash-lite | qwen3.5-omni-flash |
| **Auth** | `?key=` query param | `Authorization: Bearer` header |
| **Timestamps** | Seconds (float), Gemini generates | Milliseconds (integer), qwen3-asr-flash-filetrans generates |
| **Language detection** | Gemini auto-detects | Auto-detect or explicit `language` param |
| **Response format** | Gemini JSON → parse text field for JSON | ASR: result JSON download; Video: SSE → concatenate → parse JSON |

## Config Changes

### Environment Variables

| Remove | Add |
|--------|-----|
| `GEMINI_API_KEY` | `DASHSCOPE_API_KEY` |
| `GEMINI_MODEL` | `DASHSCOPE_BASE_URL` (default: `https://dashscope-intl.aliyuncs.com`) |
| | `DASHSCOPE_ASR_MODEL` (default: `qwen3-asr-flash-filetrans`) |
| | `DASHSCOPE_VIDEO_MODEL` (default: `qwen3.5-omni-plus`) |

### Config Struct

```go
type DashScopeConf struct {
    ApiKey     string `json:"api_key"`
    BaseURL    string `json:"base_url"`
    ASRModel   string `json:"asr_model"`
    VideoModel string `json:"video_model"`
}
```

### Config JSON

```json
{
  "gemini": null,
  "dashscope": {
    "api_key": "sk-xxx",
    "base_url": "https://dashscope-intl.aliyuncs.com",
    "asr_model": "qwen3-asr-flash-filetrans",
    "video_model": "qwen3.5-omni-flash"
  }
}
```

### MarginWithENV Defaults

```go
func (this *DashScopeConf) MarginWithENV() {
    if this.ApiKey == "" {
        this.ApiKey = os.Getenv("DASHSCOPE_API_KEY")
    }
    if this.BaseURL == "" {
        this.BaseURL = os.Getenv("DASHSCOPE_BASE_URL")
    }
    if this.BaseURL == "" {
        this.BaseURL = "https://dashscope-intl.aliyuncs.com"
    }
    if this.ASRModel == "" {
        this.ASRModel = os.Getenv("DASHSCOPE_ASR_MODEL")
    }
    if this.ASRModel == "" {
        this.ASRModel = "qwen3-asr-flash-filetrans"
    }
    if this.VideoModel == "" {
        this.VideoModel = os.Getenv("DASHSCOPE_VIDEO_MODEL")
    }
    if this.VideoModel == "" {
        this.VideoModel = "qwen3.5-omni-plus"
    }
}
```

## New Structs and Types

### DashScope Request/Response Types (in `pkg/dashscope.go`)

The following are the actual types used in the implementation. See `pkg/dashscope.go` for the complete source.

**qwen3-asr-flash-filetrans (Transcription)**:
- `dashscopeFiletransRequest` — submit body (`model`, `input.file_url`, `parameters`)
- `dashscopeFiletransParameters` — `ChannelId`, `EnableItn`, `Language`
- `dashscopeFiletransInput` — `FileUrl`
- `dashscopeFiletransSubmitResponse` — submit response (`request_id`, `output.task_id`)
- `dashscopeFiletransTaskResponse` — poll response (`output.task_status`, `output.result.transcription_url`, `output.task_metrics`)
- `dashscopeFiletransResult` — transcription result (`file_url`, `audio_info`, `transcripts[]`)
- `dashscopeFiletransSentence` — sentence-level result (`begin_time`, `end_time`, `text`, `language`, `emotion`, `words[]`)
- `dashscopeFiletransWord` — word-level result (`begin_time`, `end_time`, `text`, `punctuation`)
- `dashscopeMaaSEnvelope` — MaaS response wrapper (`code`, `message`, `data`)
- `unwrapMaaSResponse()` — transparent MaaS/standard response handling

**Qwen3.5-Omni-Plus (Video Analysis)**:
- `dashscopeChatRequest` — video analysis request
- `dashscopeMessage`, `dashscopeContentPart`, `dashscopeVideoURLValue` — message types
- `dashscopeStreamChunk` — SSE streaming chunk

### Result Types (replaces `GeminiIndexResult`)

```go
// Reuse existing subtitle/chapter entry shapes for backward compat with S3 JSON:
//   DashScopeSubtitleEntry has same JSON shape as GeminiSubtitleEntry (start/end float, text string)
//   DashScopeChapterEntry has same JSON shape as GeminiChapterEntry (start/end float, title string)
// This ensures index-ai.json on S3 remains backward compatible.

type DashScopeSubtitleEntry struct {
    Start float64 `json:"start"`
    End   float64 `json:"end"`
    Text  string  `json:"text"`
}

type DashScopeChapterEntry struct {
    Start float64 `json:"start"`
    End   float64 `json:"end"`
    Title string  `json:"title"`
}

type DashScopeTokenUsage struct {
    InputK  float64 `json:"inputK"`
    OutputK float64 `json:"outputK"`
    TotalK  float64 `json:"totalK"`
}

type DashScopeAudioTranscription struct {
    Language  string                     `json:"language"`
    Subtitles []DashScopeSubtitleEntry   `json:"subtitles"`
}

type DashScopeVideoAnalysis struct {
    Summary  string                    `json:"summary"`
    Chapters []DashScopeChapterEntry   `json:"chapters"`
}

type DashScopeIndexResult struct {
    HashId      string                     `json:"hashId"`
    Model       string                     `json:"model"`
    Source      *WistiaRespVideoAsset      `json:"source"`
    GeneratedAt string                     `json:"generatedAt"`
    Summary     string                     `json:"summary"`
    Subtitles   []DashScopeSubtitleEntry   `json:"subtitles"`
    Chapters    []DashScopeChapterEntry    `json:"chapters"`
    TokenUsage  *DashScopeTokenUsage       `json:"tokenUsage,omitempty"`
}
```

### Backward Compatibility Note

The JSON field names (`hashId`, `model`, `source`, `generatedAt`, `summary`, `subtitles`, `chapters`, `tokenUsage`) are identical between `GeminiIndexResult` and `DashScopeIndexResult`. Existing `index-ai.json` files on S3 and BoltDB entries can be read by either type. The `ToVTT()` method and `formatVTTTime()` function are reusable.

However, `db.go` references `GeminiIndexResult` directly in `SaveVideoIndex` and `FindVideoIndex`. These must be updated to use `DashScopeIndexResult`, or we define a shared interface/type alias.

**Recommended approach**: Rename `GeminiIndexResult` → `DashScopeIndexResult` (and associated types) everywhere. Since the JSON shape is identical, existing data in BoltDB and S3 is unaffected.

## SSE Streaming Handling

Qwen3.5-Omni series **requires** `stream: true`. The response is Server-Sent Events (SSE):

```
data: {"id":"...","choices":[{"delta":{"content":"The video..."},"finish_reason":null}]}
data: {"id":"...","choices":[{"delta":{"content":"shows a..."},"finish_reason":null}]}
...
data: {"id":"...","choices":[{"delta":{},"finish_reason":"stop"}],"usage":{...}}
data: [DONE]
```

### Implementation in Go

```go
func (this *DashScopeHelper) IndexVideo(videoUrl string) (string, *DashScopeTokenUsage, error) {
    // ... build request, set stream: true ...

    resp, err := client.Do(req)
    // ... error handling ...
    defer resp.Body.Close()

    var fullText strings.Builder
    var usage *DashScopeTokenUsage
    scanner := bufio.NewScanner(resp.Body)
    for scanner.Scan() {
        line := scanner.Text()
        if !strings.HasPrefix(line, "data: ") {
            continue
        }
        data := strings.TrimPrefix(line, "data: ")
        if data == "[DONE]" {
            break
        }

        var chunk dashscopeStreamChunk
        if err := json.Unmarshal([]byte(data), &chunk); err != nil {
            continue
        }
        for _, c := range chunk.Choices {
            fullText.WriteString(c.Delta.Content)
        }
        if chunk.Usage != nil {
            usage = &DashScopeTokenUsage{
                InputK:  math.Round(float64(chunk.Usage.PromptTokens)/10) / 100,
                OutputK: math.Round(float64(chunk.Usage.CompletionTokens)/10) / 100,
                TotalK:  math.Round(float64(chunk.Usage.TotalTokens)/10) / 100,
            }
        }
    }
    return fullText.String(), usage, nil
}
```

Key points:
- Use `bufio.Scanner` to read line-by-line from `resp.Body`
- Skip non-`data:` lines (blank lines, comments)
- Stop on `data: [DONE]`
- Accumulate `delta.content` from each chunk
- The last chunk with `usage` field contains token counts

## Async Transcription Flow (qwen3-asr-flash-filetrans)

qwen3-asr-flash-filetrans uses a 3-step async pattern that integrates well with our existing goroutine-based task model:

```
┌─────────────────────────────────────────────────────────┐
│ indexVideoToS3 goroutine                                │
│                                                         │
│  1. Submit ASR task                                     │
│     POST /api/v1/services/audio/asr/transcription       │
│     Headers: X-DashScope-Async: enable  (必須)          │
│     Body: {"model": "qwen3-asr-flash-filetrans",        │
│            "input": {"file_url": "..."},                │
│            "parameters": {"channel_id": [0]}}           │
│     → returns task_id                                   │
│                                                         │
│  2. Poll until done (in same goroutine)                 │
│     GET /api/v1/tasks/{task_id}                         │
│     Headers: Authorization: Bearer {key}  (不需要       │
│              X-DashScope-Async)                          │
│     Loop with 3s sleep between polls                    │
│     → returns output.result.transcription_url           │
│       when SUCCEEDED                                     │
│                                                         │
│  3. Download result                                     │
│     GET transcription_url  (valid 24h)                  │
│     → returns TranscriptionResult JSON                  │
│       with file_url, audio_info, transcripts[]          │
│                                                         │
│  4. Convert sentences → subtitle entries                │
│     begin_time/end_time (ms) → start/end (seconds)     │
│                                                         │
│  (Meanwhile, video analysis runs concurrently or after) │
└─────────────────────────────────────────────────────────┘
```

### MaaS Workspace Compatibility

If `DASHSCOPE_BASE_URL` points to a MaaS workspace endpoint (e.g., `{workspaceId}.ap-southeast-1.maas.aliyuncs.com`), responses are wrapped in an envelope:
```json
{"code": "", "message": "", "data": {標準 DashScope 回應}}
```
The code uses `unwrapMaaSResponse()` to handle both formats transparently.

### Polling Strategy

- Poll interval: 3 seconds
- Max poll duration: 10 minutes (timeout from http.Client)
- DashScope polling rate limit: 20 QPS default (we're well under this with our 3-worker semaphore)
- Terminal states: `SUCCEEDED`, `FAILED`
- Non-terminal states: `PENDING`, `RUNNING`

### Transcription Result Structure (qwen3-asr-flash-filetrans)

```json
{
  "file_url": "https://example.com/video.mp4",
  "audio_info": {"format": "mp4", "sample_rate": 44100},
  "transcripts": [
    {
      "channel_id": 0,
      "text": "Full text...",
      "sentences": [
        {
          "sentence_id": 0,
          "begin_time": 0,
          "end_time": 1440,
          "language": "zh",
          "emotion": "neutral",
          "text": "Sentence text.",
          "words": [
            {"begin_time": 0, "end_time": 160, "text": "字", "punctuation": ""}
          ]
        }
      ]
    }
  ]
}
```

### Timestamp Conversion

Timestamps are in **milliseconds** (int). Our subtitle format uses **seconds** (float64):

```go
Start: float64(sentence.BeginTime) / 1000.0
End:   float64(sentence.EndTime) / 1000.0
```

### Concurrency Opportunity

Since transcription no longer requires download+ffmpeg+upload, the two pipeline steps can potentially run concurrently:
- Step A: qwen3-asr-flash-filetrans transcription (async, takes ~10-30s)
- Step B: Qwen3.5-Omni-Plus video analysis (streaming, takes ~30-60s)

Both accept the same video URL as input. Running them in parallel with `sync.WaitGroup` could cut total indexing time roughly in half.

## Prompts

### Video Analysis Prompt (for Qwen3.5-Omni series)

The actual prompt used in `buildVideoPrompt()` (see `pkg/dashscope.go`):

```
Analyze this video and return ONLY a valid JSON object (no markdown, no explanation).

CRITICAL LANGUAGE RULE: You MUST write ALL text output in 繁體中文 (Traditional Chinese).

Use BOTH the video visual content AND the provided subtitle transcript to produce accurate results.

The JSON must have:
1. "summary": A concise summary (2-4 sentences) in 繁體中文.
2. "chapters": Array of entries with "start" (float, seconds), "end" (float, seconds), "title" (descriptive title in 繁體中文).
```

Note: The Qwen model does not support `response_mime_type: "application/json"`. We rely on the prompt to request JSON output and parse it. The `extractJSON()` helper strips markdown fences before parsing.

### Transcription

No prompt needed for qwen3-asr-flash-filetrans — it's a pure ASR model, not an LLM. Language is auto-detected or can be controlled via `language` parameter.

## Error Handling

### Transcription Errors (qwen3-asr-flash-filetrans)

| Error | Cause | Handling |
|-------|-------|----------|
| HTTP 401 | Invalid API key | Fail task immediately, log error |
| HTTP 429 | Rate limit (20 QPS polling) | Retry with backoff |
| `task_status: FAILED` | Transcription failed | Check `output.code` and `output.message` for details, fail task |
| `transcription_url` download fails | URL expired (24h) or network error | Should not happen if we download immediately after SUCCEEDED |
| Empty `transcripts` | No speech detected | Return empty subtitles, continue pipeline |
| MaaS error response | `{code: "xxx", message: "xxx"}` | `unwrapMaaSResponse()` returns error, task fails |

### Qwen3.5-Omni-Plus Errors

| Error | Cause | Handling |
|-------|-------|----------|
| HTTP 401 | Invalid API key | Fail task immediately |
| HTTP 400 | `stream: false` or bad request | Fail with descriptive error |
| HTTP 429 | Rate limit (60 RPM) | Retry with backoff (our semaphore limits concurrency) |
| SSE stream disconnects mid-response | Network issue | Check if we got partial text; retry entire request |
| Non-JSON response | Model didn't follow prompt | Strip markdown fences, attempt parse; if fails, try next resolution |
| Empty content in all chunks | Model returned nothing | Fail with "empty response from qwen" |
| Video too long (>400s at 720p) | Context limit | Try lower resolution video, or log warning |

### JSON Parsing Robustness

Qwen3.5-Omni-Plus may wrap JSON in markdown code fences. Implement a helper:

```go
func extractJSON(raw string) string {
    raw = strings.TrimSpace(raw)
    if strings.HasPrefix(raw, "```json") {
        raw = strings.TrimPrefix(raw, "```json")
        raw = strings.TrimSuffix(raw, "```")
    } else if strings.HasPrefix(raw, "```") {
        raw = strings.TrimPrefix(raw, "```")
        raw = strings.TrimSuffix(raw, "```")
    }
    return strings.TrimSpace(raw)
}
```

## Affected Files

### New Files

| File | Purpose |
|------|---------|
| `pkg/dashscope.go` | DashScopeHelper, all request/response types, ASR + video methods |
| `pkg/dashscope_test.go` | Integration tests for DashScope APIs |

### Modified Files

| File | Changes |
|------|---------|
| `pkg/conf.go` | Add `DashScopeConf` field to `Config`, update `MarginWithENV` |
| `pkg/http.go` | Replace Gemini helper with DashScope helper in `indexVideoToS3`. Remove ffmpeg/download/upload steps. |
| `pkg/db.go` | Update `SaveVideoIndex` and `FindVideoIndex` to use `DashScopeIndexResult` |
| `.env.example` | Replace `GEMINI_API_KEY`/`GEMINI_MODEL` with `DASHSCOPE_API_KEY`/`DASHSCOPE_BASE_URL`/`DASHSCOPE_ASR_MODEL`/`DASHSCOPE_VIDEO_MODEL` |
| `docker-compose.yml` | Update env vars |
| `docker-compose.e2e.yml` | Update env vars |

### Files Removed

| File | Notes |
|------|-------|
| `pkg/gemini.go` | ✅ Removed after migration verified |
| `pkg/gemini_test.go` | ✅ Removed after migration verified |

## Task Breakdown

### Phase 1: Create DashScope client (`pkg/dashscope.go`) — ✅ DONE

- [x] 1.1 Define `DashScopeConf` struct with `MarginWithENV()` method
- [x] 1.2 Define `DashScopeHelper` struct and constructor
- [x] 1.3 Implement qwen3-asr-flash-filetrans submit + poll + download
- [x] 1.4 Implement `Transcribe(videoUrl string) (*DashScopeAudioTranscription, error)`
- [x] 1.5 Define Qwen3.5-Omni request/response types
- [x] 1.6 Implement SSE stream collector
- [x] 1.7 Implement `IndexVideo(videoUrl, subtitles) (string, *DashScopeTokenUsage, error)`
- [x] 1.8 Define `DashScopeIndexResult` and associated types
- [x] 1.9 Implement `ToVTT()` method
- [x] 1.10 Implement `extractJSON()` helper
- [x] 1.11 Add MaaS envelope support (`unwrapMaaSResponse()`)

### Phase 2: Wire into config and DB — ✅ DONE

- [x] 2.1 Add `DashScopeConf *DashScopeConf` field to `Config` struct
- [x] 2.2 Update `Config.MarginWithENV()` to initialize `DashScopeConf`
- [x] 2.3 Update `db.go` `SaveVideoIndex` to use `DashScopeIndexResult`
- [x] 2.4 Update `db.go` `FindVideoIndex` to use `DashScopeIndexResult`

### Phase 3: Rewrite `indexVideoToS3` in `http.go` — ✅ DONE

- [x] 3.1 Replace Gemini helper with DashScope helper
- [x] 3.2 Remove video download, ffmpeg extraction, Gemini file upload
- [x] 3.3 Replace transcription with `dashscopeHelper.Transcribe(videoUrl)`
- [x] 3.4 Replace video analysis with `dashscopeHelper.IndexVideo(videoUrl, subtitles)`
- [x] 3.5 Update result assembly to use `DashScopeIndexResult`

### Phase 4: Update env and Docker config — ✅ DONE

- [x] 4.1 Update `.env.example`
- [x] 4.2 Update `docker-compose.yml`
- [x] 4.3 Remove ffmpeg dependency from Dockerfile

### Phase 5: Tests — ✅ DONE

- [x] 5.1 `TestDashScopeConf_MarginWithENV`
- [x] 5.2 `TestDashScopeFormatVTTTime`
- [x] 5.3 `TestDashScopeIndexResult_ToVTT`
- [x] 5.4 `TestDashScopeIndexResult_JSON`
- [x] 5.5 `TestE2E_IndexVideoToS3_DashScope`
- [x] 5.6 `TestDBHelper_SaveDashScopeVideoIndex`

### Phase 6: Cleanup — ✅ DONE

- [x] 6.1 Remove `pkg/gemini.go`
- [x] 6.2 Remove `pkg/gemini_test.go`
- [x] 6.3 `docker build` succeeds
- [x] 6.4 `go test` passes

## Cost Comparison

| Service | Gemini (old) | DashScope (current) |
|---------|-----------------|-----------------|
| **Transcription** | Included in Gemini API tokens | qwen3-asr-flash-filetrans: ~$0.000035/s (~$0.126/h) |
| **Video analysis** | Included in Gemini API tokens | qwen3.5-omni-flash: token-based |
| **File upload** | Gemini File API (free) | N/A (pass URL directly) |
| **Estimated per 10-min video** | ~$0.01-0.05 (Gemini tokens) | ~$0.021 (ASR + video tokens) |
| **Free quota** | Gemini free tier | 36,000s ASR + 1M tokens video (90 days) |

Note: DashScope ASR charges only for speech duration (silence excluded), which is typically 60-70% of wall-clock duration.

## Open Questions

1. **Backward compatibility with existing BoltDB data**: ✅ Resolved — JSON field names are identical between `GeminiIndexResult` and `DashScopeIndexResult`, so existing data is compatible.

2. **Concurrent transcription + analysis**: Not yet implemented. The two steps run sequentially. Running them in parallel could halve indexing time.

3. **Video duration limit**: Qwen3.5-Omni-Plus has ~400s limit at 720p. The code tries multiple resolutions from smallest to largest, so it can fall back to lower resolutions for longer videos.

4. **Gemini code retention**: ✅ Removed — `pkg/gemini.go` and `pkg/gemini_test.go` have been removed.

5. **Region selection**: ✅ Configurable via `DASHSCOPE_BASE_URL` env var. Supports standard DashScope endpoints and MaaS workspace endpoints.

6. **MaaS workspace support**: ✅ Implemented — `unwrapMaaSResponse()` transparently handles both standard and MaaS response formats.
