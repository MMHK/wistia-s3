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
		Log.Error(err)
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
		Log.Infof("video %s not in BoltDB, trying S3 index.json", hashId)
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

	Log.Infof("indexing video %s: %s (%dx%d)", hashId, videoUrl, chosenAsset.Width, chosenAsset.Height)

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
	Log.Infof("transcription for %s: %d subtitles, language=%s",
		hashId, len(audioResult.Subtitles), audioResult.Language)

	var videoText string
	var videoUsage *DashScopeTokenUsage
	for _, asset := range sortedFiles {
		vUrl := asset.Url

		Log.Infof("analyzing video %s at %dp for summary+chapters", hashId, asset.Height)
		text, usage, err := dashscopeHelper.IndexVideo(vUrl, audioResult.Subtitles)
		if err != nil {
			Log.Warningf("dashscope video error for %s at %dp: %v, trying next", hashId, asset.Height, err)
			continue
		}

		cleanJSON := extractJSON(text)
		testResult := &DashScopeVideoAnalysis{}
		if err := json.Unmarshal([]byte(cleanJSON), testResult); err != nil {
			Log.Warningf("dashscope video JSON parse failed for %s at %dp: %v, trying next", hashId, asset.Height, err)
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
		Log.Error(err)
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
		Log.Infof("merge-by-index: using %d refined subtitles (ASR timestamps preserved) for %s",
			len(finalSubtitles), hashId)
	} else if len(videoResult.Subtitles) > 0 {
		Log.Warningf("refined subtitle count (%d) != ASR count (%d), falling back to ASR subtitles for %s",
			len(videoResult.Subtitles), len(audioResult.Subtitles), hashId)
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
		Log.Infof("dashscope token usage for %s: input=%.1fK output=%.1fK total=%.1fK",
			hashId, videoUsage.InputK, videoUsage.OutputK, videoUsage.TotalK)
	}

	vttContent := result.ToVTT()

	jsonBin, _ := json.Marshal(result)

	_, s3JsonUrl, err := storage.PutContent(string(jsonBin),
		fmt.Sprintf("media/%s/index-ai.json", hashId),
		&UploadOptions{ContentType: "application/json", PublicRead: true})
	if err != nil {
		Log.Error(err)
		if taskId != "" {
			tasksMu.Lock()
			tasks[taskId] = &Task{ID: taskId, Status: TASK_STATUS_ERROR, Result: err.Error()}
			tasksMu.Unlock()
		}
		return err
	}
	Log.Infof("uploaded index-ai.json for %s: %s", hashId, s3JsonUrl)

	_, s3VttUrl, err := storage.PutContent(vttContent,
		fmt.Sprintf("media/%s/subtitles.vtt", hashId),
		&UploadOptions{ContentType: "text/vtt", PublicRead: true})
	if err != nil {
		Log.Error(err)
		if taskId != "" {
			tasksMu.Lock()
			tasks[taskId] = &Task{ID: taskId, Status: TASK_STATUS_ERROR, Result: err.Error()}
			tasksMu.Unlock()
		}
		return err
	}
	Log.Infof("uploaded subtitles.vtt for %s: %s", hashId, s3VttUrl)

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
				Log.Warningf("CloudFront flush failed for %s index: %v", hashId, err)
			}
		}
	}

	err = dbHelper.SaveVideoIndex(hashId, result)
	if err != nil {
		Log.Error(err)
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
