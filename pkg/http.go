package pkg

import (
	"encoding/json"
	"fmt"
	"github.com/gorilla/mux"
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
