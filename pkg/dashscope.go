package pkg

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/longbridgeapp/opencc"
)

type DashScopeConf struct {
	ApiKey     string `json:"api_key"`
	BaseURL    string `json:"base_url"`
	ASRModel   string `json:"asr_model"`
	VideoModel string `json:"video_model"`
}

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
	Language  string                   `json:"language"`
	Subtitles []DashScopeSubtitleEntry `json:"subtitles"`
}

type DashScopeVideoAnalysis struct {
	Summary   string                   `json:"summary"`
	Chapters  []DashScopeChapterEntry  `json:"chapters"`
	Subtitles []DashScopeSubtitleEntry `json:"subtitles,omitempty"`
}

type DashScopeIndexResult struct {
	HashId      string                   `json:"hashId"`
	Model       string                   `json:"model"`
	Source      *WistiaRespVideoAsset    `json:"source"`
	GeneratedAt string                   `json:"generatedAt"`
	Summary     string                   `json:"summary"`
	Subtitles   []DashScopeSubtitleEntry `json:"subtitles"`
	Chapters    []DashScopeChapterEntry  `json:"chapters"`
	TokenUsage  *DashScopeTokenUsage     `json:"tokenUsage,omitempty"`
}

func (this *DashScopeIndexResult) ToVTT() string {
	var buf bytes.Buffer
	buf.WriteString("WEBVTT\n\n")
	for i, sub := range this.Subtitles {
		if i > 0 {
			buf.WriteString("\n")
		}
		buf.WriteString(fmt.Sprintf("%s --> %s\n", formatVTTTime(sub.Start), formatVTTTime(sub.End)))
		buf.WriteString(fmt.Sprintf("%s\n", sub.Text))
	}
	return buf.String()
}

func formatVTTTime(seconds float64) string {
	h := int(seconds) / 3600
	m := (int(seconds) % 3600) / 60
	s := int(seconds) % 60
	ms := int(math.Round((seconds - float64(int(seconds))) * 1000))
	if ms >= 1000 {
		ms = 999
	}
	return fmt.Sprintf("%02d:%02d:%02d.%03d", h, m, s, ms)
}

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

type DashScopeHelper struct {
	Conf *DashScopeConf
}

func NewDashScopeHelper(conf *DashScopeConf) *DashScopeHelper {
	return &DashScopeHelper{Conf: conf}
}

var s2hk *opencc.OpenCC

func init() {
	var err error
	s2hk, err = opencc.New("s2hk")
	if err != nil {
		Log.Errorf("failed to init opencc s2hk: %v", err)
	}
}

func simpToTrad(input string) string {
	if s2hk == nil {
		return input
	}
	output, err := s2hk.Convert(input)
	if err != nil {
		return input
	}
	return output
}

type dashscopeFiletransRequest struct {
	Model      string                       `json:"model"`
	Input      dashscopeFiletransInput      `json:"input"`
	Parameters dashscopeFiletransParameters `json:"parameters,omitempty"`
}

type dashscopeFiletransParameters struct {
	ChannelId   []int  `json:"channel_id,omitempty"`
	EnableItn   bool   `json:"enable_itn,omitempty"`
	EnableWords bool   `json:"enable_words,omitempty"`
	Language    string `json:"language,omitempty"`
}

type dashscopeFiletransInput struct {
	FileUrl string `json:"file_url"`
}

type dashscopeFiletransSubmitResponse struct {
	RequestId string `json:"request_id,omitempty"`
	Output    struct {
		TaskId     string `json:"task_id"`
		TaskStatus string `json:"task_status"`
	} `json:"output"`
}

type dashscopeMaaSEnvelope struct {
	Code    string          `json:"code"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data"`
}

