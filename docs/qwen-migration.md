# Qwen Migration: Gemini → DashScope (Qwen) Video Indexing Pipeline

## Goal

Replace the Gemini-based video indexing pipeline with DashScope (Qwen) APIs. The pipeline produces subtitles (via audio transcription) and summary+chapters (via video analysis) for each Wistia video, stored as `index-ai.json` and `subtitles.vtt` on S3 and BoltDB.

## Background

The current pipeline (`pkg/gemini.go` + `pkg/http.go:indexVideoToS3`) works in two steps:

1. **Audio transcription**: ffmpeg extracts WAV → upload to Gemini File API → Gemini audio transcription → subtitle entries (start/end/text)
2. **Video analysis**: Video URL → Gemini video understanding → JSON with summary + chapter entries

This is being replaced with:

1. **Audio transcription**: Video URL (mp4) → **Fun-ASR** (DashScope async API) → sentence-level subtitles with ms timestamps
2. **Video analysis**: Video URL → **Qwen3.5-Omni-Plus** (DashScope OpenAI-compatible streaming API) → summary + chapters

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
  → PASS VIDEO URL directly to Fun-ASR (no ffmpeg, no download!)
  → Fun-ASR submit task (async, returns task_id)
  → Poll task status until SUCCEEDED
  → Download transcription result JSON
  → convert Fun-ASR sentences → subtitle entries (ms → seconds)

  → PASS VIDEO URL to Qwen3.5-Omni-Plus (OpenAI-compatible, streaming)
  → Collect SSE stream chunks → concatenate full text
  → parse JSON → summary + chapters

  → merge → DashScopeIndexResult → upload index-ai.json + subtitles.vtt to S3
  → save to BoltDB "index" bucket
```

### Key Improvement: No More ffmpeg

Fun-ASR supports mp4/mkv/mov/avi/flv/wav/mp3 and many other formats directly. Our video files are already on S3 as public URLs. We can pass the video URL directly to Fun-ASR, eliminating:
- Video download to temp dir
- ffmpeg extraction
- Gemini file upload (resumable upload + polling)

This removes ~80% of the pipeline latency and the ffmpeg binary dependency.

## API Mapping

| Step | Current (Gemini) | New (DashScope) |
|------|-----------------|-----------------|
| **Audio input** | ffmpeg extract WAV → upload to Gemini File API | Pass video URL directly (Fun-ASR accepts mp4) |
| **Transcription API** | `POST generativelanguage.googleapis.com/v1beta/models/{model}:generateContent` | `POST dashscope-intl.aliyuncs.com/api/v1/services/audio/asr/transcription` |
| **Transcription mode** | Synchronous | Async (submit → poll → download) |
| **Transcription model** | gemini-2.5-flash-lite | fun-asr |
| **Video analysis API** | Same Gemini endpoint | `POST dashscope-intl.aliyuncs.com/compatible-mode/v1/chat/completions` |
| **Video analysis mode** | Synchronous | SSE streaming (mandatory) |
| **Video model** | gemini-2.5-flash-lite | qwen3.5-omni-plus |
| **Auth** | `?key=` query param | `Authorization: Bearer` header |
| **Timestamps** | Seconds (float), Gemini generates | Milliseconds (integer), Fun-ASR generates |
| **Language detection** | Gemini auto-detects | Explicit `language_hints: ["zh", "yue", "en"]` |
| **Response format** | Gemini JSON → parse text field for JSON | ASR: result JSON download; Video: SSE → concatenate → parse JSON |

## Config Changes

### Environment Variables

| Remove | Add |
|--------|-----|
| `GEMINI_API_KEY` | `DASHSCOPE_API_KEY` |
| `GEMINI_MODEL` | `DASHSCOPE_BASE_URL` (default: `https://dashscope-intl.aliyuncs.com`) |
| | `DASHSCOPE_ASR_MODEL` (default: `fun-asr`) |
| | `DASHSCOPE_VIDEO_MODEL` (default: `qwen3.5-omni-plus`) |

