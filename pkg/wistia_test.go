package pkg

import (
	"encoding/json"
	"io"
	"mime"
	"os"
	"testing"
	"wistia-s3/tests"
)

func TestWistiaHelper_GetVideoDetail(t *testing.T) {
	conf := new(WistiaConf)
	conf.MarginWithENV()

	helper := NewWistiaHelper(conf)

	video, err := helper.GetVideoDetail("253ufvw2pf")
	if err != nil {
		t.Error(err)
		t.Fail()
		return
	}

	t.Log(tests.ToJSON(video))

	t.Log(tests.ToJSON(video.Assets.GetCover()))

	t.Log(tests.ToJSON(video.Assets.GetOriginal()))

	t.Log(tests.ToJSON(video.Assets.GetVideoFiles()))

	t.Log("PASS")
}

func TestNewWistiaHelper_MimeDetect(t *testing.T) {
	mimeType := "video/mp4"
	extList, err := mime.ExtensionsByType(mimeType)
	if err != nil {
		t.Error(err)
		t.Fail()
		return
	}
	t.Log(extList)
}

func TestWistiaHelper_MoveToS3(t *testing.T) {
	conf := new(WistiaConf)
	conf.MarginWithENV()
	helper := NewWistiaHelper(conf)
	s3Conf := loadS3Config()

	cloudFrontJson, s3Json, err := helper.MoveToS3("u7k1cgyjy0", s3Conf)
	if err != nil {
		t.Error(err)
		t.Fail()
		return
	}

	t.Logf("cloudfront: %s\n", cloudFrontJson)
	t.Logf("s3: %s\n", s3Json)

	t.Log("PASS")
}

func TestWistiaHelper_BuildTemplate(t *testing.T) {
	conf := new(WistiaConf)
	conf.MarginWithENV()
	helper := NewWistiaHelper(conf)

	data := &TemplateData{
		MediaEndPoint: "https://media.wistia.com/medias/",
		VideoName:     "Video Demo Name",
	}

	reader, err := helper.BuildTemplate("index.html", data)
	if err != nil {
		t.Error(err)
		t.Fail()
		return
	}

	tmp, err := os.CreateTemp("", "*.html")
	if err != nil {
		t.Error(err)
		t.Fail()
		return
	}
	defer os.Remove(tmp.Name())
	_, err = io.Copy(tmp, reader)
	if err != nil {
		t.Error(err)
		t.Fail()
		return
	}

	t.Log(tmp.Name())
	t.Log("PASS")
}

func TestWistiaHelper_BuildTemplateWithDelims(t *testing.T) {
	conf := new(WistiaConf)
	conf.MarginWithENV()
	helper := NewWistiaHelper(conf)

	data := &TemplateData{
		MediaEndPoint: "https://media.wistia.com/medias/",
	}

	opt := &DelimsOptions{Start: "{{", End: "}}"}
	reader, err := helper.BuildTemplateWithDelims("wistia-s3.min.js", data, opt)
	if err != nil {
		t.Error(err)
		t.Fail()
		return
	}

	tmp, err := os.CreateTemp("", "*.js")
	if err != nil {
		t.Error(err)
		t.Fail()
		return
	}
	//defer os.Remove(tmp.Name())
	_, err = io.Copy(tmp, reader)
	if err != nil {
		t.Error(err)
		t.Fail()
		return
	}

	t.Log(tmp.Name())
	t.Log("PASS")
}

func TestWistiaHelper_UploadWistiaS3JS(t *testing.T) {
	conf := new(WistiaConf)
	conf.MarginWithENV()
	helper := NewWistiaHelper(conf)
	s3Conf := loadS3Config()

	cfUrl, s3Url, err := helper.UploadWistiaS3JS(s3Conf)
	if err != nil {
		t.Error(err)
		t.Fail()
		return
	}
	t.Logf("cloudfront: %s\n", cfUrl)
	t.Logf("s3: %s\n", s3Url)
	t.Log("PASS")
}