func unwrapMaaSResponse(body []byte) ([]byte, error) {
	var env dashscopeMaaSEnvelope
	if err := json.Unmarshal(body, &env); err != nil {
		return body, nil
	}
	if env.Code != "" {
		return nil, fmt.Errorf("MaaS API error: code=%s message=%s", env.Code, env.Message)
	}
	if env.Data != nil {
		return env.Data, nil
	}
	return body, nil
}

type dashscopeFiletransTaskResponse struct {
	RequestId string `json:"request_id,omitempty"`
	Output    struct {
		TaskId        string `json:"task_id"`
		TaskStatus    string `json:"task_status"`
		SubmitTime    string `json:"submit_time,omitempty"`
		ScheduledTime string `json:"scheduled_time,omitempty"`
		EndTime       string `json:"end_time,omitempty"`
		TaskMetrics   *struct {
			TOTAL     int `json:"TOTAL"`
			SUCCEEDED int `json:"SUCCEEDED"`
			FAILED    int `json:"FAILED"`
		} `json:"task_metrics,omitempty"`
		Result *struct {
			TranscriptionUrl string `json:"transcription_url"`
		} `json:"result,omitempty"`
		Code    string `json:"code,omitempty"`
		Message string `json:"message,omitempty"`
	} `json:"output"`
	Usage *struct {
		Seconds int `json:"seconds"`
	} `json:"usage,omitempty"`
}

type dashscopeFiletransResult struct {
	FileUrl   string `json:"file_url,omitempty"`
	AudioInfo *struct {
		Format     string `json:"format"`
		SampleRate int    `json:"sample_rate"`
	} `json:"audio_info,omitempty"`
	Transcripts []struct {
		ChannelId int                          `json:"channel_id"`
		Text      string                       `json:"text"`
		Sentences []dashscopeFiletransSentence `json:"sentences"`
	} `json:"transcripts"`
}

type dashscopeFiletransWord struct {
	BeginTime   int64  `json:"begin_time"`
	EndTime     int64  `json:"end_time"`
	Text        string `json:"text"`
	Punctuation string `json:"punctuation,omitempty"`
}

type dashscopeFiletransSentence struct {
	SentenceId int                      `json:"sentence_id"`
	BeginTime  int64                    `json:"begin_time"`
	EndTime    int64                    `json:"end_time"`
	Language   string                   `json:"language,omitempty"`
	Emotion    string                   `json:"emotion,omitempty"`
	Text       string                   `json:"text"`
	Words      []dashscopeFiletransWord `json:"words,omitempty"`
}

