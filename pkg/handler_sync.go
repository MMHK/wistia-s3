package pkg

import (
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"strconv"
	"time"
)

func (s *HTTPService) SyncWistiaVideos(w http.ResponseWriter, r *http.Request) {
	taskID := generateID()
	task := &Task{
		ID:     taskID,
		Status: TASK_STATUS_RUNNING,
	}

	tasksMu.Lock()
	tasks[taskID] = task
	tasksMu.Unlock()

	go func(taskId string) {
		helper := NewWistiaHelper(s.config.WistiaConf)
		dbHelper := NewDBHelper(s.config.DBConf)

		videos, err := helper.ListAllVideos()
		if err != nil {
			Log.Error("sync: failed to list Wistia videos", "error", err, "task_id", taskId)
			if len(videos) == 0 {
				tasksMu.Lock()
				tasks[taskId] = &Task{
					Status: TASK_STATUS_ERROR,
					Result: err.Error(),
					ID:     taskId,
				}
				tasksMu.Unlock()
				return
			}
			Log.Warn("sync: partial results from ListAllVideos, saving what we got", "count", len(videos), "task_id", taskId)
		}

		saved := 0
		failed := 0
		for _, video := range videos {
			if err := dbHelper.SaveWistiaCatalogVideo(video.HashId, video); err != nil {
				Log.Error("sync: failed to save catalog video", "error", err, "hash", video.HashId, "task_id", taskId)
				failed++
				continue
			}
			saved++
		}

		meta := &WistiaSyncMeta{
			LastSyncAt: time.Now().Format(time.RFC3339),
			TotalCount: len(videos),
			PageCount:  (len(videos) + 49) / 50,
		}
		if err := dbHelper.SaveWistiaSyncMeta(meta); err != nil {
			Log.Error("sync: failed to save sync meta", "error", err, "task_id", taskId)
			tasksMu.Lock()
			tasks[taskId] = &Task{
				Status: TASK_STATUS_ERROR,
				Result: err.Error(),
				ID:     taskId,
			}
			tasksMu.Unlock()
			return
		}

		resultMsg := fmt.Sprintf("synced %d videos (%d failed) out of %d total", saved, failed, len(videos))
		if err != nil {
			resultMsg = fmt.Sprintf("partial sync: %d videos saved (%d failed), error: %v", saved, failed, err)
		}
		Log.Info("sync: completed", "saved", saved, "failed", failed, "total", len(videos), "task_id", taskId)
		tasksMu.Lock()
		tasks[taskId] = &Task{
			Status: TASK_STATUS_FINISHED,
			Result: resultMsg,
			ID:     taskId,
		}
		tasksMu.Unlock()
	}(taskID)

	s.ResponseJSON(task, w)
}

type wistiaMediaListResponse struct {
	Status     bool              `json:"status"`
	Data       []*WistiaRespVideo `json:"data"`
	Pagination *paginationMeta    `json:"pagination,omitempty"`
}

type paginationMeta struct {
	Page       int `json:"page"`
	PerPage    int `json:"per_page"`
	Total      int `json:"total"`
	TotalPages int `json:"total_pages"`
}

func (s *HTTPService) GetWistiaMedia(w http.ResponseWriter, r *http.Request) {
	dbHelper := NewDBHelper(s.config.DBConf)

	hash := r.URL.Query().Get("hash")
	if hash != "" {
		video, err := dbHelper.FindWistiaCatalogVideo(hash)
		if err != nil {
			Log.Error("get wistia media: video not found", "error", err, "hash", hash)
			s.ResponseJSONError(&APIStandardError{
				Status:     false,
				Error:      fmt.Sprintf("video not found: %s", hash),
				HttpStatus: http.StatusNotFound,
			}, w)
			return
		}
		s.ResponseJSON([]*WistiaRespVideo{video}, w)
		return
	}

	all, err := dbHelper.GetAllWistiaCatalogVideos()
	if err != nil {
		Log.Error("get wistia media: failed to fetch catalog", "error", err)
		s.ResponseJSONError(&APIStandardError{
			Status:     false,
			Error:      "failed to fetch wistia catalog",
			HttpStatus: http.StatusInternalServerError,
		}, w)
		return
	}

	archivedFilter := r.URL.Query().Get("archived")
	if archivedFilter != "" {
		filterArchived := archivedFilter == "true"
		filtered := make([]*WistiaRespVideo, 0, len(all))
		for _, v := range all {
			if v.Archived == filterArchived {
				filtered = append(filtered, v)
			}
		}
		all = filtered
	}

	page := 1
	perPage := 50
	if p := r.URL.Query().Get("page"); p != "" {
		if v, err := strconv.Atoi(p); err == nil && v > 0 {
			page = v
		}
	}
	if pp := r.URL.Query().Get("per_page"); pp != "" {
		if v, err := strconv.Atoi(pp); err == nil && v > 0 {
			perPage = v
		}
	}
	if perPage > 100 {
		perPage = 100
	}

	total := len(all)
	totalPages := int(math.Ceil(float64(total) / float64(perPage)))
	if totalPages < 1 {
		totalPages = 1
	}

	start := (page - 1) * perPage
	end := start + perPage
	if start > total {
		start = total
	}
	if end > total {
		end = total
	}

	resp := wistiaMediaListResponse{
		Status: true,
		Data:   all[start:end],
		Pagination: &paginationMeta{
			Page:       page,
			PerPage:    perPage,
			Total:      total,
			TotalPages: totalPages,
		},
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	encoder := json.NewEncoder(w)
	encoder.SetEscapeHTML(false)
	encoder.Encode(resp)
}
