package pkg

import (
	"encoding/json"
	"fmt"
	"github.com/gorilla/mux"
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

const TASK_STATUS_INIT = "init"

const TASK_STATUS_RUNNING = "running"

const TASK_STATUS_FINISHED = "finished"

const TASK_STATUS_ERROR = "error"

type HTTPService struct {
	config *Config
	uploadQueue chan bool
}

type APIStandardError struct {
	Status     bool   `json:"status"`
	Error      string `json:"error"`
	HttpStatus int    `json:"-"`
}

type APIResponse struct {
	Status bool        `json:"status"`
	Data   interface{} `json:"data"`
}

type Task struct {
	ID     string      `json:"id"`
	Status string      `json:"status"`
	Result interface{} `json:"result,omitempty"`
}

type MultipleMediaBody struct {
	HashList []string `json:"media"`
}

type MoveToS3Options struct {
	OverRider bool
}

type MoveToS3Result struct {
	HashId     string `json:"hash"`
	CloudFront string `json:"cloudfront"`
	S3         string `json:"s3"`
	Status     bool   `json:"status"`
	Error      string `json:"error"`
}

var (
	tasks   = make(map[string]*Task)
	tasksMu sync.Mutex
)

func generateID() string {
	return fmt.Sprintf("%d", time.Now().UnixNano())
}

func NewHTTP(conf *Config) *HTTPService {
	return &HTTPService{
		config: conf,
		uploadQueue: make(chan bool, conf.WistiaConf.WorkerLimit),
	}
}

// 中间件函数，用于处理错误并返回固定格式的 JSON 错误响应
func JSONErrorMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// 捕获所有的 HTTP 错误
		defer func() {
			if err := recover(); err != nil {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusInternalServerError)
				json.NewEncoder(w).Encode(APIStandardError{
					HttpStatus: http.StatusInternalServerError,
					Error:      "Internal Server Error",
					Status:     false,
				})
			}
		}()
		// 调用下一个处理器
		next.ServeHTTP(w, r)
	})
}

func (s *HTTPService) Start() {
	r := mux.NewRouter()

	r.Use(JSONErrorMiddleware)

	r.HandleFunc("/", s.RedirectSwagger)
	r.HandleFunc("/refresh/media", s.RefreshVideoInfo).Methods("POST")
	r.HandleFunc("/media", s.GetAllVideo).Methods("GET")
	r.HandleFunc("/move/{hash}", s.VideoToS3).Methods("POST")
	r.HandleFunc("/move", s.VideoToS3).Methods("POST")
	r.HandleFunc("/index/{hash}", s.IndexVideo).Methods("POST")
	r.HandleFunc("/index", s.IndexAllVideo).Methods("POST")
	r.HandleFunc("/index/{hash}", s.GetIndex).Methods("GET")
	r.HandleFunc("/tasks/{id}", s.GetTask).Methods("GET")
	r.PathPrefix("/swagger/").Handler(http.StripPrefix("/swagger/",
		http.FileServer(http.Dir(fmt.Sprintf("%s/swagger", s.config.Webroot)))))
	r.NotFoundHandler = http.HandlerFunc(s.NotFoundHandle)

	Log.Info("http service starting")
	Log.Infof("Please open http://%s\n", s.config.Listen)
	err := http.ListenAndServe(s.config.Listen, r)
	if err != nil {
		Log.Fatal(err)
	}
}

func (s *HTTPService) ResponseJSON(source interface{}, writer http.ResponseWriter) {
	encoder := json.NewEncoder(writer)
	encoder.SetEscapeHTML(false)

	writer.Header().Set("Content-Type", "application/json")
	writer.WriteHeader(http.StatusOK)
	err := encoder.Encode(&APIResponse{
		Status: true,
		Data:   source,
	})
	if err != nil {
		s.ResponseJSONError(&APIStandardError{
			Status:     false,
			Error:      err.Error(),
			HttpStatus: http.StatusInternalServerError,
		}, writer)
	}
}

func (s *HTTPService) ResponseJSONError(err *APIStandardError, writer http.ResponseWriter) {
	writer.Header().Add("Content-Type", "application/json")
	writer.WriteHeader(err.HttpStatus)
	encoder := json.NewEncoder(writer)
	encoder.SetEscapeHTML(false)
	encoder.Encode(err)
}

func (s *HTTPService) NotFoundHandle(writer http.ResponseWriter, request *http.Request) {
	s.ResponseJSONError(&APIStandardError{
		Status:     false,
		Error:      "handle not found!",
		HttpStatus: http.StatusNotFound,
	}, writer)
}

func (s *HTTPService) RedirectSwagger(writer http.ResponseWriter, request *http.Request) {
	http.Redirect(writer, request, "/swagger/index.html", 301)
}

// 查询任务状态和结果
func (s *HTTPService) GetTask(w http.ResponseWriter, r *http.Request) {
	params := mux.Vars(r)
	taskID := params["id"]

	tasksMu.Lock()
	task, exists := tasks[taskID]
	tasksMu.Unlock()

	if !exists {
		s.ResponseJSONError(&APIStandardError{
			Status:     false,
			Error:      "task not found",
			HttpStatus: http.StatusNotFound,
		}, w)
		return
	}

	s.ResponseJSON(task, w)
}