func (this *DashScopeHelper) Transcribe(videoUrl string) (*DashScopeAudioTranscription, error) {
	submitBody := dashscopeFiletransRequest{
		Model: this.Conf.ASRModel,
		Input: dashscopeFiletransInput{
			FileUrl: videoUrl,
		},
		Parameters: dashscopeFiletransParameters{
			ChannelId:   []int{0},
			EnableItn:   true,
			EnableWords: true,
		},
	}
	jsonBody, err := json.Marshal(submitBody)
	if err != nil {
		return nil, err
	}

	submitUrl := fmt.Sprintf("%s/api/v1/services/audio/asr/transcription", this.Conf.BaseURL)
	Log.Infof("submitting ASR task: %s", submitUrl)

	req, err := http.NewRequest("POST", submitUrl, bytes.NewReader(jsonBody))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+this.Conf.ApiKey)
	req.Header.Set("X-DashScope-Async", "enable")
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 10 * time.Minute}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("submit ASR task failed: %w", err)
	}
	respBody, err := io.ReadAll(resp.Body)
	resp.Body.Close()
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("ASR submit API error (status %d): %s", resp.StatusCode, string(respBody))
	}

	unwrapped, err := unwrapMaaSResponse(respBody)
	if err != nil {
		return nil, err
	}
	var submitResp dashscopeFiletransSubmitResponse
	if err := json.Unmarshal(unwrapped, &submitResp); err != nil {
		return nil, fmt.Errorf("parse submit response failed: %w, body: %s", err, string(respBody))
	}

	taskId := submitResp.Output.TaskId
	Log.Infof("ASR task submitted: %s", taskId)

	taskUrl := fmt.Sprintf("%s/api/v1/tasks/%s", this.Conf.BaseURL, taskId)
	var taskResp dashscopeFiletransTaskResponse
	for {
		time.Sleep(3 * time.Second)

		req, err := http.NewRequest("GET", taskUrl, nil)
		if err != nil {
			return nil, err
		}
		req.Header.Set("Authorization", "Bearer "+this.Conf.ApiKey)

		resp, err := client.Do(req)
		if err != nil {
			return nil, fmt.Errorf("poll task failed: %w", err)
		}
		respBody, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			return nil, err
		}

		if resp.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("poll task returned status %d: %s", resp.StatusCode, string(respBody))
		}

		unwrapped, err := unwrapMaaSResponse(respBody)
		if err != nil {
			return nil, err
		}

		if err := json.Unmarshal(unwrapped, &taskResp); err != nil {
			return nil, fmt.Errorf("parse task response failed: %w, body: %s", err, string(respBody))
		}

		Log.Infof("ASR task %s status: %s", taskId, taskResp.Output.TaskStatus)

		switch taskResp.Output.TaskStatus {
		case "SUCCEEDED":
		case "FAILED":
			return nil, fmt.Errorf("ASR task failed: %s", string(unwrapped))
		case "PENDING", "RUNNING":
			continue
		default:
			return nil, fmt.Errorf("unknown ASR task status: %s (response: %s)", taskResp.Output.TaskStatus, string(unwrapped))
		}
		break
	}

	if taskResp.Output.Result == nil {
		return nil, fmt.Errorf("ASR task completed but no result")
	}

	transcriptionUrl := taskResp.Output.Result.TranscriptionUrl
	Log.Infof("downloading ASR result: %s", transcriptionUrl)

	resp, err = http.Get(transcriptionUrl)
	if err != nil {
		return nil, fmt.Errorf("download ASR result failed: %w", err)
	}
	respBody, err = io.ReadAll(resp.Body)
	resp.Body.Close()
	if err != nil {
		return nil, err
	}

	var transResult dashscopeFiletransResult
	if err := json.Unmarshal(respBody, &transResult); err != nil {
		return nil, fmt.Errorf("parse transcription result failed: %w", err)
	}

	var subtitles []DashScopeSubtitleEntry
	for _, transcript := range transResult.Transcripts {
		for _, sentence := range transcript.Sentences {
			subtitles = append(subtitles, DashScopeSubtitleEntry{
				Start: float64(sentence.BeginTime) / 1000.0,
				End:   float64(sentence.EndTime) / 1000.0,
				Text:  sentence.Text,
			})
		}
	}

	transcription := &DashScopeAudioTranscription{
		Language:  "zh",
		Subtitles: subtitles,
	}

	Log.Infof("dashscope ASR done: %d subtitles", len(transcription.Subtitles))
	return transcription, nil
}

