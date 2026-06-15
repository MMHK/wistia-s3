package pkg

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"wistia-s3/tests"
)

func TestDashScopeConf_MarginWithENV(t *testing.T) {
	conf := new(DashScopeConf)
	conf.MarginWithENV()

	if conf.ApiKey == "" {
		t.Skip("DASHSCOPE_API_KEY not set in .env, skipping")
	}

	t.Logf("base_url: %s", conf.BaseURL)
	t.Logf("asr_model: %s", conf.ASRModel)
	t.Logf("video_model: %s", conf.VideoModel)

	if conf.BaseURL != "https://dashscope-intl.aliyuncs.com" {
		t.Errorf("expected default base_url https://dashscope-intl.aliyuncs.com, got %s", conf.BaseURL)
	}
	if conf.ASRModel != "fun-asr" {
		t.Errorf("expected default asr_model fun-asr, got %s", conf.ASRModel)
	}
	if conf.VideoModel != "qwen3.5-omni-plus" {
		t.Errorf("expected default video_model qwen3.5-omni-plus, got %s", conf.VideoModel)
	}

	t.Log("PASS")
}

func TestDashScopeFormatVTTTime(t *testing.T) {
	tests := []struct {
		seconds  float64
		expected string
	}{
		{0.0, "00:00:00.000"},
		{3.5, "00:00:03.500"},
		{65.123, "00:01:05.123"},
		{3661.999, "01:01:01.999"},
		{0.001, "00:00:00.001"},
		{59.999, "00:00:59.999"},
	}

	for _, tc := range tests {
		result := formatVTTTime(tc.seconds)
		if result != tc.expected {
			t.Errorf("formatVTTTime(%f) = %s, want %s", tc.seconds, result, tc.expected)
		}
	}

	t.Log("PASS")
}

func TestDashScopeIndexResult_ToVTT(t *testing.T) {
	result := &DashScopeIndexResult{
		HashId: "test123",
		Subtitles: []DashScopeSubtitleEntry{
			{Start: 0.0, End: 3.5, Text: "Hello everyone"},
			{Start: 3.5, End: 7.2, Text: "Welcome to the video"},
			{Start: 65.0, End: 70.5, Text: "This is chapter two"},
		},
	}

	vtt := result.ToVTT()

	if !strings.HasPrefix(vtt, "WEBVTT\n") {
		t.Errorf("VTT should start with WEBVTT header, got: %s", vtt[:20])
	}

	if !strings.Contains(vtt, "00:00:00.000 --> 00:00:03.500") {
		t.Errorf("VTT should contain first timestamp, got: %s", vtt)
	}

	if !strings.Contains(vtt, "Hello everyone") {
		t.Errorf("VTT should contain subtitle text, got: %s", vtt)
	}

	if !strings.Contains(vtt, "00:01:05.000 --> 00:01:10.500") {
		t.Errorf("VTT should format times > 60s correctly, got: %s", vtt)
	}

	lines := strings.Split(vtt, "\n")
	t.Logf("VTT output (%d lines):\n%s", len(lines), vtt)

	t.Log("PASS")
}

func TestDashScopeIndexResult_ToVTT_Empty(t *testing.T) {
	result := &DashScopeIndexResult{
		HashId:    "test123",
		Subtitles: []DashScopeSubtitleEntry{},
	}

	vtt := result.ToVTT()
	if vtt != "WEBVTT\n\n" {
		t.Errorf("empty subtitles should produce header only, got: %q", vtt)
	}

	t.Log("PASS")
}

