package pkg

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/url"
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
	TrackingID	  string
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
	Created   string                    `json:"created"`
	Thumbnail *WistiaRespVideoThumbnail `json:"thumbnail"`
	Assets    *AssetList                `json:"assets"`
	Project   *WistiaRespVideoProject   `json:"project"`
}

type WistiaConf struct {
	WistiaApiKey    string `json:"wistia_api_key"`
	WorkerLimit     int    `json:"worker_limit"`
	TemplateDirPath string `json:"template_dir_path"`
	GATrackingId    string `json:"ga_tracking_id"`
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

	if len(this.GATrackingId) <= 0 {
		this.GATrackingId = os.Getenv("GA_TRACKING_ID")
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
		Log.Error("failed to create Wistia API request", "error", err, "hash", hashId)
		return nil, err
	}
	req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", this.Conf.WistiaApiKey))

	client := &http.Client{
		Timeout:   60 * time.Second,
	}
	resp, err := client.Do(req)
	if err != nil {
		Log.Error("failed to execute Wistia API request", "error", err, "hash", hashId)
		return nil, err
	}
	defer resp.Body.Close()

	decoder := json.NewDecoder(resp.Body)
	videoResult := new(WistiaRespVideo)
	if err := decoder.Decode(&videoResult); err != nil {
		Log.Error("failed to decode Wistia API response", "error", err, "hash", hashId)
		return nil, err
	}
	return videoResult, nil
}

func (this *WistiaHelper) ListAllVideos() ([]*WistiaRespVideo, error) {
	const perPage = 50
	allVideos := make([]*WistiaRespVideo, 0)
	page := 1

	client := &http.Client{
		Timeout: 60 * time.Second,
	}

	for {
		this.queue <- true

		baseURL := fmt.Sprintf("%s/medias.json", WISTIA_API_ENDPOINT)
		u, err := url.Parse(baseURL)
		if err != nil {
			<-this.queue
			Log.Error("failed to parse Wistia list API URL", "error", err, "page", page)
			return allVideos, err
		}

		query := u.Query()
		query.Add("page", strconv.Itoa(page))
		query.Add("per_page", strconv.Itoa(perPage))
		u.RawQuery = query.Encode()

		Log.Info("fetching Wistia video list", "page", page, "per_page", perPage)

		req, err := http.NewRequest("GET", u.String(), nil)
		if err != nil {
			<-this.queue
			Log.Error("failed to create Wistia list API request", "error", err, "page", page)
			return allVideos, err
		}
		req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", this.Conf.WistiaApiKey))

		resp, err := client.Do(req)
		if err != nil {
			<-this.queue
			Log.Error("failed to execute Wistia list API request", "error", err, "page", page)
			return allVideos, err
		}

		if resp.StatusCode != 200 {
			body, _ := io.ReadAll(resp.Body)
			resp.Body.Close()
			<-this.queue
			Log.Error("Wistia list API returned non-200", "status", resp.StatusCode, "page", page, "body", string(body))
			return allVideos, fmt.Errorf("Wistia API returned status %d: %s", resp.StatusCode, string(body))
		}

		var pageVideos []*WistiaRespVideo
		decoder := json.NewDecoder(resp.Body)
		if err := decoder.Decode(&pageVideos); err != nil {
			resp.Body.Close()
			<-this.queue
			Log.Error("failed to decode Wistia list API response", "error", err, "page", page)
			return allVideos, err
		}
		resp.Body.Close()
		<-this.queue

		Log.Info("fetched Wistia video list page", "page", page, "count", len(pageVideos), "total", len(allVideos)+len(pageVideos))

		allVideos = append(allVideos, pageVideos...)

		if len(pageVideos) < perPage {
			break
		}
		page++
	}

	Log.Info("finished fetching all Wistia videos", "total", len(allVideos))
	return allVideos, nil
}

