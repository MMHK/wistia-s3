package pkg

import (
	"encoding/json"
	"net/http"
	"sync"

	"github.com/gorilla/mux"
)

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