func TestDashScopeIndexResult_JSON(t *testing.T) {
	result := &DashScopeIndexResult{
		HashId: "u7k1cgyjy0",
		Model:  "qwen3.5-omni-plus",
		Source: &WistiaRespVideoAsset{
			Type:     "Mp4VideoFile",
			Url:      "https://example.com/224.mp4",
			FileSize: 4861043,
			Width:    400,
			Height:   224,
		},
		GeneratedAt: "2026-06-15T10:30:00Z",
		Summary:     "A test video about software development.",
		Subtitles: []DashScopeSubtitleEntry{
			{Start: 0.0, End: 3.5, Text: "Hello"},
			{Start: 3.5, End: 7.0, Text: "World"},
		},
		Chapters: []DashScopeChapterEntry{
			{Start: 0.0, End: 45.0, Title: "Intro"},
			{Start: 45.0, End: 120.0, Title: "Main"},
		},
		TokenUsage: &DashScopeTokenUsage{
			InputK:  1.2,
			OutputK: 0.3,
			TotalK:  1.5,
		},
	}

	bin, err := json.Marshal(result)
	if err != nil {
		t.Error(err)
		t.Fail()
		return
	}

	t.Log(tests.ToJSON(result))

	var parsed DashScopeIndexResult
	err = json.Unmarshal(bin, &parsed)
	if err != nil {
		t.Error(err)
		t.Fail()
		return
	}

	if parsed.HashId != result.HashId {
		t.Errorf("HashId mismatch: %s vs %s", parsed.HashId, result.HashId)
	}
	if parsed.Model != result.Model {
		t.Errorf("Model mismatch: %s vs %s", parsed.Model, result.Model)
	}
	if len(parsed.Subtitles) != 2 {
		t.Errorf("expected 2 subtitles, got %d", len(parsed.Subtitles))
	}
	if len(parsed.Chapters) != 2 {
		t.Errorf("expected 2 chapters, got %d", len(parsed.Chapters))
	}
	if parsed.Source.Height != 224 {
		t.Errorf("expected source height 224, got %d", parsed.Source.Height)
	}
	if parsed.TokenUsage == nil {
		t.Errorf("expected token usage, got nil")
	}

	t.Log("PASS")
}

func TestExtractJSON(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{
			input:    `{"summary": "test"}`,
			expected: `{"summary": "test"}`,
		},
		{
			input:    "```json\n{\"summary\": \"test\"}\n```",
			expected: `{"summary": "test"}`,
		},
		{
			input:    "```\n{\"summary\": \"test\"}\n```",
			expected: `{"summary": "test"}`,
		},
		{
			input:    "  ```json\n{\"summary\": \"test\"}\n```  ",
			expected: `{"summary": "test"}`,
		},
	}

	for i, tc := range tests {
		result := extractJSON(tc.input)
		if result != tc.expected {
			t.Errorf("test %d: extractJSON(%q) = %q, want %q", i, tc.input, result, tc.expected)
		}
	}

	t.Log("PASS")
}

func TestDBHelper_SaveDashScopeVideoIndex(t *testing.T) {
	conf := new(Config)
	conf.MarginWithENV()

	dbHelper := NewDBHelper(conf.DBConf)

	index := &DashScopeIndexResult{
		HashId: "test_dashscope_001",
		Model:  "qwen3.5-omni-plus",
		Source: &WistiaRespVideoAsset{
			Type:     "Mp4VideoFile",
			Url:      "https://example.com/224.mp4",
			FileSize: 4861043,
			Width:    400,
			Height:   224,
		},
		GeneratedAt: "2026-06-15T10:30:00Z",
		Summary:     "A test video for unit testing.",
		Subtitles: []DashScopeSubtitleEntry{
			{Start: 0.0, End: 3.0, Text: "Testing one two three"},
		},
		Chapters: []DashScopeChapterEntry{
			{Start: 0.0, End: 60.0, Title: "Test Chapter"},
		},
	}

	err := dbHelper.SaveVideoIndex("test_dashscope_001", index)
	if err != nil {
		t.Error(err)
		t.Fail()
		return
	}

	found, err := dbHelper.FindVideoIndex("test_dashscope_001")
	if err != nil {
		t.Error(err)
		t.Fail()
		return
	}

	if found.HashId != "test_dashscope_001" {
		t.Errorf("expected hashId test_dashscope_001, got %s", found.HashId)
	}
	if found.Summary != "A test video for unit testing." {
		t.Errorf("summary mismatch: %s", found.Summary)
	}
	if len(found.Subtitles) != 1 {
		t.Errorf("expected 1 subtitle, got %d", len(found.Subtitles))
	}
	if len(found.Chapters) != 1 {
		t.Errorf("expected 1 chapter, got %d", len(found.Chapters))
	}

	t.Log(tests.ToJSON(found))
	t.Log("PASS")
}