func (this *WistiaHelper) ArchiveVideos(videoHashList []string) error {
	baseURL := fmt.Sprintf("%s/medias/archive.json", WISTIA_API_ENDPOINT)
	u, err := url.Parse(baseURL)
	if err != nil {
		Log.Error("failed to parse archive API URL", "error", err)
		return err
	}

	query := u.Query()
	for _, hashId := range videoHashList {
		query.Add("hashed_ids[]", hashId)
	}

	u.RawQuery = query.Encode()

	Log.Debug("archive API request URL", "url", u.String(), "count", len(videoHashList))

	req, err := http.NewRequest( "PUT", u.String(), nil)
	if err != nil {
		Log.Error("failed to create archive API request", "error", err)
		return err
	}
	req.Header.Add("Accept", "application/json")
	req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", this.Conf.WistiaApiKey))

	client := &http.Client{
		Timeout:   60 * time.Second,
	}
	resp, err := client.Do(req)
	if err != nil {
		Log.Error("failed to execute archive API request", "error", err)
		return err
	}
	defer resp.Body.Close()

	bin, err := io.ReadAll(resp.Body)
	if err != nil {
		Log.Error("failed to read archive API response", "error", err)
		return err
	}

	Log.Debug("archive API response", "body", string(bin), "status", resp.StatusCode)

	if resp.StatusCode != 200 {
		return errors.New(string((bin)))
	}


	return nil
}

type DelimsOptions struct {
	Start string
	End   string
}

func (this *WistiaHelper) BuildTemplateWithDelims(filename string, data *TemplateData, delimiter *DelimsOptions) (io.Reader, error) {
	fileFullpath := filepath.ToSlash(filepath.Join(this.Conf.TemplateDirPath, filename));

	Log.Debug("parsing template file", "path", fileFullpath)

	parser, err := template.ParseFiles(fileFullpath)
	if err != nil {
		Log.Error("failed to parse template file", "error", err, "path", fileFullpath)
		return nil, err
	}

	if delimiter != nil {
		parser = parser.Delims(delimiter.Start, delimiter.End)
	}

	var buf bytes.Buffer
	err = parser.Execute(&buf, &data)
	if err != nil {
		Log.Error("failed to execute template", "error", err, "path", fileFullpath)
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
		Log.Error("failed to create S3 storage for player JS upload", "error", err)
		return "", "", err
	}

	data := TemplateData{
		MediaEndPoint: fmt.Sprintf("https://s3.%s.amazonaws.com/%s/%s/media", conf.Region, conf.Bucket, conf.PrefixPath),
		TrackingID: this.Conf.GATrackingId,
	}
	remoteKey := fmt.Sprintf("media/%s", jsPath)
	reader, err := this.BuildTemplate(jsPath, &data)
	if err != nil {
		Log.Error("failed to build player JS template for S3", "error", err, "key", remoteKey)
		return "", "", err
	}
	_, s3Url, err := storage.PutStream(reader, remoteKey, &UploadOptions{ContentType: "text/javascript", PublicRead: true})
	if err != nil {
		Log.Error("failed to upload player JS to S3", "error", err, "key", remoteKey)
		return "", "", err
	}
	Log.Info("uploaded player JS to S3", "file", jsPath, "url", s3Url)

	if conf.UseCloudFront() {
		data.MediaEndPoint = fmt.Sprintf("https://%s/%s/cloudfront/media", conf.CloudFrontDomain, conf.PrefixPath)
		remoteKey = fmt.Sprintf("cloudfront/media/%s", jsPath)
		reader, err := this.BuildTemplate(jsPath, &data)
		if err != nil {
			Log.Error("failed to build player JS template for CloudFront", "error", err, "key", remoteKey)
			return "", s3Url, err
		}
		_, cloudFrontUrl, err := storage.PutStream(reader, remoteKey, &UploadOptions{ContentType: "text/javascript", PublicRead: true})
		if err != nil {
			Log.Error("failed to upload player JS to CloudFront", "error", err, "key", remoteKey)
			return "", "", err
		}
		cloudFrontUrl = fmt.Sprintf("https://%s/%s/cloudfront/media/%s", conf.CloudFrontDomain, conf.PrefixPath, jsPath)

		Log.Info("uploaded player JS to CloudFront", "file", jsPath, "url", cloudFrontUrl)

		cfHelper := NewCloudFrontHelper(conf)
		if cfHelper != nil {
			flushPaths := []string{fmt.Sprintf("/%s/cloudfront/media/wistia-s3.min.js", conf.PrefixPath)}
			if err := cfHelper.InvalidatePaths(flushPaths); err != nil {
				Log.Warn("CloudFront invalidation failed for player JS", "error", err, "file", jsPath)
			}
		}

		return cloudFrontUrl, s3Url, nil
	}

	return "", s3Url, nil
}

