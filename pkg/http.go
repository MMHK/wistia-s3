package pkg

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/mux"
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
	r.HandleFunc("/ui", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/ui/index.html", http.StatusFound)
	})
	r.PathPrefix("/ui/").Handler(http.StripPrefix("/ui/",
		http.FileServer(http.Dir(fmt.Sprintf("%s/webui", s.config.Webroot)))))
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
