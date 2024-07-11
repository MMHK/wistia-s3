package pkg

import (
	"encoding/json"
	"fmt"
	"mime"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"
)

const WISTIA_API_ENDPOINT = "https://api.wistia.com/v1/"

type WistiaRespVideoAsset struct {
	Type        string `json:"type"`
	Url         string `json:"url"`
	FileSize    int    `json:"fileSize"`
	ContentType string `json:"contentType"`
	Width       int    `json:"width"`
	Height      int    `json:"height"`
}

type WistiaRespVideoProject struct {
	Name   string `json:"name"`
	Id     int `json:"id"`
	HashId string `json:"hashed_id"`
}

type WistiaRespVideoThumbnail struct {
	Url    string `json:"url"`
	Width  int    `json:"width"`
	Height int    `json:"height"`
}

type AssetList []*WistiaRespVideoAsset

func (a *AssetList) GetVideoFiles() []*WistiaRespVideoAsset {
	var result []*WistiaRespVideoAsset
	for _, asset := range *a {
		if strings.Contains(asset.Type, "VideoFile") {
			result = append(result, asset)
		}
	}
	return result
}

func (a *AssetList) GetCover() *WistiaRespVideoAsset {
	for _, asset := range *a {
		if asset.Type == "StillImageFile" {
			return asset
		}
	}
	return nil
}

func (a *AssetList) GetOriginal() *WistiaRespVideoAsset {
	for _, asset := range *a {
		if asset.Type == "OriginalFile" {
			return asset
		}
	}
	return nil
}

type WistiaRespVideo struct {
	Name      string                    `json:"name"`
	Id        int                       `json:"id"`
	HashId    string                    `json:"hashed_id"`
	Duration  float32                   `json:"duration"`
	Status    string                    `json:"status"`
	Progress  float32                   `json:"progress"`
	Archived  bool                      `json:"archived"`
	Section   string                    `json:"section"`
	Thumbnail *WistiaRespVideoThumbnail `json:"thumbnail"`
	Assets    *AssetList                `json:"assets"`
	Project   *WistiaRespVideoProject   `json:"project"`
}

type WistiaConf struct {
	WistiaApiKey string `json:"wistia_api_key"`
}

func (this *WistiaConf) MarginWithENV() *WistiaConf {
	if this.WistiaApiKey == "" {
		this.WistiaApiKey = os.Getenv("WISTIA_API_KEY")
	}

	return this
}

type WistiaHelper struct {
	Conf *WistiaConf
}

func NewWistiaHelper(conf *WistiaConf) *WistiaHelper {
	return &WistiaHelper{
		Conf: conf,
	}
}

func (this *WistiaHelper) GetVideoDetail(hashId string) (*WistiaRespVideo , error) {
	req, err := http.NewRequest( "GET", fmt.Sprintf("%s/medias/%s.json", WISTIA_API_ENDPOINT, hashId), nil)
	if err != nil {
		Log.Error(err)
		return nil, err
	}
	req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", this.Conf.WistiaApiKey))

	client := &http.Client{
		Timeout:   60 * time.Second, // 总体请求超时
	}
	resp, err := client.Do(req)
	if err != nil {
		Log.Error(err)
		return nil, err
	}
	defer resp.Body.Close()

	decoder := json.NewDecoder(resp.Body)
	videoResult := new(WistiaRespVideo)
	if err := decoder.Decode(&videoResult); err != nil {
		Log.Error(err)
		return nil, err
	}
	return videoResult, nil
}

func (this *WistiaHelper) MoveToS3(hashId string, conf *S3Config) (string, error) {
	video, err := this.GetVideoDetail(hashId)
	if err != nil {
		Log.Error(err)
		return "", err
	}
	wg := sync.WaitGroup{}

	storage, err := NewS3Storage(conf)
	if err != nil {
		Log.Error(err)
		return "", err
	}

	workerLimit := 3
	queue := make(chan bool, workerLimit)
	defer close(queue)

	for _, asset := range *video.Assets {
		wg.Add(1)

		go func(asset *WistiaRespVideoAsset, wg *sync.WaitGroup) {
			defer wg.Done()
			defer func() {
				<-queue
			}()
			queue <- true

			Log.Infof("Downloading [%s]\n", asset.Url)

			req, err := http.NewRequest( "GET", asset.Url, nil)
			client := &http.Client{
				Timeout:   60 * time.Second, // 总体请求超时
			}
			resp, err := client.Do(req)
			if err != nil {
				Log.Errorf("download [%s] Error: %s\n", asset.Url, err)
				return
			}
			defer resp.Body.Close()

			extension := ".bin"
			// get extension from asset.ContentType or mimeType
			extList, err := mime.ExtensionsByType(asset.ContentType)
			if err == nil && len(extList) > 0 {
				extension = extList[0]
			}
			if asset.ContentType == "image/jpg" {
				extension = ".jpg"
			}

			remoteKey := fmt.Sprintf("media/%s/%d%s", video.HashId, asset.Height, extension)
			if asset.Type == "StillImageFile" {
				remoteKey = fmt.Sprintf("media/%s/cover%s", video.HashId, extension)
			}
			if asset.Type == "OriginalFile" {
				remoteKey = fmt.Sprintf("media/%s/original%s", video.HashId, extension)
			}

			_, url, err := storage.PutStream(resp.Body, remoteKey, &UploadOptions{ContentType: asset.ContentType, PublicRead: true})
			if err != nil {
				Log.Errorf("upload [%s] Error: %s\n", asset.Url, err)
				return
			}

			Log.Infof("Uploaded %s to %s\n", asset.Url, url)

			asset.Url = url

		}(asset, &wg)
	}

	wg.Wait()

	remoteKey := fmt.Sprintf("media/%s/index.json", video.HashId)
	bin, err := json.Marshal(video)
	if err != nil {
		Log.Error(err)
		return "", err
	}
	_, url, err := storage.PutContent(string(bin), remoteKey, &UploadOptions{ContentType: "application/json", PublicRead: true})

	return url, nil
}