### Config Struct

```go
// Replace GeminiConf with:
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
    "asr_model": "fun-asr",
    "video_model": "qwen3.5-omni-plus"
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
        this.ASRModel = "fun-asr"
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

### DashScope Request/Response Types (new file: `pkg/dashscope.go`)

```go
// --- Fun-ASR (Transcription) ---

type dashscopeASRSubmitRequest struct {
    Model      string                `json:"model"`
    Input      dashscopeASRInput     `json:"input"`
    Parameters dashscopeASRParams    `json:"parameters,omitempty"`
}

type dashscopeASRInput struct {
    FileURLs []string `json:"file_urls"`
}

type dashscopeASRParams struct {
    ChannelID     []int    `json:"channel_id,omitempty"`
    LanguageHints []string `json:"language_hints,omitempty"`
}

type dashscopeASRSubmitResponse struct {
    RequestID string `json:"request_id"`
    Output    struct {
        TaskID     string `json:"task_id"`
        TaskStatus string `json:"task_status"`
    } `json:"output"`
}

type dashscopeTaskResponse struct {
    RequestID string `json:"request_id"`
    Output    struct {
        TaskID     string `json:"task_id"`
        TaskStatus string `json:"task_status"`
        Results    []struct {
            FileURL          string `json:"file_url"`
            TranscriptionURL string `json:"transcription_url"`
            SubtaskStatus    string `json:"subtask_status"`
        } `json:"results"`
        TaskMetrics struct {
            TOTAL     int `json:"TOTAL"`
            SUCCEEDED int `json:"SUCCEEDED"`
            FAILED    int `json:"FAILED"`
        } `json:"task_metrics"`
    } `json:"output"`
}

type dashscopeTranscriptionResult struct {
    FileURL    string `json:"file_url"`
    Properties struct {
        AudioFormat                       string `json:"audio_format"`
        Channels                          []int  `json:"channels"`
        OriginalSamplingRate              int    `json:"original_sampling_rate"`
        OriginalDurationInMilliseconds    int    `json:"original_duration_in_milliseconds"`
    } `json:"properties"`
    Transcripts []struct {
        ChannelID                     int    `json:"channel_id"`
        ContentDurationInMilliseconds int    `json:"content_duration_in_milliseconds"`
        Text                          string `json:"text"`
        Sentences                     []struct {
            BeginTime  int    `json:"begin_time"`
            EndTime    int    `json:"end_time"`
            Text       string `json:"text"`
            SentenceID int    `json:"sentence_id"`
            SpeakerID  int    `json:"speaker_id,omitempty"`
            Words      []struct {
                BeginTime   int    `json:"begin_time"`
                EndTime     int    `json:"end_time"`
                Text        string `json:"text"`
                Punctuation string `json:"punctuation"`
            } `json:"words"`
        } `json:"sentences"`
    } `json:"transcripts"`
}

// --- Qwen3.5-Omni-Plus (Video Analysis) ---

type dashscopeChatRequest struct {
    Model         string              `json:"model"`
    Messages      []dashscopeMessage  `json:"messages"`
    Stream        bool                `json:"stream"`
    StreamOptions struct {
        IncludeUsage bool `json:"include_usage"`
    } `json:"stream_options"`
    Modalities []string `json:"modalities"`
    MaxTokens  int      `json:"max_tokens,omitempty"`
}

type dashscopeMessage struct {
    Role    string               `json:"role"`
    Content []dashscopeContentPart `json:"content"`
}

type dashscopeContentPart struct {
    Type     string                  `json:"type"`
    Text     string                  `json:"text,omitempty"`
    VideoURL *dashscopeVideoURLValue `json:"video_url,omitempty"`
}

type dashscopeVideoURLValue struct {
    URL string `json:"url"`
}

