package pkg

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/mux"
)

func (s *HTTPService) IndexVideo(w http.ResponseWriter, r *http.Request) {
	params := mux.Vars(r)
	hashId := params["hash"]

	force := r.URL.Query().Get("force") == "true"

	if !force {
		dbHelper := NewDBHelper(s.config.DBConf)
		existing, err := dbHelper.FindVideoIndex(hashId)
		if err == nil && existing != nil {
			s.ResponseJSON(existing, w)
			return
		}
	}

	taskID := generateID()
	task := &Task{
		ID:     taskID,
		Status: TASK_STATUS_RUNNING,
	}

	tasksMu.Lock()
	tasks[taskID] = task
	tasksMu.Unlock()

	go func(hashId string, taskId string) {
		defer func() {
			<-s.uploadQueue
		}()
		s.uploadQueue <- true

		s.indexVideoToS3(hashId, taskId)
	}(hashId, taskID)

	s.ResponseJSON(task, w)
}

func (s *HTTPService) IndexAllVideo(w http.ResponseWriter, r *http.Request) {
	list := &MultipleMediaBody{}
	if err := json.NewDecoder(r.Body).Decode(&list); err != nil {
		s.ResponseJSONError(&APIStandardError{
			Status:     false,
			Error:      err.Error(),
			HttpStatus: http.StatusBadRequest,
		}, w)
		return
	}

	taskID := generateID()
	task := &Task{
		ID:     taskID,
		Status: TASK_STATUS_RUNNING,
	}

	tasksMu.Lock()
	tasks[taskID] = task
	tasksMu.Unlock()

	go func(taskId string) {
		results := make([]*MoveToS3Result, len(list.HashList))
		wg := sync.WaitGroup{}

		for i, hashId := range list.HashList {
			wg.Add(1)
			go func(hashId string, idx int, wg *sync.WaitGroup) {
				defer wg.Done()
				defer func() {
					<-s.uploadQueue
				}()
				s.uploadQueue <- true

				err := s.indexVideoToS3(hashId, "")
				if err != nil {
					results[idx] = &MoveToS3Result{
						HashId: hashId,
						Status: false,
						Error:  err.Error(),
					}
					return
				}
				results[idx] = &MoveToS3Result{
					HashId: hashId,
					Status: true,
				}
			}(hashId, i, &wg)
		}

		wg.Wait()

		tasksMu.Lock()
		tasks[taskId] = &Task{
			Status: TASK_STATUS_FINISHED,
			Result: results,
			ID:     taskId,
		}
		tasksMu.Unlock()
	}(taskID)

	s.ResponseJSON(task, w)
}

func (s *HTTPService) GetIndex(w http.ResponseWriter, r *http.Request) {
	params := mux.Vars(r)
	hashId := params["hash"]

	dbHelper := NewDBHelper(s.config.DBConf)
	index, err := dbHelper.FindVideoIndex(hashId)
	if err != nil {
		s.ResponseJSONError(&APIStandardError{
			Status:     false,
			Error:      err.Error(),
			HttpStatus: http.StatusNotFound,
		}, w)
		return
	}

	s.ResponseJSON(index, w)
}