func TestE2E_IndexVideoToS3_DashScope(t *testing.T) {
	conf := new(Config)
	conf.MarginWithENV()

	if conf.DashScopeConf == nil || conf.DashScopeConf.ApiKey == "" {
		t.Skip("DASHSCOPE_API_KEY not set in .env, skipping E2E test")
	}
	if conf.Storage == nil || conf.Storage.S3 == nil || conf.Storage.S3.AccessKey == "" {
		t.Skip("S3 credentials not set in .env, skipping E2E test")
	}

	rawJSON := `{
    "name": "TrainingVideo_PGB01_14082023",
    "id": 108341755,
    "hashed_id": "u7k1cgyjy0",
    "duration": 177.842,
    "status": "ready",
    "progress": 1,
    "archived": false,
    "section": "Zurich Training Video (For Training Site)",
    "thumbnail": {
        "url": "https://embed-ssl.wistia.com/deliveries/f61380909a61d92b56c60a7ebb95328feb4d91f0.jpg?image_crop_resized=200x120",
        "width": 200,
        "height": 120
    },
    "assets": [
        {
            "type": "OriginalFile",
            "url": "https://demo.static.mixmedia.com/wistia-backup/media/u7k1cgyjy0/original.mp4",
            "fileSize": 112194856,
            "contentType": "video/mp4",
            "width": 1920,
            "height": 1080
        },
        {
            "type": "IphoneVideoFile",
            "url": "https://demo.static.mixmedia.com/wistia-backup/media/u7k1cgyjy0/360.mp4",
            "fileSize": 6767304,
            "contentType": "video/mp4",
            "width": 640,
            "height": 360
        },
        {
            "type": "Mp4VideoFile",
            "url": "https://demo.static.mixmedia.com/wistia-backup/media/u7k1cgyjy0/224.mp4",
            "fileSize": 4861043,
            "contentType": "video/mp4",
            "width": 400,
            "height": 224
        },
        {
            "type": "MdMp4VideoFile",
            "url": "https://demo.static.mixmedia.com/wistia-backup/media/u7k1cgyjy0/540.mp4",
            "fileSize": 9299596,
            "contentType": "video/mp4",
            "width": 960,
            "height": 540
        },
        {
            "type": "HdMp4VideoFile",
            "url": "https://demo.static.mixmedia.com/wistia-backup/media/u7k1cgyjy0/720.mp4",
            "fileSize": 12515969,
            "contentType": "video/mp4",
            "width": 1280,
            "height": 720
        },
        {
            "type": "HdMp4VideoFile",
            "url": "https://demo.static.mixmedia.com/wistia-backup/media/u7k1cgyjy0/1080.mp4",
            "fileSize": 19703676,
            "contentType": "video/mp4",
            "width": 1920,
            "height": 1080
        },
        {
            "type": "StillImageFile",
            "url": "https://demo.static.mixmedia.com/wistia-backup/media/u7k1cgyjy0/cover.jpg",
            "fileSize": 381245,
            "contentType": "image/jpg",
            "width": 1920,
            "height": 1080
        },
        {
            "type": "StoryboardFile",
            "url": "https://demo.static.mixmedia.com/wistia-backup/media/u7k1cgyjy0/2260.jpg",
            "fileSize": 1080266,
            "contentType": "image/jpg",
            "width": 2000,
            "height": 2260
        }
    ],
    "project": {
        "name": "SpeedyAgency",
        "id": 2637588,
        "hashed_id": "wvgixiqhom"
    }
}`

	var video WistiaRespVideo
	err := json.Unmarshal([]byte(rawJSON), &video)
	if err != nil {
		t.Fatalf("failed to parse video JSON: %v", err)
	}
	t.Logf("video: %s (hash: %s, duration: %.1fs)", video.Name, video.HashId, video.Duration)

	videoFiles := video.Assets.GetVideoFiles()
	if len(videoFiles) == 0 {
		t.Fatal("no video files found in assets")
	}

	smallest := videoFiles[0]
	for _, f := range videoFiles[1:] {
		if f.FileSize < smallest.FileSize {
			smallest = f
		}
	}
	t.Logf("smallest video: %s (%dx%d, %d bytes)", smallest.Url, smallest.Width, smallest.Height, smallest.FileSize)

	s3Conf := conf.Storage.S3
	videoUrl := smallest.Url
	t.Logf("video URL for DashScope: %s", videoUrl)

	dashscopeHelper := NewDashScopeHelper(conf.DashScopeConf)

	t.Logf("=== Step 1: Transcribe with qwen3-asr-flash-filetrans ===")
	audioResult, err := dashscopeHelper.Transcribe(videoUrl)
	if err != nil {
		t.Fatalf("transcription failed: %v", err)
	}
	t.Logf("transcription: %d subtitles, language=%s", len(audioResult.Subtitles), audioResult.Language)
	for i, sub := range audioResult.Subtitles {
		t.Logf("  sub[%d] [%.3f - %.3f] %s", i, sub.Start, sub.End, sub.Text)
		if i >= 10 {
			t.Logf("  ... (%d more)", len(audioResult.Subtitles)-11)
			break
		}
	}

	t.Logf("=== Step 2: Index video with Qwen3.5-Omni-Plus ===")
	var videoText string
	var videoUsage *DashScopeTokenUsage
	var chosenAsset *WistiaRespVideoAsset

	sortedFiles := make([]*WistiaRespVideoAsset, len(videoFiles))
	copy(sortedFiles, videoFiles)
	for i := 0; i < len(sortedFiles); i++ {
		for j := i + 1; j < len(sortedFiles); j++ {
			if sortedFiles[j].FileSize < sortedFiles[i].FileSize {
				sortedFiles[i], sortedFiles[j] = sortedFiles[j], sortedFiles[i]
			}
		}
	}

	for _, asset := range sortedFiles {
		vUrl := asset.Url
		t.Logf("trying video analysis at %dp: %s", asset.Height, vUrl)
		text, usage, err := dashscopeHelper.IndexVideo(vUrl, audioResult.Subtitles)
		if err != nil {
			t.Logf("video analysis failed for %dp: %v, trying next", asset.Height, err)
			continue
		}

		cleanJSON := extractJSON(text)
		testResult := &DashScopeVideoAnalysis{}
		if err := json.Unmarshal([]byte(cleanJSON), testResult); err != nil {
			t.Logf("video parse failed for %dp: %v, trying next", asset.Height, err)
			continue
		}

		videoText = text
		videoUsage = usage
		chosenAsset = asset
		t.Logf("video analysis succeeded at %dp", asset.Height)
		break
	}

	if videoText == "" {
		t.Fatal("all video resolutions failed for summary+chapters")
	}

	videoResult := &DashScopeVideoAnalysis{}
	if err := json.Unmarshal([]byte(extractJSON(videoText)), videoResult); err != nil {
		t.Fatalf("video parse failed: %v\nraw: %s", err, videoText)
	}
	t.Logf("summary: %s", videoResult.Summary)
	t.Logf("chapters: %d entries", len(videoResult.Chapters))
	for i, ch := range videoResult.Chapters {
		t.Logf("  chapter[%d] [%.1f - %.1f] %s", i, ch.Start, ch.End, ch.Title)
	}

	if videoUsage != nil {
		t.Logf("token usage: input=%.1fK output=%.1fK total=%.1fK",
			videoUsage.InputK, videoUsage.OutputK, videoUsage.TotalK)
	}

	result := &DashScopeIndexResult{
		HashId:      video.HashId,
		Model:       conf.DashScopeConf.VideoModel,
		Source:      chosenAsset,
		GeneratedAt: "2026-06-15T00:00:00Z",
		Summary:     videoResult.Summary,
		Subtitles:   audioResult.Subtitles,
		Chapters:    videoResult.Chapters,
		TokenUsage:  videoUsage,
	}

	t.Logf("=== Step 3: Assemble result ===")
	t.Logf("subtitles: %d entries", len(result.Subtitles))
	t.Logf("chapters: %d entries", len(result.Chapters))

	if len(result.Subtitles) == 0 {
		t.Error("no subtitles returned")
	}
	if len(result.Chapters) == 0 {
		t.Error("no chapters returned")
	}
	if result.Summary == "" {
		t.Error("empty summary")
	}

	vttContent := result.ToVTT()

	jsonBin, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		t.Fatalf("failed to marshal result: %v", err)
	}

	t.Logf("=== Step 4: Upload to S3 ===")
	storage, err := NewS3Storage(s3Conf)
	if err != nil {
		t.Fatalf("S3 storage error: %v", err)
	}

	_, s3JsonUrl, err := storage.PutContent(string(jsonBin),
		fmt.Sprintf("media/%s/index-ai.json", video.HashId),
		&UploadOptions{ContentType: "application/json", PublicRead: true})
	if err != nil {
		t.Fatalf("failed to upload index-ai.json: %v", err)
	}
	t.Logf("uploaded index-ai.json: %s", s3JsonUrl)

	_, s3VttUrl, err := storage.PutContent(vttContent,
		fmt.Sprintf("media/%s/subtitles.vtt", video.HashId),
		&UploadOptions{ContentType: "text/vtt", PublicRead: true})
	if err != nil {
		t.Fatalf("failed to upload subtitles.vtt: %v", err)
	}
	t.Logf("uploaded subtitles.vtt: %s", s3VttUrl)

	t.Logf("=== Step 5: Save to BoltDB ===")
	dbHelper := NewDBHelper(conf.DBConf)
	err = dbHelper.SaveVideoIndex(video.HashId, result)
	if err != nil {
		t.Fatalf("failed to save to BoltDB: %v", err)
	}
	t.Log("saved to BoltDB")

	found, err := dbHelper.FindVideoIndex(video.HashId)
	if err != nil {
		t.Fatalf("failed to read back from BoltDB: %v", err)
	}
	if found.Summary != result.Summary {
		t.Errorf("BoltDB round-trip summary mismatch")
	}
	if len(found.Subtitles) != len(result.Subtitles) {
		t.Errorf("BoltDB round-trip subtitles count mismatch: %d vs %d", len(found.Subtitles), len(result.Subtitles))
	}
	t.Log("BoltDB round-trip verified")

	t.Logf("=== E2E TEST PASSED ===")
	t.Logf("  Video:      %s (%s)", video.Name, video.HashId)
	t.Logf("  Model:      %s", result.Model)
	t.Logf("  Source:     %s (%dx%d)", chosenAsset.Type, chosenAsset.Width, chosenAsset.Height)
	t.Logf("  Summary:    %s", result.Summary[:minInt(len(result.Summary), 80)])
	t.Logf("  Subtitles:  %d entries", len(result.Subtitles))
	t.Logf("  Chapters:   %d entries", len(result.Chapters))
	if result.TokenUsage != nil {
		t.Logf("  Tokens:     input=%.1fK output=%.1fK total=%.1fK",
			result.TokenUsage.InputK, result.TokenUsage.OutputK, result.TokenUsage.TotalK)
	}
	t.Logf("  S3 JSON:    %s", s3JsonUrl)
	t.Logf("  S3 VTT:     %s", s3VttUrl)
	t.Log("PASS")
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}
