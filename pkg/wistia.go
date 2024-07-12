package pkg

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"text/template"
	"time"
)

const WISTIA_API_ENDPOINT = "https://api.wistia.com/v1/"

type TemplateData struct {
	MediaEndPoint string
	VideoName     string
	WistiaS3JSUrl string
	HashId 	      string
}

type WistiaRespVideoAsset struct {
	Type        string `json:"type"`
	Url         string `json:"url"`
	FileSize    int    `json:"fileSize"`
	ContentType string `json:"contentType"`
	Width       int    `json:"width"`
	Height      int    `json:"height"`
	S3Key       string `json:"-"`
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
	WistiaApiKey    string `json:"wistia_api_key"`
	WorkerLimit     int    `json:"worker_limit"`
	TemplateDirPath string `json:"template_dir_path"`
}

func (this *WistiaConf) MarginWithENV() *WistiaConf {
	if this.WistiaApiKey == "" {
		this.WistiaApiKey = os.Getenv("WISTIA_API_KEY")
	}
	if this.WorkerLimit == 0 {
		this.WorkerLimit, _ = strconv.Atoi(os.Getenv("WISTIA_WORKER_LIMIT"))
	}
	if this.WorkerLimit == 0 {
		this.WorkerLimit = 3
	}

	if this.TemplateDirPath == "" {
		this.TemplateDirPath = os.Getenv("TEMPLATE_DIR_PATH")
	}

	return this
}

type WistiaHelper struct {
	Conf  *WistiaConf
	queue chan bool
}

