package pkg

import (
	"encoding/json"
	"fmt"
	"github.com/gorilla/mux"
	"net/http"
	"sync"
	"time"
)

const TASK_STATUS_INIT = "init";
const TASK_STATUS_RUNNING = "running";
const TASK_STATUS_FINISHED = "finished";
const TASK_STATUS_ERROR = "error";


type HTTPService struct {
	config *Config
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

type Move2S3Result struct {
	CloudFront string `json:"cloudfront"`
	S3         string `json:"s3"`
}

type Task struct {
	ID     string      `json:"id"`
	Status string      `json:"status"`
	Result interface{} `json:"result,omitempty"`
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
					Error: "Internal Server Error",
					Status: false,
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
	r.HandleFunc("/media", s.GetAllVideo).Methods("GET")
	r.HandleFunc("/move/{hash}", s.VideoToS3).Methods("POST")
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
	params := mux.Vars(r)
	videoHash := params["hash"]

	taskID := generateID()
	task := &Task{
		ID:     taskID,
		Status: TASK_STATUS_RUNNING,
	}

	tasksMu.Lock()
	tasks[taskID] = task
	tasksMu.Unlock()

	go func(hashId string, taskId string) {
		helper := NewWistiaHelper(s.config.WistiaConf)
		cloudFrontJson, s3Json, err := helper.MoveToS3(hashId, s.config.Storage.S3)
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

		tasksMu.Lock()
		tasks[taskID] = &Task{
			Status: TASK_STATUS_FINISHED,
			Result: &Move2S3Result{
				CloudFront: cloudFrontJson,
				S3:         s3Json,
			},
			ID:     taskID,
		}
		tasksMu.Unlock()

		go func() {
			dbHelper := NewDBHelper(s.config.DBConf)
			resp, err := http.Get(s3Json)
			if err != nil {
				Log.Error(err)
				return
			}
			defer resp.Body.Close()
			err = dbHelper.SaveVideoInfo(hashId, resp.Body)
			if err != nil {
				Log.Error(err)
			}
		}()

	}(videoHash, taskID)

	s.ResponseJSON(task, w)
}

func (s *HTTPService) GetAllVideo(w http.ResponseWriter, r *http.Request) {
	dbHelper := NewDBHelper(s.config.DBConf)
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