func (s *HTTPService) VideoToS3(w http.ResponseWriter, r *http.Request) {
	list := &MultipleMediaBody{
		HashList: []string{},
	}
	params := mux.Vars(r)
	videoHash := params["hash"]

	queryParams := r.URL.Query()
	refresh := queryParams.Get("forceRefresh")
	opt := &MoveToS3Options{
		OverRider: false,
	}
	if refresh == "true" {
		opt.OverRider = true
	}


	if len(videoHash) > 0 {
		list.HashList = append(list.HashList, videoHash)
	} else {
		if err := json.NewDecoder(r.Body).Decode(&list); err != nil {
			s.ResponseJSONError(&APIStandardError{
				Status:     false,
				Error:      err.Error(),
				HttpStatus: http.StatusBadRequest,
			}, w)
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

	go s.MoveVideoToS3(list, taskID, opt)

	s.ResponseJSON(task, w)
}

func (s *HTTPService) SaveVideoInfo(s3Json string, hashId string) error {
	dbHelper := NewDBHelper(s.config.DBConf)
	resp, err := http.Get(s3Json)
	if err != nil {
		Log.Error(err)
		return err
	}
	defer resp.Body.Close()
	err = dbHelper.SaveVideoInfo(hashId, resp.Body)
	if err != nil {
		Log.Error(err)
		return err
	}

	return nil
}

func (s *HTTPService) FindVideoInfo(hashId string) (*WistiaRespVideo, error) {
	dbHelper := NewDBHelper(s.config.DBConf)
	return dbHelper.FindVideoInfo(hashId)
}

func (s *HTTPService) MoveVideoToS3(source *MultipleMediaBody, TaskId string, options *MoveToS3Options) {
	wg := sync.WaitGroup{}
	overRider := false
	if options != nil && options.OverRider {
		overRider = true
	}

	resultList := make([]*MoveToS3Result, len(source.HashList))

	for i, hashId := range source.HashList {
		wg.Add(1)
		go func(hashId string, taskId string, index int, wg *sync.WaitGroup) {
			defer wg.Done()
			defer func() {
				<- s.uploadQueue
			}()
			s.uploadQueue <- true


			helper := NewWistiaHelper(s.config.WistiaConf)
			if !overRider {
				dbHelper := NewDBHelper(s.config.DBConf)
				_, err := dbHelper.FindVideoInfo(hashId)
				if err == nil {
					cloudfrontJson, s3Json := helper.GenerateVideoInfoURL(hashId, s.config.Storage.S3)

					resultList[index] = &MoveToS3Result{
						HashId:     hashId,
						Status:     true,
						S3:         s3Json,
						CloudFront: cloudfrontJson,
					}
					return
				}
			}

			cloudFrontJson, s3Json, err := helper.MoveToS3(hashId, s.config.Storage.S3)
			if err != nil {
				Log.Error(err)
				resultList[index] = &MoveToS3Result{
					HashId: hashId,
					Status: false,
					Error:  err.Error(),
				}
				return
			}

			resultList[index] = &MoveToS3Result{
				HashId:     hashId,
				Status:     true,
				S3:         s3Json,
				CloudFront: cloudFrontJson,
			}

			defer func() {
				go s.SaveVideoInfo(s3Json, hashId)
			}()

		}(hashId, TaskId, i, &wg)
	}

	wg.Wait()

	tasksMu.Lock()
	tasks[TaskId] = &Task{
		Status: TASK_STATUS_FINISHED,
		Result: resultList,
		ID:     TaskId,
	}
	tasksMu.Unlock()
}

func (s *HTTPService) GetAllVideo(w http.ResponseWriter, r *http.Request) {
	// 获取查询参数
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
		s.ResponseJSON([]*WistiaRespVideo{video}, w)
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

	s.ResponseJSON(videos, w)
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
			Log.Error(err)
			tasksMu.Lock()
			tasks[taskID] = &Task{
				Status: TASK_STATUS_ERROR,
				Result: err.Error(),
				ID:     taskID,
			}
			tasksMu.Unlock()
			return
		}

		Log.Debug("list all video json from S3")
		files, err := s3.ListFiles("media")
		if err != nil {
			Log.Error(err)
			tasksMu.Lock()
			tasks[taskID] = &Task{
				Status: TASK_STATUS_ERROR,
				Result: err.Error(),
				ID:     taskID,
			}
			tasksMu.Unlock()
			return
		}

		Log.Debug("saving video info to database")
		dbHelper := NewDBHelper(s.config.DBConf)

		for _, row := range files {
			if strings.HasSuffix(row, "index.json") {
				url := fmt.Sprintf("https://s3.%s.amazonaws.com/%s/%s", s.config.Storage.S3.Region, s.config.Storage.S3.Bucket, row)
				resp, err := http.Get(url)
				if err != nil {
					Log.Error(err)
					continue
				}
				defer resp.Body.Close()

				hashId := filepath.Base(strings.Replace(row, "/index.json", "", 1))

				err = dbHelper.SaveVideoInfo(hashId, resp.Body)
				if err != nil {
					Log.Error(err)
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

	go s.indexVideoToS3(hashId, taskID)

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

	result := &DashScopeIndexResult{
		HashId:      hashId,
		Model:       s.config.DashScopeConf.VideoModel,
		Source:      chosenAsset,
		GeneratedAt: time.Now().UTC().Format(time.RFC3339),
		Summary:     videoResult.Summary,
		Subtitles:   audioResult.Subtitles,
		Chapters:    videoResult.Chapters,
		TokenUsage:  videoUsage,
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