func buildVideoPrompt(subtitles []DashScopeSubtitleEntry) string {
	var subtitleContext strings.Builder
	if len(subtitles) > 0 {
		subtitleContext.WriteString("\n\nBelow is the subtitle transcript (with timestamps in seconds) for reference:\n")
		for _, sub := range subtitles {
			subtitleContext.WriteString(fmt.Sprintf("[%.1f-%.1f] %s\n", sub.Start, sub.End, sub.Text))
		}
	}

	return fmt.Sprintf(`Analyze this video and return ONLY a valid JSON object (no markdown, no explanation).

CRITICAL LANGUAGE RULE: You MUST write ALL text output in 繁體中文 (Traditional Chinese). Do NOT use English or 簡體中文 (Simplified Chinese). The summary, chapter titles, and all text content must be in 繁體中文.

Use BOTH the video visual content AND the provided subtitle transcript to produce accurate results. Use the subtitle timestamps as reference to determine chapter boundaries.

The JSON must have:
1. "summary": A concise summary (2-4 sentences) in 繁體中文, based on BOTH audio (subtitles) and visual content.
2. "chapters": Array of entries with "start" (float, seconds), "end" (float, seconds), "title" (descriptive title in 繁體中文). Use the subtitle timestamps as reference to produce chapter time ranges that align with actual content transitions in the video.
3. "subtitles": Array of corrected subtitle entries. Each entry has "start" (float), "end" (float), "text" (string). You MUST preserve the original "start" and "end" timestamps from the input transcript EXACTLY — copy them verbatim. You MUST output the SAME NUMBER of entries in the SAME ORDER as the input transcript. Only fix the "text" field based on audio and visual context: correct homophones, wrong characters, and punctuation. If an entry is already correct, copy it verbatim. All text must be in 繁體中文.%s`, subtitleContext.String())
}

type dashscopeResponseFmt struct {
	Type string `json:"type"`
}

type dashscopeChatRequest struct {
	Model         string             `json:"model"`
	Messages      []dashscopeMessage `json:"messages"`
	Stream        bool               `json:"stream"`
	StreamOptions struct {
		IncludeUsage bool `json:"include_usage"`
	} `json:"stream_options"`
	Modalities     []string              `json:"modalities"`
	MaxTokens      int                   `json:"max_tokens,omitempty"`
	ResponseFormat *dashscopeResponseFmt `json:"response_format,omitempty"`
	EnableThinking bool                  `json:"enable_thinking,omitempty"`
	ThinkingBudget int                   `json:"thinking_budget,omitempty"`
}

type dashscopeMessage struct {
	Role    string                 `json:"role"`
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
			Content          string `json:"content"`
			ReasoningContent string `json:"reasoning_content,omitempty"`
		} `json:"delta"`
		FinishReason *string `json:"finish_reason"`
	} `json:"choices"`
	Usage *struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
		TotalTokens      int `json:"total_tokens"`
	} `json:"usage,omitempty"`
}

func (this *DashScopeHelper) IndexVideo(videoUrl string, subtitles []DashScopeSubtitleEntry) (string, *DashScopeTokenUsage, error) {
	prompt := buildVideoPrompt(subtitles)
	reqBody := dashscopeChatRequest{
		Model: this.Conf.VideoModel,
		Messages: []dashscopeMessage{
			{
				Role: "user",
				Content: []dashscopeContentPart{
					{
						Type:     "video_url",
						VideoURL: &dashscopeVideoURLValue{URL: videoUrl},
					},
					{
						Type: "text",
						Text: prompt,
					},
				},
			},
		},
		Modalities: []string{"text"},
		MaxTokens:  32768,
	}
	reqBody.Stream = true
	reqBody.StreamOptions.IncludeUsage = true

	body, err := json.Marshal(reqBody)
	if err != nil {
		return "", nil, err
	}

	url := fmt.Sprintf("%s/compatible-mode/v1/chat/completions", this.Conf.BaseURL)
	req, err := http.NewRequest("POST", url, bytes.NewReader(body))
	if err != nil {
		return "", nil, err
	}
	req.Header.Set("Authorization", "Bearer "+this.Conf.ApiKey)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 10 * time.Minute}
	resp, err := client.Do(req)
	if err != nil {
		return "", nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		buf := new(bytes.Buffer)
		buf.ReadFrom(resp.Body)
		return "", nil, fmt.Errorf("dashscope video API error (status %d): %s", resp.StatusCode, buf.String())
	}

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

	if err := scanner.Err(); err != nil {
		return "", nil, fmt.Errorf("SSE stream read error: %w", err)
	}

	if fullText.Len() == 0 {
		return "", nil, fmt.Errorf("empty response from dashscope video API")
	}

	return fullText.String(), usage, nil
}
