# DashScope API Research Report

## Executive Summary

DashScope (Alibaba Cloud Model Studio) provides both transcription and video understanding APIs that can replace the Gemini pipeline. A single API key works for all services within a region. Key caveat: **Paraformer is China-only**; for international (Singapore) use, substitute with **Fun-ASR** which has the same API interface.

**Important**: This project uses **qwen3-asr-flash-filetrans** for transcription (not Fun-ASR). It has a slightly different API structure — see Section 2B below.

---

## 1. General DashScope Configuration

### Base URLs by Region

| Region | Domain | OpenAI-compatible Base | DashScope Base |
|--------|--------|------------------------|----------------|
| Singapore (International) | `dashscope-intl.aliyuncs.com` | `https://dashscope-intl.aliyuncs.com/compatible-mode/v1` | `https://dashscope-intl.aliyuncs.com/api/v1` |
| Beijing (China Mainland) | `dashscope.aliyuncs.com` | `https://dashscope.aliyuncs.com/compatible-mode/v1` | `https://dashscope.aliyuncs.com/api/v1` |
| US (Virginia) | `dashscope-us.aliyuncs.com` | `https://dashscope-us.aliyuncs.com/compatible-mode/v1` | `https://dashscope-us.aliyuncs.com/api/v1` |

### MaaS Workspace Endpoints

Private workspace endpoints use the format:
```
https://{workspaceId}.ap-southeast-1.maas.aliyuncs.com
```

**Response format difference**: MaaS endpoints wrap ALL responses in an envelope:
```json
{"code": "", "message": "", "data": {標準 DashScope 回應}}
```

The `data` field contains the standard DashScope response body. Success is indicated by `code: ""` (empty string).

Code handles this transparently via `unwrapMaaSResponse()` — see `pkg/dashscope.go:183`.

### Authentication