type dashscopeStreamChunk struct {
    ID      string `json:"id"`
    Choices []struct {
        Delta struct {
            Content string `json:"content"`
        } `json:"delta"`
        FinishReason *string `json:"finish_reason"`
    } `json:"choices"`
    Usage *struct {
        PromptTokens     int `json:"prompt_tokens"`
        CompletionTokens int `json:"completion_tokens"`
        TotalTokens      int `json:"total_tokens"`
    } `json:"usage,omitempty"`
}
```

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

Qwen3.5-Omni-Plus **requires** `stream: true`. The response is Server-Sent Events (SSE):

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

## Async Transcription Flow (Fun-ASR)

Fun-ASR uses a 3-step async pattern that integrates well with our existing goroutine-based task model:

```
┌─────────────────────────────────────────────────────────┐
│ indexVideoToS3 goroutine                                │
│                                                         │
│  1. Submit ASR task                                     │
│     POST /api/v1/services/audio/asr/transcription       │
│     Headers: X-DashScope-Async: enable                  │
│     → returns task_id                                   │
│                                                         │
│  2. Poll until done (in same goroutine)                 │
│     GET /api/v1/tasks/{task_id}                         │
│     Loop with 3s sleep between polls                    │
│     → returns transcription_url when SUCCEEDED          │
│                                                         │
│  3. Download result                                     │
│     GET transcription_url                               │
│     → returns TranscriptionResult JSON                  │
│                                                         │
│  4. Convert sentences → subtitle entries                │
│     begin_time/end_time (ms) → start/end (seconds)     │
│                                                         │
│  (Meanwhile, video analysis runs concurrently or after) │
└─────────────────────────────────────────────────────────┘
```

### Polling Strategy

- Poll interval: 3 seconds (matches research doc example)
- Max poll duration: 10 minutes (timeout, matches our Gemini HTTP client timeout)
- DashScope polling rate limit: 20 QPS default (we're well under this with our 3-worker semaphore)
- Terminal states: `SUCCEEDED`, `FAILED`
- Non-terminal states: `PENDING`, `RUNNING`

### Timestamp Conversion

Fun-ASR returns timestamps in **milliseconds** (int). Our subtitle format uses **seconds** (float64):

```go
func msToSeconds(ms int) float64 {
    return float64(ms) / 1000.0
}
```

### Concurrency Opportunity

Since transcription no longer requires download+ffmpeg+upload, the two pipeline steps can potentially run concurrently:
- Step A: Fun-ASR transcription (async, takes ~10-30s)
- Step B: Qwen3.5-Omni-Plus video analysis (streaming, takes ~30-60s)

Both accept the same video URL as input. Running them in parallel with `sync.WaitGroup` could cut total indexing time roughly in half.

## Prompts

### Video Analysis Prompt (for Qwen3.5-Omni-Plus)

Reuse the same prompt as Gemini (`geminiVideoPrompt`), adapted for the new API:

```
Analyze this video and return ONLY a valid JSON object (no markdown, no explanation).

CRITICAL LANGUAGE RULE: Detect the primary spoken language in the video. You MUST write "summary" and all chapter "title" values in that SAME language. Do NOT use English if the video is spoken in another language.