func (s *HTTPService) indexVideoToS3(hashId string, taskId string) error {
	s3Conf := s.config.Storage.S3

	storage, err := NewS3Storage(s3Conf)
	if err != nil {
		Log.Error("failed to create S3 storage", "error", err, "hash", hashId, "task", taskId)
		if taskId != "" {
			tasksMu.Lock()
			tasks[taskId] = &Task{ID: taskId, Status: TASK_STATUS_ERROR, Result: err.Error()}
			tasksMu.Unlock()
		}
		return err
	}

	dbHelper := NewDBHelper(s.config.DBConf)
	video, err := dbHelper.FindVideoInfo(hashId)
	if err != nil {
		Log.Info("video not in BoltDB, trying S3 index.json", "hash", hashId, "task", taskId)
		s3IndexUrl := fmt.Sprintf("https://s3.%s.amazonaws.com/%s/%s/media/%s/index.json",
			s3Conf.Region, s3Conf.Bucket, s3Conf.PrefixPath, hashId)
		resp, httpErr := http.Get(s3IndexUrl)
		if httpErr != nil || resp.StatusCode != http.StatusOK {
			errMsg := fmt.Sprintf("video not found, run /move/%s first", hashId)
			if taskId != "" {
				tasksMu.Lock()
				tasks[taskId] = &Task{ID: taskId, Status: TASK_STATUS_ERROR, Result: errMsg}
				tasksMu.Unlock()
			}
			return fmt.Errorf(errMsg)
		}
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		video = &WistiaRespVideo{}
		if err = json.Unmarshal(body, video); err != nil {
			if taskId != "" {
				tasksMu.Lock()
				tasks[taskId] = &Task{ID: taskId, Status: TASK_STATUS_ERROR, Result: err.Error()}
				tasksMu.Unlock()
			}
			return err
		}
	}

	if video.Assets == nil {
		errMsg := fmt.Sprintf("no assets found for video %s", hashId)
		if taskId != "" {
			tasksMu.Lock()
			tasks[taskId] = &Task{ID: taskId, Status: TASK_STATUS_ERROR, Result: errMsg}
			tasksMu.Unlock()
		}
		return fmt.Errorf(errMsg)
	}

	videoFiles := video.Assets.GetVideoFiles()
	if len(videoFiles) == 0 {
		errMsg := fmt.Sprintf("no video files found for video %s", hashId)
		if taskId != "" {
			tasksMu.Lock()
			tasks[taskId] = &Task{ID: taskId, Status: TASK_STATUS_ERROR, Result: errMsg}
			tasksMu.Unlock()
		}
		return fmt.Errorf(errMsg)
	}

	sortedFiles := make([]*WistiaRespVideoAsset, len(videoFiles))
	copy(sortedFiles, videoFiles)
	for i := 0; i < len(sortedFiles); i++ {
		for j := i + 1; j < len(sortedFiles); j++ {
			if sortedFiles[j].FileSize < sortedFiles[i].FileSize {
				sortedFiles[i], sortedFiles[j] = sortedFiles[j], sortedFiles[i]
			}
		}
	}

	dashscopeHelper := NewDashScopeHelper(s.config.DashScopeConf)

	chosenAsset := sortedFiles[0]
	videoUrl := chosenAsset.Url

	Log.Info("indexing video", "hash", hashId, "url", videoUrl, "width", chosenAsset.Width, "height", chosenAsset.Height, "task", taskId)

	audioResult, err := dashscopeHelper.Transcribe(videoUrl)
	if err != nil {
		errMsg := fmt.Sprintf("transcription failed: %v", err)
		if taskId != "" {
			tasksMu.Lock()
			tasks[taskId] = &Task{ID: taskId, Status: TASK_STATUS_ERROR, Result: errMsg}
			tasksMu.Unlock()
		}
		return fmt.Errorf(errMsg)
	}
	Log.Info("transcription complete", "hash", hashId, "subtitles", len(audioResult.Subtitles), "language", audioResult.Language, "task", taskId)

	var videoText string
	var videoUsage *DashScopeTokenUsage
	for _, asset := range sortedFiles {
		vUrl := asset.Url

		Log.Info("analyzing video for summary+chapters", "hash", hashId, "height", asset.Height, "task", taskId)
		text, usage, err := dashscopeHelper.IndexVideo(vUrl, audioResult.Subtitles)
		if err != nil {
			Log.Warn("dashscope video analysis failed, trying next resolution", "hash", hashId, "height", asset.Height, "error", err, "task", taskId)
			continue
		}

		cleanJSON := extractJSON(text)
		testResult := &DashScopeVideoAnalysis{}
		if err := json.Unmarshal([]byte(cleanJSON), testResult); err != nil {
			Log.Warn("dashscope video JSON parse failed, trying next resolution", "hash", hashId, "height", asset.Height, "error", err, "task", taskId)
			continue
		}

		videoText = text
		videoUsage = usage
		chosenAsset = asset
		break
	}

	if videoText == "" {
		errMsg := fmt.Sprintf("all video resolutions failed for %s", hashId)
		if taskId != "" {
			tasksMu.Lock()
			tasks[taskId] = &Task{ID: taskId, Status: TASK_STATUS_ERROR, Result: errMsg}
			tasksMu.Unlock()
		}
		return fmt.Errorf(errMsg)
	}

	videoResult := &DashScopeVideoAnalysis{}
	if err := json.Unmarshal([]byte(extractJSON(videoText)), videoResult); err != nil {
		Log.Error("failed to parse final video analysis JSON", "error", err, "hash", hashId, "task", taskId)
		if taskId != "" {
			tasksMu.Lock()
			tasks[taskId] = &Task{ID: taskId, Status: TASK_STATUS_ERROR, Result: err.Error()}
			tasksMu.Unlock()
		}
		return err
	}

	finalSubtitles := make([]DashScopeSubtitleEntry, len(audioResult.Subtitles))
	copy(finalSubtitles, audioResult.Subtitles)
	if len(videoResult.Subtitles) == len(audioResult.Subtitles) && len(videoResult.Subtitles) > 0 {
		for i := range finalSubtitles {
			finalSubtitles[i].Text = videoResult.Subtitles[i].Text
		}
		Log.Info("merge-by-index: using refined subtitles with ASR timestamps preserved", "hash", hashId, "subtitles", len(finalSubtitles), "task", taskId)
	} else if len(videoResult.Subtitles) > 0 {
		Log.Warn("refined subtitle count mismatch, falling back to ASR subtitles", "hash", hashId, "refined", len(videoResult.Subtitles), "asr", len(audioResult.Subtitles), "task", taskId)
	}

	result := &DashScopeIndexResult{
		HashId:      hashId,
		Model:       s.config.DashScopeConf.VideoModel,
		Source:      chosenAsset,
		GeneratedAt: time.Now().UTC().Format(time.RFC3339),
		Summary:     videoResult.Summary,
		Subtitles:   finalSubtitles,
		Chapters:    videoResult.Chapters,
		TokenUsage:  videoUsage,
	}

	result.Summary = simpToTrad(result.Summary)
	for i := range result.Subtitles {
		result.Subtitles[i].Text = simpToTrad(result.Subtitles[i].Text)
	}
	for i := range result.Chapters {
		result.Chapters[i].Title = simpToTrad(result.Chapters[i].Title)
	}

	if videoUsage != nil {
		Log.Info("dashscope token usage", "hash", hashId, "inputK", videoUsage.InputK, "outputK", videoUsage.OutputK, "totalK", videoUsage.TotalK, "task", taskId)
	}

	vttContent := result.ToVTT()

	jsonBin, _ := json.Marshal(result)

	_, s3JsonUrl, err := storage.PutContent(string(jsonBin),
		fmt.Sprintf("media/%s/index-ai.json", hashId),
		&UploadOptions{ContentType: "application/json", PublicRead: true})
	if err != nil {
		Log.Error("failed to upload index-ai.json to S3", "error", err, "hash", hashId, "task", taskId)
		if taskId != "" {
			tasksMu.Lock()
			tasks[taskId] = &Task{ID: taskId, Status: TASK_STATUS_ERROR, Result: err.Error()}
			tasksMu.Unlock()
		}
		return err
	}
	Log.Info("uploaded index-ai.json to S3", "hash", hashId, "url", s3JsonUrl, "task", taskId)

	_, s3VttUrl, err := storage.PutContent(vttContent,
		fmt.Sprintf("media/%s/subtitles.vtt", hashId),
		&UploadOptions{ContentType: "text/vtt", PublicRead: true})
	if err != nil {
		Log.Error("failed to upload subtitles.vtt to S3", "error", err, "hash", hashId, "task", taskId)
		if taskId != "" {
			tasksMu.Lock()
			tasks[taskId] = &Task{ID: taskId, Status: TASK_STATUS_ERROR, Result: err.Error()}
			tasksMu.Unlock()
		}
		return err
	}
	Log.Info("uploaded subtitles.vtt to S3", "hash", hashId, "url", s3VttUrl, "task", taskId)

	if s3Conf.UseCloudFront() {
		storage.PutContent(string(jsonBin),
			fmt.Sprintf("cloudfront/media/%s/index-ai.json", hashId),
			&UploadOptions{ContentType: "application/json", PublicRead: true})
		storage.PutContent(vttContent,
			fmt.Sprintf("cloudfront/media/%s/subtitles.vtt", hashId),
			&UploadOptions{ContentType: "text/vtt", PublicRead: true})

		cfHelper := NewCloudFrontHelper(s3Conf)
		if cfHelper != nil {
			flushPaths := []string{
				fmt.Sprintf("/%s/cloudfront/media/%s/index-ai.json", s3Conf.PrefixPath, hashId),
				fmt.Sprintf("/%s/cloudfront/media/%s/subtitles.vtt", s3Conf.PrefixPath, hashId),
			}
			if err := cfHelper.InvalidatePaths(flushPaths); err != nil {
				Log.Warn("CloudFront cache invalidation failed", "hash", hashId, "error", err, "paths", flushPaths, "task", taskId)
			}
		}
	}

	err = dbHelper.SaveVideoIndex(hashId, result)
	if err != nil {
		Log.Error("failed to save video index to BoltDB", "error", err, "hash", hashId, "task", taskId)
	}

	if taskId != "" {
		tasksMu.Lock()
		tasks[taskId] = &Task{
			ID:     taskId,
			Status: TASK_STATUS_FINISHED,
			Result: result,
		}
		tasksMu.Unlock()
	}

	return nil
}