func NewWistiaHelper(conf *WistiaConf) *WistiaHelper {
	return &WistiaHelper{
		Conf: conf,
		queue: make(chan bool, conf.WorkerLimit),
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

type DelimsOptions struct {
	Start string
	End   string
}

func (this *WistiaHelper) BuildTemplateWithDelims(filename string, data *TemplateData, delimiter *DelimsOptions) (io.Reader, error) {
	fileFullpath := filepath.ToSlash(filepath.Join(this.Conf.TemplateDirPath, filename));

	Log.Debugf("Template file: %s\n", fileFullpath)

	parser, err := template.ParseFiles(fileFullpath)
	if err != nil {
		Log.Error(err)
		return nil, err
	}

	if delimiter != nil {
		parser = parser.Delims(delimiter.Start, delimiter.End)
	}

	var buf bytes.Buffer
	err = parser.Execute(&buf, &data)
	if err != nil {
		Log.Error(err)
		return nil, err
	}

	return &buf, nil
}

func (this *WistiaHelper) BuildTemplate(filename string, data *TemplateData) (io.Reader, error) {
	return this.BuildTemplateWithDelims(filename, data, nil)
}

func (this *WistiaHelper) UploadWistiaS3JS(conf *S3Config) (string, string, error) {
	jsPath := "wistia-s3.min.js"

	storage, err := NewS3Storage(conf)
	if err != nil {
		Log.Error(err)
		return "", "", err
	}

	data := TemplateData{
		MediaEndPoint: fmt.Sprintf("https://s3.%s.amazonaws.com/%s/%s/media", conf.Region, conf.Bucket, conf.PrefixPath),
	}
	remoteKey := fmt.Sprintf("media/%s", jsPath)
	reader, err := this.BuildTemplate(jsPath, &data)
	if err != nil {
		Log.Error(err)
		return "", "", err
	}
	_, s3Url, err := storage.PutStream(reader, remoteKey, &UploadOptions{ContentType: "text/javascript", PublicRead: true})
	if err != nil {
		Log.Errorf("upload [%s] Error: %s\n", remoteKey, err)
		return "", "", err
	}
	Log.Infof("Uploaded %s to %s\n", jsPath, s3Url)

	if conf.UseCloudFront() {
		data.MediaEndPoint = fmt.Sprintf("https://%s/%s/cloudfront/media", conf.CloudFrontDomain, conf.PrefixPath)
		remoteKey = fmt.Sprintf("cloudfront/media/%s", jsPath)
		reader, err := this.BuildTemplate(jsPath, &data)
		if err != nil {
			Log.Error(err)
			return "", s3Url, err
		}
		_, cloudFrontUrl, err := storage.PutStream(reader, remoteKey, &UploadOptions{ContentType: "text/javascript", PublicRead: true})
		if err != nil {
			Log.Errorf("upload [%s] Error: %s\n", remoteKey, err)
			return "", "", err
		}
		cloudFrontUrl = fmt.Sprintf("https://%s/%s/cloudfront/media/%s", conf.CloudFrontDomain, conf.PrefixPath, jsPath)

		Log.Infof("Uploaded %s to %s\n", jsPath, cloudFrontUrl)

		return cloudFrontUrl, s3Url, nil
	}

	return "", s3Url, nil
}

func (this *WistiaHelper) UploadDemoPage(video *WistiaRespVideo, conf *S3Config, wg *sync.WaitGroup) (string, string, error) {
	if wg != nil {
		defer wg.Done()
	}
	defer func() {
		<-this.queue
	}()
	this.queue <- true

	data := TemplateData{
		HashId:        video.HashId,
		MediaEndPoint: fmt.Sprintf("https://s3.%s.amazonaws.com/%s/%s/media", conf.Region, conf.Bucket, conf.PrefixPath),
		VideoName:     video.Name,
		WistiaS3JSUrl: fmt.Sprintf("https://s3.%s.amazonaws.com/%s/%s/media/wistia-s3.min.js", conf.Region, conf.Bucket, conf.PrefixPath),
	}

	storage, err := NewS3Storage(conf)
	if err != nil {
		Log.Error(err)
		return "", "", err
	}

	tplName := "index.html"

	remoteKey := fmt.Sprintf("media/%s/%s", video.HashId, tplName)
	Log.Infof("generate %s => %s\n", tplName, remoteKey)
	reader, err := this.BuildTemplate(tplName, &data)
	if err != nil {
		Log.Error(err)
		return "", "", err
	}
	_, s3Url, err := storage.PutStream(reader, remoteKey, &UploadOptions{ContentType: "text/html", PublicRead: true})
	if err != nil {
		Log.Errorf("upload [%s] Error: %s\n", remoteKey, err)
		return "", "", err
	}
	Log.Infof("Uploaded %s to %s\n", tplName, s3Url)

	if conf.UseCloudFront() {
		if wg != nil {
			defer wg.Done()
		}
		defer func() {
			<-this.queue
		}()
		this.queue <- true

		remoteKey = fmt.Sprintf("cloudfront/media/%s/%s", video.HashId, tplName)
		data.MediaEndPoint = fmt.Sprintf("https://%s/%s/cloudfront/media", conf.CloudFrontDomain, conf.PrefixPath)
		data.WistiaS3JSUrl = fmt.Sprintf("https://%s/%s/cloudfront/media/wistia-s3.min.js", conf.CloudFrontDomain, conf.PrefixPath)

		Log.Infof("generate %s => %s\n", tplName, remoteKey)
		reader, err := this.BuildTemplate(tplName, &data)
		if err != nil {
			Log.Error(err)
			return "", s3Url, err
		}
		_, _, err = storage.PutStream(reader, remoteKey, &UploadOptions{ContentType: "text/html", PublicRead: true})
		if err != nil {
			Log.Errorf("upload [%s] Error: %s\n", remoteKey, err)
			return "", s3Url, err
		}
		cfUrl := fmt.Sprintf("https://%s/%s/cloudfront/media/%s/%s", conf.CloudFrontDomain, conf.PrefixPath, video.HashId, tplName)
		Log.Infof("Uploaded %s to %s\n", tplName, cfUrl)

		return cfUrl, s3Url, nil
	}

	return "", s3Url, nil
}

func (this *WistiaHelper) MoveToS3(hashId string, conf *S3Config) (string, string, error) {
	video, err := this.GetVideoDetail(hashId)
	if err != nil {
		Log.Error(err)
		return "", "", err
	}
	wg := sync.WaitGroup{}

	storage, err := NewS3Storage(conf)
	if err != nil {
		Log.Error(err)
		return "", "", err
	}

	for _, asset := range *video.Assets {
		wg.Add(1)

		go func(asset *WistiaRespVideoAsset, wg *sync.WaitGroup) {
			defer wg.Done()
			defer func() {
				<-this.queue
			}()
			this.queue <- true

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

			path, url, err := storage.PutStream(resp.Body, remoteKey, &UploadOptions{ContentType: asset.ContentType, PublicRead: true})
			if err != nil {
				Log.Errorf("upload [%s] Error: %s\n", asset.Url, err)
				return
			}

			asset.S3Key = path

			Log.Infof("Uploaded %s to %s\n", asset.Url, url)

			asset.Url = url

		}(asset, &wg)
	}

	counter := 1
	if conf.UseCloudFront() {
		counter = 2
	}
	wg.Add(counter)
	go this.UploadDemoPage(video, conf, &wg)

	wg.Wait()

	// S3 endpoint
	Log.Debug("upload s3 index file")
	remoteKey := fmt.Sprintf("media/%s/index.json", video.HashId)
	bin, err := json.Marshal(video)
	if err != nil {
		Log.Error(err)
		return "", "", err
	}
	_, s3Url, err := storage.PutContent(string(bin), remoteKey, &UploadOptions{ContentType: "application/json", PublicRead: true})
	if err != nil {
		Log.Error(err)
		return "", "", err
	}

	cloudFrontUrl := ""
	if conf.UseCloudFront() {
		Log.Debug("upload cloudfront index file")
		for _, asset := range *video.Assets {
			asset.Url = fmt.Sprintf("https://%s/%s", conf.CloudFrontDomain, strings.TrimLeft(asset.S3Key, "/"))
		}
		bin, err := json.Marshal(video)
		if err != nil {
			Log.Error(err)
			return "", s3Url, err
		}
		remoteKey = fmt.Sprintf("cloudfront/media/%s/index.json", video.HashId)
		path, _, err := storage.PutContent(string(bin), remoteKey, &UploadOptions{ContentType: "application/json", PublicRead: true})
		if err != nil {
			Log.Error(err)
			return "", s3Url, err
		}
		cloudFrontUrl = fmt.Sprintf("https://%s/%s", conf.CloudFrontDomain, strings.TrimLeft(path, "/"))
	}

	return cloudFrontUrl, s3Url, nil
}