func (this *WistiaHelper) UploadDemoPage(tplName string, video *WistiaRespVideo, conf *S3Config, wg *sync.WaitGroup) (string, string, error) {
	data := TemplateData{
		HashId:        video.HashId,
		MediaEndPoint: fmt.Sprintf("https://s3.%s.amazonaws.com/%s/%s/media", conf.Region, conf.Bucket, conf.PrefixPath),
		VideoName:     video.Name,
		WistiaS3JSUrl: fmt.Sprintf("https://s3.%s.amazonaws.com/%s/%s/media/wistia-s3.min.js", conf.Region, conf.Bucket, conf.PrefixPath),
	}

	storage, err := NewS3Storage(conf)
	if err != nil {
		Log.Error("failed to create S3 storage for demo page", "error", err, "hash", video.HashId, "template", tplName)
		return "", "", err
	}

	if wg != nil {
		defer wg.Done()
	}
	defer func() {
		<-this.queue
	}()
	this.queue <- true

	remoteKey := fmt.Sprintf("media/%s/%s", video.HashId, tplName)
	Log.Info("generating page from template", "template", tplName, "key", remoteKey, "hash", video.HashId)
	reader, err := this.BuildTemplate(tplName, &data)
	if err != nil {
		Log.Error("failed to build page template for S3", "error", err, "template", tplName, "hash", video.HashId)
		return "", "", err
	}
	_, s3Url, err := storage.PutStream(reader, remoteKey, &UploadOptions{ContentType: "text/html", PublicRead: true})
	if err != nil {
		Log.Error("failed to upload page to S3", "error", err, "key", remoteKey, "template", tplName)
		return "", "", err
	}
	Log.Info("uploaded page to S3", "template", tplName, "url", s3Url, "hash", video.HashId)

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

		Log.Info("generating CloudFront page from template", "template", tplName, "key", remoteKey, "hash", video.HashId)
		reader, err := this.BuildTemplate(tplName, &data)
		if err != nil {
			Log.Error("failed to build page template for CloudFront", "error", err, "template", tplName, "hash", video.HashId)
			return "", s3Url, err
		}
		_, _, err = storage.PutStream(reader, remoteKey, &UploadOptions{ContentType: "text/html", PublicRead: true})
		if err != nil {
			Log.Error("failed to upload page to CloudFront", "error", err, "key", remoteKey, "template", tplName)
			return "", s3Url, err
		}
		cfUrl := fmt.Sprintf("https://%s/%s/cloudfront/media/%s/%s", conf.CloudFrontDomain, conf.PrefixPath, video.HashId, tplName)
		Log.Info("uploaded page to CloudFront", "template", tplName, "url", cfUrl, "hash", video.HashId)

		return cfUrl, s3Url, nil
	}

	return "", s3Url, nil
}