The JSON must have:
1. "summary": A concise summary (2-4 sentences) based on BOTH audio and visual content, in the video's spoken language.
2. "chapters": Array of entries with "start" (float, seconds), "end" (float, seconds), "title" (descriptive title in the video's spoken language).
```

Note: Qwen3.5-Omni-Plus does not support `response_mime_type: "application/json"` like Gemini. We rely on the prompt to request JSON output and parse it. If the model occasionally returns non-JSON (markdown code blocks), we should strip ```json fences before parsing.

### Transcription

No prompt needed for Fun-ASR — it's a pure ASR model, not an LLM. Language is controlled via `language_hints` parameter.

## Error Handling

### Fun-ASR Errors

| Error | Cause | Handling |
|-------|-------|----------|
| HTTP 401 | Invalid API key | Fail task immediately, log error |
| HTTP 429 | Rate limit (20 QPS polling) | Retry with backoff |
| `task_status: FAILED` | Transcription failed | Check error details in task response, fail task |
| `transcription_url` download fails | URL expired (24h) or network error | Should not happen if we download immediately after SUCCEEDED |
| Empty `transcripts` | No speech detected | Return empty subtitles, continue pipeline |
| Unsupported format | File not in supported list | Should not happen (we pass mp4 URLs) |

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
| `pkg/http.go` | Replace `geminiHelper` usage in `indexVideoToS3` with `dashscopeHelper`. Remove ffmpeg/download/upload steps. Remove `os/exec` import if no longer needed. |
| `pkg/db.go` | Update `SaveVideoIndex` and `FindVideoIndex` to use `DashScopeIndexResult` instead of `GeminiIndexResult` |
| `.env.example` | Replace `GEMINI_API_KEY`/`GEMINI_MODEL` with `DASHSCOPE_API_KEY`/`DASHSCOPE_BASE_URL`/`DASHSCOPE_ASR_MODEL`/`DASHSCOPE_VIDEO_MODEL` |
| `docker-compose.yml` | Update env vars |
| `docker-compose.e2e.yml` | Update env vars |

### Files to Remove (after migration verified)

| File | Notes |
|------|-------|
| `pkg/gemini.go` | Keep until migration is verified, then remove |
| `pkg/gemini_test.go` | Keep until migration is verified, then remove |

### Files NOT Affected

| File | Why |
|------|-----|
| `pkg/wistia.go` | Wistia API integration unchanged |
| `pkg/storage_s3.go` | S3 upload logic unchanged |
| `pkg/log.go` | Logging unchanged |
| `web/*` | Frontend unchanged (reads from same API endpoints) |
| `webroot/*` | Static files unchanged |

## Task Breakdown

### Phase 1: Create DashScope client (`pkg/dashscope.go`)

- [ ] 1.1 Define `DashScopeConf` struct with `MarginWithENV()` method
- [ ] 1.2 Define `DashScopeHelper` struct and constructor
- [ ] 1.3 Implement Fun-ASR submit: `submitTranscription(fileURLs []string) (string, error)`
- [ ] 1.4 Implement Fun-ASR poll: `pollTask(taskID string) (*dashscopeTaskResponse, error)`
- [ ] 1.5 Implement Fun-ASR download: `downloadResult(url string) (*dashscopeTranscriptionResult, error)`
- [ ] 1.6 Implement Fun-ASR combined: `Transcribe(videoUrl string) (*DashScopeAudioTranscription, error)` — submit + poll + download + convert to subtitle entries
- [ ] 1.7 Define Qwen3.5-Omni-Plus request/response types
- [ ] 1.8 Implement SSE stream collector: `streamChat(req *dashscopeChatRequest) (string, *DashScopeTokenUsage, error)`
- [ ] 1.9 Implement `IndexVideo(videoUrl string) (string, *DashScopeTokenUsage, error)` using stream collector
- [ ] 1.10 Define `DashScopeIndexResult` and associated types (subtitle, chapter, token usage)
- [ ] 1.11 Implement `ToVTT()` method on `DashScopeIndexResult`
- [ ] 1.12 Implement `extractJSON()` helper for stripping markdown fences

### Phase 2: Wire into config and DB

- [ ] 2.1 Add `DashScopeConf *DashScopeConf` field to `Config` struct in `conf.go`
- [ ] 2.2 Update `Config.MarginWithENV()` to initialize `DashScopeConf`
- [ ] 2.3 Update `db.go` `SaveVideoIndex` parameter type from `*GeminiIndexResult` to `*DashScopeIndexResult`
- [ ] 2.4 Update `db.go` `FindVideoIndex` return type from `*GeminiIndexResult` to `*DashScopeIndexResult`

### Phase 3: Rewrite `indexVideoToS3` in `http.go`

- [ ] 3.1 Replace `NewGeminiHelper(s.config.GeminiConf)` with `NewDashScopeHelper(s.config.DashScopeConf)`
- [ ] 3.2 Remove video download to temp dir (lines 636-648)
- [ ] 3.3 Remove ffmpeg audio extraction (lines 651-667)
- [ ] 3.4 Remove Gemini file upload (lines 669-680)
- [ ] 3.5 Replace `geminiHelper.IndexAudio()` with `dashscopeHelper.Transcribe(videoUrl)` (pass video URL directly)
- [ ] 3.6 Replace `geminiHelper.IndexVideo()` with `dashscopeHelper.IndexVideo(videoUrl)`
- [ ] 3.7 Update result assembly: `GeminiIndexResult` → `DashScopeIndexResult`
- [ ] 3.8 Update token usage logging
- [ ] 3.9 Remove unused imports: `os/exec`, `math` (if no longer needed), `os` (if temp dir no longer needed)
- [ ] 3.10 (Optional) Run transcription and video analysis concurrently with `sync.WaitGroup`

### Phase 4: Update env and Docker config

- [ ] 4.1 Update `.env.example`: replace GEMINI vars with DASHSCOPE vars
- [ ] 4.2 Update `docker-compose.yml` env section
- [ ] 4.3 Update `docker-compose.e2e.yml` env section
- [ ] 4.4 Update `Dockerfile` if ffmpeg was installed (remove if no longer needed)

### Phase 5: Tests

- [ ] 5.1 Write `TestDashScopeConf_MarginWithENV` in `dashscope_test.go`
- [ ] 5.2 Write `TestDashScopeIndexResult_ToVTT` in `dashscope_test.go`
- [ ] 5.3 Write `TestDashScopeHelper_Transcribe` (integration, needs real API key)
- [ ] 5.4 Write `TestDashScopeHelper_IndexVideo` (integration, needs real API key)
- [ ] 5.5 Write `TestE2E_IndexVideoToS3_DashScope` (full pipeline integration test)
- [ ] 5.6 Verify existing `db_test.go` still passes with new type names

### Phase 6: Cleanup

- [ ] 6.1 Remove `pkg/gemini.go`
- [ ] 6.2 Remove `pkg/gemini_test.go`
- [ ] 6.3 Verify `docker build` succeeds
- [ ] 6.4 Verify `go test ./pkg/...` passes (with valid `.env`)
- [ ] 6.5 Verify `yarn build` in `web/` still works (should be unaffected)

## Cost Comparison

| Service | Gemini (current) | DashScope (new) |
|---------|-----------------|-----------------|
| **Transcription** | Included in Gemini API tokens | $0.000035/second of speech (~$0.126/hour) |
| **Video analysis** | Included in Gemini API tokens | Currently FREE (preview) |
| **File upload** | Gemini File API (free) | N/A (pass URL directly) |
| **Estimated per 10-min video** | ~$0.01-0.05 (Gemini tokens) | ~$0.021 (ASR only, video free) |
| **Free quota** | Gemini free tier | 36,000s ASR + 1M tokens video (90 days) |

Note: Gemini pricing depends on token usage which varies by model. DashScope ASR charges only for speech duration (silence excluded), which is typically 60-70% of wall-clock duration.

## Open Questions

1. **Backward compatibility with existing BoltDB data**: Existing `index` bucket entries are stored as `GeminiIndexResult` JSON. Since the JSON field names are identical, `json.Unmarshal` into `DashScopeIndexResult` should work. Should we add a migration script, or rely on JSON compatibility?

2. **Concurrent transcription + analysis**: Should we run Fun-ASR and Qwen video analysis in parallel (both accept the same video URL)? This would halve indexing time but complicate error handling.

3. **Video duration limit**: Qwen3.5-Omni-Plus has ~400s limit at 720p. Our videos may exceed this. Should we fall back to a lower resolution or skip video analysis for long videos?

4. **Gemini code retention**: Should we keep `gemini.go` as a fallback/option, or fully remove it? If keeping, we could add a config flag to choose provider.

5. **Region selection**: Singapore (`dashscope-intl`) is the default. Should we make this configurable for teams in other regions?