- **Method**: Bearer token in `Authorization` header
- **Header**: `Authorization: Bearer sk-xxxxxxxxxxxxxxxx`
- **No** `?key=` parameter needed
- API keys are **region-specific** (Singapore key won't work in Beijing)

### How to Get an API Key

- Singapore: https://bailian.console.alibabacloud.com → Key Management
- Beijing: https://bailian.console.aliyun.com → Key Management

### Rate Limits (Qwen3.5-Omni series)

| Model | RPM (requests/min) | TPM (tokens/min) |
|-------|-------------------|------------------|
| qwen3.5-omni-plus | 60 | 100,000 |
| qwen3.5-omni-flash | 60 | 100,000 |

### Single Key for All Services

**Yes.** One DashScope API key (for a given region) works for:
- Transcription (qwen3-asr-flash-filetrans / Fun-ASR / Paraformer)
- Video understanding (Qwen3.5-Omni-Plus)
- Text generation (Qwen)
- All other Model Studio services

---

## 2. File Transcription API (Fun-ASR / Paraformer)

### CRITICAL: Region Availability

| Model | International (Singapore) | China Mainland (Beijing) |
|-------|--------------------------|--------------------------|
| **fun-asr** | Available | Available |
| **fun-asr-mtl** | Available | Available |
| **paraformer-v2** | NOT AVAILABLE | Available only |
| **paraformer-8k-v2** | NOT AVAILABLE | Available only |

**Recommendation**: Use `fun-asr` for international deployments. It has the same async REST API pattern, supports timestamps, and has Cantonese support.

### API Flow (3-step async)

1. **Submit** transcription task → get `task_id`
2. **Poll** task status until `SUCCEEDED`
3. **Download** result JSON from `transcription_url` (valid 24h)

### Step 1: Submit Task

**Endpoint (Singapore)**:
```
POST https://dashscope-intl.aliyuncs.com/api/v1/services/audio/asr/transcription
```

**Endpoint (Beijing)**:
```
POST https://dashscope.aliyuncs.com/api/v1/services/audio/asr/transcription
```

**Headers**:
```
Authorization: Bearer sk-xxx
Content-Type: application/json
X-DashScope-Async: enable          # REQUIRED - without this, request fails
```

**Request Body (Fun-ASR)**:
```json
{
  "model": "fun-asr",
  "input": {
    "file_urls": [
      "https://example.com/audio.mp3"
    ]
  },
  "parameters": {
    "channel_id": [0],
    "language_hints": ["zh", "yue", "en"]
  }
}
```

**Request Body (Paraformer-v2, Beijing only)**:
```json
{
  "model": "paraformer-v2",
  "input": {
    "file_urls": [
      "https://example.com/audio.mp3"
    ]
  },
  "parameters": {
    "channel_id": [0],
    "language_hints": ["zh", "yue", "en"],
    "timestamp_alignment_enabled": true,
    "diarization_enabled": false,
    "disfluency_removal_enabled": false
  }
}
```

**Submit Response**:
```json
{
  "request_id": "xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx",
  "output": {
    "task_id": "xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx",
    "task_status": "PENDING"
  }
}
```

### Step 2: Poll Task Status

**Endpoint (Singapore)**:
```
GET https://dashscope-intl.aliyuncs.com/api/v1/tasks/{task_id}
```

**Headers**:
```
Authorization: Bearer sk-xxx
X-DashScope-Async: enable
Content-Type: application/json
```

**Poll Response (completed)**:
```json
{
  "request_id": "xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx",
  "output": {
    "task_id": "xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx",
    "task_status": "SUCCEEDED",
    "submit_time": "2024-12-16 16:30:59.170",
    "scheduled_time": "2024-12-16 16:30:59.204",
    "end_time": "2024-12-16 16:31:02.375",
    "results": [
      {
        "file_url": "https://example.com/audio.mp3",
        "transcription_url": "https://dashscope-result-bj.oss-cn-beijing.aliyuncs.com/xxx/result.json",
        "subtask_status": "SUCCEEDED"
      }
    ],
    "task_metrics": {
      "TOTAL": 1,
      "SUCCEEDED": 1,
      "FAILED": 0
    }
  }
}
```

### Step 3: Download Transcription Result

GET the `transcription_url` (valid for 24 hours). The result is a JSON file:

```json
{
  "file_url": "https://example.com/audio.mp3",
  "properties": {
    "audio_format": "pcm_s16le",
    "channels": [0],
    "original_sampling_rate": 16000,
    "original_duration_in_milliseconds": 3834
  },
  "transcripts": [
    {
      "channel_id": 0,
      "content_duration_in_milliseconds": 3720,
      "text": "Full transcription text here...",
      "sentences": [
        {
          "begin_time": 100,
          "end_time": 3820,
          "text": "Hello world, this is a test.",
          "sentence_id": 1,
          "speaker_id": 0,
          "words": [
            {
              "begin_time": 100,
              "end_time": 596,
              "text": "Hello ",
              "punctuation": ""
            },
            {
              "begin_time": 596,
              "end_time": 844,
              "text": "world",
              "punctuation": ", "
            }
          ]
        }
      ]
    }
  ]
}
```

### Timestamp Format

Timestamps are in **milliseconds** (integer) at two levels:

| Level | Fields | Description |
|-------|--------|-------------|
| **Sentence** | `sentences[].begin_time`, `sentences[].end_time` | Start/end of each sentence in ms |
| **Word** | `sentences[].words[].begin_time`, `sentences[].words[].end_time` | Start/end of each word in ms |

- Paraformer: timestamps OFF by default; set `timestamp_alignment_enabled: true` to enable
- Fun-ASR: timestamps always ON, cannot be disabled
- Word-level timestamps availability varies by language (accuracy guaranteed for: zh, en, ja, ko, de, fr, es, it, pt, ru)

### Cantonese Support

**Yes!** Language code: `yue`

Supported in both Fun-ASR and Paraformer-v2. Add to `language_hints`:
```json
"language_hints": ["zh", "yue", "en"]
```

Note: This is Cantonese as a spoken language (no standard written form). The output will be in the language's script. For Paraformer-v2, Cantonese is explicitly listed among 18 supported Chinese dialects.

### File Requirements

| Constraint | Limit |
|------------|-------|
| Max file size | 2 GB |
| Max duration | 12 hours |
| Max URLs per request | 100 |
| Supported formats | aac, amr, avi, flac, flv, m4a, mkv, mov, mp3, mp4, mpeg, ogg, opus, wav, webm, wma, wmv |
| Audio input types | HTTP/HTTPS URLs (files must be publicly accessible) |
| Base64 input | NOT supported for file transcription (only for real-time) |

### Billing

**Charged by speech duration only** (silence/non-speech excluded).

| Model | Region | Price |
|-------|--------|-------|
| fun-asr | International (Singapore) | $0.000035/second (~$0.126/hour) |
| fun-asr | China Mainland (Beijing) | $0.000032/second (~$0.115/hour) |
| paraformer-v2 | China Mainland only | Check Beijing pricing page |
| paraformer-v2 | International | NOT AVAILABLE |

Free quota (International): 36,000 seconds (10 hours), valid 90 days.

### Polling Rate Limits

Task status query: default 20 QPS, max 100 QPS. For high-concurrency, configure async callback instead of polling.

---

## 3. Qwen3.5-Omni-Plus API (Video Understanding)

### Supports OpenAI-Compatible Format

**Yes!** Qwen3.5-Omni-Plus supports the OpenAI Chat Completions API format. This is the recommended approach.

### API Endpoint

**OpenAI-compatible (Singapore)**:
```
POST https://dashscope-intl.aliyuncs.com/compatible-mode/v1/chat/completions
```

**OpenAI-compatible (Beijing)**:
```
POST https://dashscope.aliyuncs.com/compatible-mode/v1/chat/completions
```

**DashScope native (Singapore)**:
```
POST https://dashscope-intl.aliyuncs.com/api/v1/services/aigc/multimodal-generation/generation
```

### Authentication

Same Bearer token:
```
Authorization: Bearer sk-xxx
Content-Type: application/json
```

### Video Input via Public HTTPS URL

**Yes!** Supports public HTTPS URLs directly (like Gemini's `file_uri`). Also supports:
- Public URL (HTTPS)
- Base64 encoding (`data:;base64,...`)
- Local file upload via DashScope file API

### Request Format (OpenAI-Compatible, Streaming Required)

**CRITICAL**: `stream` MUST be set to `true`, otherwise the API returns an error.

```json
{
  "model": "qwen3.5-omni-plus",
  "messages": [
    {
      "role": "user",
      "content": [
        {
          "type": "video_url",
          "video_url": {
            "url": "https://example.com/video.mp4"
          }
        },
        {
          "type": "text",
          "text": "Describe what happens in this video."
        }
      ]
    }
  ],
  "stream": true,
  "stream_options": {
    "include_usage": true
  },
  "modalities": ["text"],
  "max_tokens": 4096
}
```

**With audio output** (text + speech):
```json
{
  "model": "qwen3.5-omni-plus",
  "messages": [...],
  "stream": true,
  "stream_options": {"include_usage": true},
  "modalities": ["text", "audio"],
  "audio": {"voice": "Tina", "format": "wav"}
}
```

### cURL Example (text-only output)

```bash
curl -X POST "https://dashscope-intl.aliyuncs.com/compatible-mode/v1/chat/completions" \
  -H "Authorization: Bearer $DASHSCOPE_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "qwen3.5-omni-plus",
    "messages": [
      {
        "role": "user",
        "content": [
          {"type": "video_url", "video_url": {"url": "https://example.com/video.mp4"}},
          {"type": "text", "text": "What is the video about?"}
        ]
      }
    ],
    "stream": true,
    "stream_options": {"include_usage": true},
    "modalities": ["text"]
  }'
```

### Response Format (SSE streaming)

Standard OpenAI SSE streaming chunks:
```
data: {"id":"chatcmpl-xxx","object":"chat.completion.chunk","created":1234567890,"model":"qwen3.5-omni-plus","choices":[{"index":0,"delta":{"role":"assistant","content":"The video shows..."},"finish_reason":null}]}

data: {"id":"chatcmpl-xxx","object":"chat.completion.chunk","created":1234567890,"model":"qwen3.5-omni-plus","choices":[{"index":0,"delta":{"content":"a beautiful mountain"},"finish_reason":null}]}

...

data: {"id":"chatcmpl-xxx","object":"chat.completion.chunk","created":1234567890,"model":"qwen3.5-omni-plus","choices":[{"index":0,"delta":{},"finish_reason":"stop"}],"usage":{"prompt_tokens":12345,"completion_tokens":100,"total_tokens":12445}}

data: [DONE]
```

### Video Limits

| Constraint | Limit |
|------------|-------|
| Max context | 256K tokens |
| Video duration | ~400 seconds of 720p video at 1 FPS (fills 256K context) |
| Audio duration | ~10 hours of continuous audio |
| Max images per request | 2,048 (public URL) / 250 (base64) |
| Image file size | ≤20 MB (Qwen3.5 series) |
| Supported video formats | Via URL: mp4, mov, etc. (any format the model can decode) |
| Frame rate | Default 2 FPS, configurable via `fps` parameter (0.1-10) |
| Max frames (Qwen3.5) | 8,000 frames |

### Pricing

**Currently in preview — model invocation is temporarily FREE** (as of the research date).

After preview:

| Model | Input (Text/Image/Video) | Input (Audio) | Output (Text) |
|-------|-------------------------|---------------|---------------|
| qwen3.5-omni-plus | TBD (tiered by input size) | TBD | TBD |

Free quota (International): 1M input + 1M output tokens, valid 90 days.

For reference, qwen3.5-plus (text) pricing:
- 0-256K input tokens: $0.40/1M input, $2.40/1M output
- 256K-1M input tokens: $0.50/1M input, $3.00/1M output

### Rate Limits

| Model | RPM | TPM |
|-------|-----|-----|
| qwen3.5-omni-plus | 60 | 100,000 |

### DashScope Native Format (Non-OpenAI)

If you prefer the native DashScope format:

```json
{
  "model": "qwen3.5-omni-plus",
  "input": {
    "messages": [
      {
        "role": "user",
        "content": [
          {"video": "https://example.com/video.mp4"},
          {"text": "Describe this video."}
        ]
      }
    ]
  },
  "parameters": {
    "result_format": "message"
  }
}
```

**Endpoint**: `POST https://dashscope-intl.aliyuncs.com/api/v1/services/aigc/multimodal-generation/generation`

---

## 4. Go HTTP Client Examples

### 4.1 File Transcription (Fun-ASR, Async)

```go
package main

import (
    "encoding/json"
    "fmt"
    "io"
    "net/http"
    "time"
)

const (
    apiKey = "sk-xxx"
    // Singapore region
    submitURL = "https://dashscope-intl.aliyuncs.com/api/v1/services/audio/asr/transcription"
    queryBase = "https://dashscope-intl.aliyuncs.com/api/v1/tasks/"
)

type SubmitRequest struct {
    Model string `json:"model"`
    Input struct {
        FileURLs []string `json:"file_urls"`
    } `json:"input"`
    Parameters struct {
        ChannelID    []int    `json:"channel_id,omitempty"`
        LanguageHints []string `json:"language_hints,omitempty"`
    } `json:"parameters,omitempty"`
}

type SubmitResponse struct {
    RequestID string `json:"request_id"`
    Output    struct {
        TaskID     string `json:"task_id"`
        TaskStatus string `json:"task_status"`
    } `json:"output"`
}

type TaskResponse struct {
    RequestID string `json:"request_id"`
    Output    struct {
        TaskID     string `json:"task_id"`
        TaskStatus string `json:"task_status"`
        Results    []struct {
            FileURL          string `json:"file_url"`
            TranscriptionURL string `json:"transcription_url"`
            SubtaskStatus    string `json:"subtask_status"`
        } `json:"results"`
    } `json:"output"`
}

type TranscriptionResult struct {
    FileURL    string `json:"file_url"`
    Properties struct {
        AudioFormat                   string  `json:"audio_format"`
        Channels                      []int   `json:"channels"`
        OriginalSamplingRate          int     `json:"original_sampling_rate"`
        OriginalDurationInMilliseconds int    `json:"original_duration_in_milliseconds"`
    } `json:"properties"`
    Transcripts []struct {
        ChannelID                       int    `json:"channel_id"`
        ContentDurationInMilliseconds   int    `json:"content_duration_in_milliseconds"`
        Text                            string `json:"text"`
        Sentences                       []struct {
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

func submitTranscription(fileURLs []string) (string, error) {
    reqBody := SubmitRequest{
        Model: "fun-asr",
    }
    reqBody.Input.FileURLs = fileURLs
    reqBody.Parameters.ChannelID = []int{0}
    reqBody.Parameters.LanguageHints = []string{"zh", "yue", "en"}

    body, _ := json.Marshal(reqBody)
    req, _ := http.NewRequest("POST", submitURL, bytes.NewReader(body))
    req.Header.Set("Authorization", "Bearer "+apiKey)
    req.Header.Set("Content-Type", "application/json")
    req.Header.Set("X-DashScope-Async", "enable")

    resp, err := http.DefaultClient.Do(req)
    if err != nil {
        return "", err
    }
    defer resp.Body.Close()

    var result SubmitResponse
    json.NewDecoder(resp.Body).Decode(&result)
    return result.Output.TaskID, nil
}

func pollTask(taskID string) (*TaskResponse, error) {
    for {
        req, _ := http.NewRequest("GET", queryBase+taskID, nil)
        req.Header.Set("Authorization", "Bearer "+apiKey)
        req.Header.Set("X-DashScope-Async", "enable")
        req.Header.Set("Content-Type", "application/json")

        resp, err := http.DefaultClient.Do(req)
        if err != nil {
            return nil, err
        }

        var result TaskResponse
        json.NewDecoder(resp.Body).Decode(&result)
        resp.Body.Close()

        if result.Output.TaskStatus == "SUCCEEDED" || result.Output.TaskStatus == "FAILED" {
            return &result, nil
        }
        time.Sleep(3 * time.Second)
    }
}

func downloadResult(transcriptionURL string) (*TranscriptionResult, error) {
    resp, err := http.Get(transcriptionURL)
    if err != nil {
        return nil, err
    }
    defer resp.Body.Close()

    var result TranscriptionResult
    err = json.NewDecoder(resp.Body).Decode(&result)
    return &result, err
}

func main() {
    taskID, _ := submitTranscription([]string{"https://example.com/audio.mp3"})
    result, _ := pollTask(taskID)
    transcription, _ := downloadResult(result.Output.Results[0].TranscriptionURL)
    fmt.Println(transcription.Transcripts[0].Text)
}
```

### 4.2 Video Understanding (Qwen3.5-Omni-Plus, Streaming)

```go
package main

import (
    "bufio"
    "bytes"
    "encoding/json"
    "fmt"
    "net/http"
    "strings"
)

const (
    videoAPIKey = "sk-xxx"
    videoURL    = "https://dashscope-intl.aliyuncs.com/compatible-mode/v1/chat/completions"
)

type Message struct {
    Role    string        `json:"role"`
    Content []ContentPart `json:"content"`
}

type ContentPart struct {
    Type     string    `json:"type"`
    Text     string    `json:"text,omitempty"`
    VideoURL *VideoURL `json:"video_url,omitempty"`
}

type VideoURL struct {
    URL string `json:"url"`
}

type ChatRequest struct {
    Model         string    `json:"model"`
    Messages      []Message `json:"messages"`
    Stream        bool      `json:"stream"`
    StreamOptions struct {
        IncludeUsage bool `json:"include_usage"`
    } `json:"stream_options"`
    Modalities []string `json:"modalities"`
    MaxTokens  int      `json:"max_tokens,omitempty"`
}

type StreamChunk struct {
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

func analyzeVideo(videoFileURL, prompt string) (string, error) {
    reqBody := ChatRequest{
        Model: "qwen3.5-omni-plus",
        Messages: []Message{
            {
                Role: "user",
                Content: []ContentPart{
                    {
                        Type:     "video_url",
                        VideoURL: &VideoURL{URL: videoFileURL},
                    },
                    {
                        Type: "text",
                        Text: prompt,
                    },
                },
            },
        },
        Modalities: []string{"text"},
        MaxTokens:  4096,
    }
    reqBody.Stream = true
    reqBody.StreamOptions.IncludeUsage = true

    body, _ := json.Marshal(reqBody)
    req, _ := http.NewRequest("POST", videoURL, bytes.NewReader(body))
    req.Header.Set("Authorization", "Bearer "+videoAPIKey)
    req.Header.Set("Content-Type", "application/json")

    resp, err := http.DefaultClient.Do(req)
    if err != nil {
        return "", err
    }
    defer resp.Body.Close()

    var fullText strings.Builder
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

        var chunk StreamChunk
        if err := json.Unmarshal([]byte(data), &chunk); err != nil {
            continue
        }
        for _, c := range chunk.Choices {
            fullText.WriteString(c.Delta.Content)
        }
    }
    return fullText.String(), nil
}

func main() {
    result, _ := analyzeVideo(
        "https://example.com/video.mp4",
        "Describe what happens in this video in detail.",
    )
    fmt.Println(result)
}
```

---

## 5. Gotchas and Limitations

### Transcription
1. **Paraformer is Beijing-only.** Use `fun-asr` for Singapore/international.
2. Audio files must be accessible via **public HTTPS URL** — no direct upload (no multipart), no `oss://` via REST API, no base64 for file transcription.
3. `transcription_url` expires after **24 hours** — download immediately.
4. Billing is based on **speech duration only** (silence excluded), not wall-clock duration.
5. Polling default QPS: **20**, max **100**. Use callbacks for high throughput.
6. For Cantonese output, note that `yue` is spoken Cantonese — there's no standard written form. Output text will be in Chinese characters.

### Video Understanding (Qwen3.5-Omni-Plus)
1. **`stream: true` is mandatory.** Non-streaming calls will error.
2. Max ~400s of 720p video at 1 FPS. For longer videos, consider extracting keyframes or splitting.
3. The `modalities` field must be set. Use `["text"]` for text-only output.
4. Currently in **free preview** — pricing not yet finalized.
5. Rate limit is only **60 RPM** — implement queuing for batch processing.
6. Token counting: video frames and audio are converted to tokens internally. A long video can consume significant tokens.
7. The OpenAI-compatible endpoint is the recommended path for Go clients.

### General
1. API keys are **not interchangeable** between regions.
2. There's also a **US (Virginia)** endpoint at `dashscope-us.aliyuncs.com`.
3. DashScope supports **both** native format and OpenAI-compatible format — for Go, the OpenAI-compatible format is simpler to implement.

---

## 6. Migration Mapping (Gemini → DashScope)

| Gemini Feature | DashScope Replacement | Notes |
|---------------|----------------------|-------|
| Gemini file upload + transcription | Fun-ASR async API | Need public URL instead of upload |
| Gemini video understanding | Qwen3.5-Omni-Plus | OpenAI-compatible API, streaming required |
| Gemini timestamps | `sentences[].begin_time/end_time` | Milliseconds, sentence + word level |
| Gemini language detection | `language_hints` parameter | Explicit language codes required |
| Gemini Cantonese | `language_hints: ["yue"]` | Supported in Fun-ASR and Paraformer |

---

## 7. Summary: Key Decisions

| Decision | Recommendation |
|----------|---------------|
| Transcription model | `fun-asr` (international) or `paraformer-v2` (Beijing) |
| Video model | `qwen3.5-omni-plus` |
| API format | OpenAI-compatible for video, DashScope native for transcription |
| Region | Singapore (`dashscope-intl.aliyuncs.com`) |
| Auth | `Authorization: Bearer sk-xxx` |
| Single API key | Yes, one key for both services |
| Audio file delivery | Must be a public HTTPS URL |
| Cantonese | Supported via `language_hints: ["yue"]` |