func (this *WistiaHelper) MoveToS3(hashId string, conf *S3Config) (string, string, error) {
	video, err := this.GetVideoDetail(hashId)
	if err != nil {
		Log.Error("failed to get video details for migration", "error", err, "hash", hashId)
		return "", "", err
	}
	wg := sync.WaitGroup{}

	storage, err := NewS3Storage(conf)
	if err != nil {
		Log.Error("failed to create S3 storage for migration", "error", err, "hash", hashId)
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

			Log.Info("downloading video asset", "url", asset.Url, "type", asset.Type, "height", asset.Height, "hash", video.HashId)

			req, err := http.NewRequest( "GET", asset.Url, nil)
			client := &http.Client{
				Timeout:   60 * time.Second,
			}
			resp, err := client.Do(req)
			if err != nil {
				Log.Error("failed to download video asset", "error", err, "url", asset.Url, "type", asset.Type, "hash", video.HashId)
				return
			}
			defer resp.Body.Close()

			extension := ".bin"
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
				Log.Error("failed to upload video asset to S3", "error", err, "url", asset.Url, "key", remoteKey, "type", asset.Type, "hash", video.HashId)
				return
			}

			asset.S3Key = path

			Log.Info("uploaded video asset to S3", "url", url, "key", remoteKey, "type", asset.Type, "hash", video.HashId)

			asset.Url = url

		}(asset, &wg)
	}

	counter := 2
	if conf.UseCloudFront() {
		counter = 4
	}
	wg.Add(counter)
	go this.UploadDemoPage("index.html", video, conf, &wg)
	go this.UploadDemoPage("demo.html", video, conf, &wg)

	wg.Wait()

	// S3 endpoint
	Log.Debug("uploading S3 index.json", "hash", video.HashId)
	remoteKey := fmt.Sprintf("media/%s/index.json", video.HashId)
	bin, err := json.Marshal(video)
	if err != nil {
		Log.Error("failed to marshal video metadata for S3 index", "error", err, "hash", video.HashId)
		return "", "", err
	}
	_, s3Url, err := storage.PutContent(string(bin), remoteKey, &UploadOptions{ContentType: "application/json", PublicRead: true})
	if err != nil {
		Log.Error("failed to upload S3 index.json", "error", err, "key", remoteKey, "hash", video.HashId)
		return "", "", err
	}
	Log.Debug("uploaded S3 index.json", "key", remoteKey, "url", s3Url, "hash", video.HashId)

	cloudFrontUrl := ""
	if conf.UseCloudFront() {
		Log.Debug("uploading CloudFront index.json", "hash", video.HashId)
		for _, asset := range *video.Assets {
			asset.Url = fmt.Sprintf("https://%s/%s", conf.CloudFrontDomain, strings.TrimLeft(asset.S3Key, "/"))
		}
		bin, err := json.Marshal(video)
		if err != nil {
			Log.Error("failed to marshal video metadata for CloudFront index", "error", err, "hash", video.HashId)
			return "", s3Url, err
		}
		remoteKey = fmt.Sprintf("cloudfront/media/%s/index.json", video.HashId)
		path, _, err := storage.PutContent(string(bin), remoteKey, &UploadOptions{ContentType: "application/json", PublicRead: true})
		if err != nil {
			Log.Error("failed to upload CloudFront index.json", "error", err, "key", remoteKey, "hash", video.HashId)
			return "", s3Url, err
		}
		cloudFrontUrl = fmt.Sprintf("https://%s/%s", conf.CloudFrontDomain, strings.TrimLeft(path, "/"))
		Log.Debug("uploaded CloudFront index.json", "key", remoteKey, "url", cloudFrontUrl, "hash", video.HashId)

		cfHelper := NewCloudFrontHelper(conf)
		if cfHelper != nil {
			flushPaths := []string{fmt.Sprintf("/%s/cloudfront/media/%s/*", conf.PrefixPath, hashId)}
			if err := cfHelper.InvalidatePaths(flushPaths); err != nil {
				Log.Warn("CloudFront invalidation failed for video", "error", err, "hash", hashId)
			}
		}
	}

	return cloudFrontUrl, s3Url, nil
}

func (this *WistiaHelper) GenerateVideoInfoURL(hashId string, conf *S3Config) (string, string) {
	return fmt.Sprintf("https://%s/%s/cloudfront/media/%s/index.json", conf.CloudFrontDomain, conf.PrefixPath, hashId),
		fmt.Sprintf("https://s3.%s.amazonaws.com/%s/%s/media/%s/index.json", conf.Region, conf.Bucket, conf.PrefixPath, hashId)
}