func TestWistiaHelper_UploadDemoPage(t *testing.T) {
	conf := new(WistiaConf)
	conf.MarginWithENV()
	helper := NewWistiaHelper(conf)
	s3Conf := loadS3Config()

	videoData := new(WistiaRespVideo)

	raw := `{
    "name": "TrainingVideo_PGB01_14082023",
    "id": 108341755,
    "hashed_id": "u7k1cgyjy0",
    "duration": 177.842,
    "status": "ready",
    "progress": 1,
    "archived": false,
    "section": "Zurich Training Video (For Training Site)",
    "thumbnail": {
        "url": "https://embed-ssl.wistia.com/deliveries/f61380909a61d92b56c60a7ebb95328feb4d91f0.jpg?image_crop_resized=200x120",
        "width": 200,
        "height": 120
    },
    "assets": [
        {
            "type": "OriginalFile",
            "url": "https://s3.ap-southeast-1.amazonaws.com/s3.test.mixmedia.com/wistia-backup/media/u7k1cgyjy0/original.mp4",
            "fileSize": 112194856,
            "contentType": "video/mp4",
            "width": 1920,
            "height": 1080
        },
        {
            "type": "IphoneVideoFile",
            "url": "https://s3.ap-southeast-1.amazonaws.com/s3.test.mixmedia.com/wistia-backup/media/u7k1cgyjy0/360.mp4",
            "fileSize": 6767304,
            "contentType": "video/mp4",
            "width": 640,
            "height": 360
        },
        {
            "type": "Mp4VideoFile",
            "url": "https://s3.ap-southeast-1.amazonaws.com/s3.test.mixmedia.com/wistia-backup/media/u7k1cgyjy0/224.mp4",
            "fileSize": 4861043,
            "contentType": "video/mp4",
            "width": 400,
            "height": 224
        },
        {
            "type": "MdMp4VideoFile",
            "url": "https://s3.ap-southeast-1.amazonaws.com/s3.test.mixmedia.com/wistia-backup/media/u7k1cgyjy0/540.mp4",
            "fileSize": 9299596,
            "contentType": "video/mp4",
            "width": 960,
            "height": 540
        },
        {
            "type": "HdMp4VideoFile",
            "url": "https://s3.ap-southeast-1.amazonaws.com/s3.test.mixmedia.com/wistia-backup/media/u7k1cgyjy0/720.mp4",
            "fileSize": 12515969,
            "contentType": "video/mp4",
            "width": 1280,
            "height": 720
        },
        {
            "type": "HdMp4VideoFile",
            "url": "https://s3.ap-southeast-1.amazonaws.com/s3.test.mixmedia.com/wistia-backup/media/u7k1cgyjy0/1080.mp4",
            "fileSize": 19703676,
            "contentType": "video/mp4",
            "width": 1920,
            "height": 1080
        },
        {
            "type": "StillImageFile",
            "url": "https://s3.ap-southeast-1.amazonaws.com/s3.test.mixmedia.com/wistia-backup/media/u7k1cgyjy0/cover.jpg",
            "fileSize": 381245,
            "contentType": "image/jpg",
            "width": 1920,
            "height": 1080
        },
        {
            "type": "StoryboardFile",
            "url": "https://s3.ap-southeast-1.amazonaws.com/s3.test.mixmedia.com/wistia-backup/media/u7k1cgyjy0/2260.jpg",
            "fileSize": 1080266,
            "contentType": "image/jpg",
            "width": 2000,
            "height": 2260
        }
    ],
    "project": {
        "name": "SpeedyAgency",
        "id": 2637588,
        "hashed_id": "wvgixiqhom"
    }
}`
	err := json.Unmarshal([]byte(raw), videoData)
	if err != nil {
		t.Error(err)
		t.Fail()
		return
	}

	cfUrl, s3Url, err := helper.UploadDemoPage("index.html", videoData, s3Conf, nil)
	if err != nil {
		t.Error(err)
		t.Fail()
		return
	}
	t.Logf("cloudfront: %s\n", cfUrl)
	t.Logf("s3: %s\n", s3Url)

	cfUrl, s3Url, err = helper.UploadDemoPage("demo.html", videoData, s3Conf, nil)
	if err != nil {
		t.Error(err)
		t.Fail()
		return
	}
	t.Logf("cloudfront: %s\n", cfUrl)
	t.Logf("s3: %s\n", s3Url)

	t.Log("PASS")
}

func TestWistiaHelper_reUploadAllDemoPage(t *testing.T) {
	conf := new(WistiaConf)
	conf.MarginWithENV()
	helper := NewWistiaHelper(conf)
	s3Conf := loadS3Config()

	type videoList struct {
		Data []*WistiaRespVideo `json:"data"`
	}
	var list videoList

	bin, err := os.ReadFile(os.Getenv("ALL_VIDEO_JSON"))
	if err != nil {
		t.Error(err)
		t.Fail()
		return
	}

	err = json.Unmarshal(bin, &list)
	if err != nil {
		t.Error(err)
		t.Fail()
		return
	}

	for _, videoData := range list.Data {
		cfUrl, s3Url, err := helper.UploadDemoPage("index.html", videoData, s3Conf, nil)
		if err != nil {
			t.Error(err)
			continue
		}
		t.Logf("cloudfront: %s\n", cfUrl)
		t.Logf("s3: %s\n", s3Url)

		cfUrl, s3Url, err = helper.UploadDemoPage("demo.html", videoData, s3Conf, nil)
		if err != nil {
			t.Error(err)
			continue
		}
		t.Logf("cloudfront: %s\n", cfUrl)
		t.Logf("s3: %s\n", s3Url)
	}

	t.Log("PASS")
}

func TestWistiaHelper_GenerateVideoInfoURL(t *testing.T) {
	conf := new(WistiaConf)
	conf.MarginWithENV()

	s3Conf := loadS3Config()

	helper := NewWistiaHelper(conf)

	cloudfrontJson, s3Json := helper.GenerateVideoInfoURL("7bg0z4stnx", s3Conf)
	t.Log(cloudfrontJson)
	t.Log(s3Json)
}

func TestWistiaHelper_ListAllVideos(t *testing.T) {
	conf := new(WistiaConf)
	conf.MarginWithENV()

	helper := NewWistiaHelper(conf)

	videos, err := helper.ListAllVideos()
	if err != nil {
		t.Error(err)
		t.Fail()
		return
	}

	t.Logf("Total videos fetched: %d", len(videos))

	for i, v := range videos {
		if i >= 3 {
			break
		}
		t.Logf("Video %d: hash=%s, name=%s, archived=%v", i, v.HashId, v.Name, v.Archived)
	}

	t.Log("PASS")
}

func TestWistiaHelper_ArchiveVideos(t *testing.T) {
	conf := new(WistiaConf)
	conf.MarginWithENV()
	helper := NewWistiaHelper(conf)

	err := helper.ArchiveVideos([]string{"thm6u6imgj"})
	if err != nil {
		t.Error(err)
		t.Fail()
		return
	}
	t.Log("PASS")
}