type UpdateSubtitlesRequest struct {
	Subtitles []DashScopeSubtitleEntry `json:"subtitles"`
}

func (s *HTTPService) UpdateSubtitles(w http.ResponseWriter, r *http.Request) {
	params := mux.Vars(r)
	hashId := params["hash"]

	var req UpdateSubtitlesRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.ResponseJSONError(&APIStandardError{
			Status:     false,
			Error:      err.Error(),
			HttpStatus: http.StatusBadRequest,
		}, w)
		return
	}

	if len(req.Subtitles) == 0 {
		s.ResponseJSONError(&APIStandardError{
			Status:     false,
			Error:      "subtitles array cannot be empty",
			HttpStatus: http.StatusBadRequest,
		}, w)
		return
	}

	dbHelper := NewDBHelper(s.config.DBConf)
	index, err := dbHelper.FindVideoIndex(hashId)
	if err != nil {
		s.ResponseJSONError(&APIStandardError{
			Status:     false,
			Error:      fmt.Sprintf("index not found for %s, run AI index first", hashId),
			HttpStatus: http.StatusNotFound,
		}, w)
		return
	}

	index.Subtitles = req.Subtitles

	s3Conf := s.config.Storage.S3
	storage, err := NewS3Storage(s3Conf)
	if err != nil {
		s.ResponseJSONError(&APIStandardError{
			Status:     false,
			Error:      err.Error(),
			HttpStatus: http.StatusInternalServerError,
		}, w)
		return
	}

	vttContent := index.ToVTT()
	jsonBin, _ := json.Marshal(index)

	_, _, err = storage.PutContent(string(jsonBin),
		fmt.Sprintf("media/%s/index-ai.json", hashId),
		&UploadOptions{ContentType: "application/json", PublicRead: true})
	if err != nil {
		s.ResponseJSONError(&APIStandardError{
			Status:     false,
			Error:      fmt.Sprintf("failed to upload index-ai.json: %v", err),
			HttpStatus: http.StatusInternalServerError,
		}, w)
		return
	}

	_, _, err = storage.PutContent(vttContent,
		fmt.Sprintf("media/%s/subtitles.vtt", hashId),
		&UploadOptions{ContentType: "text/vtt", PublicRead: true})
	if err != nil {
		s.ResponseJSONError(&APIStandardError{
			Status:     false,
			Error:      fmt.Sprintf("failed to upload subtitles.vtt: %v", err),
			HttpStatus: http.StatusInternalServerError,
		}, w)
		return
	}

	if s3Conf.UseCloudFront() {
		storage.PutContent(string(jsonBin),
			fmt.Sprintf("cloudfront/media/%s/index-ai.json", hashId),
			&UploadOptions{ContentType: "application/json", PublicRead: true})
		storage.PutContent(vttContent,
			fmt.Sprintf("cloudfront/media/%s/subtitles.vtt", hashId),
			&UploadOptions{ContentType: "text/vtt", PublicRead: true})

		cfHelper := NewCloudFrontHelper(s3Conf)
		if cfHelper != nil {
			flushPaths := []string{
				fmt.Sprintf("/%s/cloudfront/media/%s/index-ai.json", s3Conf.PrefixPath, hashId),
				fmt.Sprintf("/%s/cloudfront/media/%s/subtitles.vtt", s3Conf.PrefixPath, hashId),
			}
			if err := cfHelper.InvalidatePaths(flushPaths); err != nil {
				Log.Warn("CloudFront cache invalidation failed", "hash", hashId, "error", err)
			}
		}
	}

	err = dbHelper.SaveVideoIndex(hashId, index)
	if err != nil {
		Log.Error("failed to save updated index to BoltDB", "error", err, "hash", hashId)
	}

	Log.Info("subtitles updated", "hash", hashId, "count", len(req.Subtitles))

	s.ResponseJSON(map[string]interface{}{
		"hashId":        hashId,
		"updatedAt":     time.Now().UTC().Format(time.RFC3339),
		"subtitleCount": len(req.Subtitles),
	}, w)
}
