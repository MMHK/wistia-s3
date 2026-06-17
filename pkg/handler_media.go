package pkg

import (
	"encoding/json"
	"fmt"
	"net/http"
	"path/filepath"
	"strings"
)

func (s *HTTPService) videoWithIndex(v *WistiaRespVideo) map[string]interface{} {
	bin, _ := json.Marshal(v)
	var m map[string]interface{}
	json.Unmarshal(bin, &m)
	dbHelper := NewDBHelper(s.config.DBConf)
	idx, err := dbHelper.FindVideoIndex(v.HashId)
	if err == nil && idx != nil {
		m["index"] = idx
	}
	return m
}

func (s *HTTPService) GetAllVideo(w http.ResponseWriter, r *http.Request) {
	queryParams := r.URL.Query()
	hashId := queryParams.Get("hash")

	dbHelper := NewDBHelper(s.config.DBConf)

	if len(hashId) > 0 {
		video, err := dbHelper.FindVideoInfo(hashId)
		if err != nil {
			s.ResponseJSONError(&APIStandardError{
				Status:     false,
				Error:      err.Error(),
				HttpStatus: http.StatusInternalServerError,
			}, w)
			return
		}
		s.ResponseJSON([]map[string]interface{}{s.videoWithIndex(video)}, w)
		return
	}

	videos, err := dbHelper.GetAllVideoInfo()
	if err != nil {
		s.ResponseJSONError(&APIStandardError{
			Status:     false,
			Error:      err.Error(),
			HttpStatus: http.StatusInternalServerError,
		}, w)
		return
	}

	result := make([]map[string]interface{}, len(videos))
	for i, v := range videos {
		result[i] = s.videoWithIndex(v)
	}
	s.ResponseJSON(result, w)
}

func (s *HTTPService) RefreshVideoInfo(w http.ResponseWriter, r *http.Request) {
	taskID := generateID()
	task := &Task{
		ID:     taskID,
		Status: TASK_STATUS_RUNNING,
	}

	tasksMu.Lock()
	tasks[taskID] = task
	tasksMu.Unlock()

	go func(taskId string) {

		s3, err := NewS3Storage(s.config.Storage.S3)
		if err != nil {
			Log.Error("failed to create S3 storage client for media refresh", "error", err, "task_id", taskId)
			tasksMu.Lock()
			tasks[taskID] = &Task{
				Status: TASK_STATUS_ERROR,
				Result: err.Error(),
				ID:     taskID,
			}
			tasksMu.Unlock()
			return
		}

		Log.Debug("listing all video JSON files from S3", "task_id", taskId)
		files, err := s3.ListFiles("media")
		if err != nil {
			Log.Error("failed to list media files from S3", "error", err, "task_id", taskId, "prefix", "media")
			tasksMu.Lock()
			tasks[taskID] = &Task{
				Status: TASK_STATUS_ERROR,
				Result: err.Error(),
				ID:     taskID,
			}
			tasksMu.Unlock()
			return
		}

		Log.Debug("saving video info to database", "task_id", taskId, "file_count", len(files))
		dbHelper := NewDBHelper(s.config.DBConf)

		for _, row := range files {
			if strings.HasSuffix(row, "index.json") {
				url := fmt.Sprintf("https://s3.%s.amazonaws.com/%s/%s", s.config.Storage.S3.Region, s.config.Storage.S3.Bucket, row)
				resp, err := http.Get(url)
				if err != nil {
					Log.Error("failed to fetch index.json from S3", "error", err, "url", url)
					continue
				}
				defer resp.Body.Close()

				hashId := filepath.Base(strings.Replace(row, "/index.json", "", 1))

				err = dbHelper.SaveVideoInfo(hashId, resp.Body)
				if err != nil {
					Log.Error("failed to save video info to database", "error", err, "hash", hashId)
					continue
				}
			} else if strings.HasSuffix(row, "index-ai.json") {
				url := fmt.Sprintf("https://s3.%s.amazonaws.com/%s/%s", s.config.Storage.S3.Region, s.config.Storage.S3.Bucket, row)
				resp, err := http.Get(url)
				if err != nil {
					Log.Error("failed to fetch index-ai.json from S3", "error", err, "url", url)
					continue
				}
				defer resp.Body.Close()

				var result DashScopeIndexResult
				if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
					Log.Error("failed to decode index-ai.json", "error", err, "url", url)
					continue
				}

				hashId := filepath.Base(strings.Replace(row, "/index-ai.json", "", 1))
				if err := dbHelper.SaveVideoIndex(hashId, &result); err != nil {
					Log.Error("failed to save AI video index to database", "error", err, "hash", hashId)
					continue
				}
			}
		}

		tasksMu.Lock()
		tasks[taskID] = &Task{
			Status: TASK_STATUS_FINISHED,
			Result: true,
			ID:     taskID,
		}
		tasksMu.Unlock()

	}(taskID)

	s.ResponseJSON(task, w)

}

func (s *HTTPService) SaveVideoInfo(s3Json string, hashId string) error {
	dbHelper := NewDBHelper(s.config.DBConf)
	resp, err := http.Get(s3Json)
	if err != nil {
		Log.Error("failed to fetch video JSON from S3", "error", err, "url", s3Json, "hash", hashId)
		return err
	}
	defer resp.Body.Close()
	err = dbHelper.SaveVideoInfo(hashId, resp.Body)
	if err != nil {
		Log.Error("failed to save video info to database", "error", err, "hash", hashId, "url", s3Json)
		return err
	}

	return nil
}

func (s *HTTPService) FindVideoInfo(hashId string) (*WistiaRespVideo, error) {
	dbHelper := NewDBHelper(s.config.DBConf)
	return dbHelper.FindVideoInfo(hashId)
}
