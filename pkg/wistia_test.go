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
		VideoName: "Video Demo Name",
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

	raw := `{"name":"TrainingVideo_ZDH02_04072024","id":121310790,"hashed_id":"7bg0z4stnx","duration":161.16,"status":"ready","progress":1,"archived":false,"section":"Zurich Training Video (For Training Site)","thumbnail":{"url":"https://embed-ssl.wistia.com/deliveries/0fe9093d49c2f11336c99ccedd1b34a4c98acfc2.jpg?image_crop_resized=200x120","width":200,"height":120},"assets":[{"type":"OriginalFile","url":"https://s3.ap-southeast-1.amazonaws.com/s3.test.mixmedia.com/wistia-backup/media/7bg0z4stnx/original.mp4","fileSize":101105785,"contentType":"video/mp4","width":1920,"height":1080},{"type":"IphoneVideoFile","url":"https://s3.ap-southeast-1.amazonaws.com/s3.test.mixmedia.com/wistia-backup/media/7bg0z4stnx/360.mp4","fileSize":5330201,"contentType":"video/mp4","width":640,"height":360},{"type":"Mp4VideoFile","url":"https://s3.ap-southeast-1.amazonaws.com/s3.test.mixmedia.com/wistia-backup/media/7bg0z4stnx/224.mp4","fileSize":4078427,"contentType":"video/mp4","width":400,"height":224},{"type":"MdMp4VideoFile","url":"https://s3.ap-southeast-1.amazonaws.com/s3.test.mixmedia.com/wistia-backup/media/7bg0z4stnx/540.mp4","fileSize":7401440,"contentType":"video/mp4","width":960,"height":540},{"type":"HdMp4VideoFile","url":"https://s3.ap-southeast-1.amazonaws.com/s3.test.mixmedia.com/wistia-backup/media/7bg0z4stnx/720.mp4","fileSize":9561403,"contentType":"video/mp4","width":1280,"height":720},{"type":"HdMp4VideoFile","url":"https://s3.ap-southeast-1.amazonaws.com/s3.test.mixmedia.com/wistia-backup/media/7bg0z4stnx/1080.mp4","fileSize":14841661,"contentType":"video/mp4","width":1920,"height":1080},{"type":"StillImageFile","url":"https://s3.ap-southeast-1.amazonaws.com/s3.test.mixmedia.com/wistia-backup/media/7bg0z4stnx/cover.jpg","fileSize":567309,"contentType":"image/jpg","width":1920,"height":1080},{"type":"StoryboardFile","url":"https://s3.ap-southeast-1.amazonaws.com/s3.test.mixmedia.com/wistia-backup/media/7bg0z4stnx/2260.jpg","fileSize":816443,"contentType":"image/jpg","width":2000,"height":2260}],"project":{"name":"SpeedyAgency","id":2637588,"hashed_id":"wvgixiqhom"}}`
	err := json.Unmarshal([]byte(raw), videoData)
	if err != nil {
		t.Error(err)
		t.Fail()
		return
	}

	cfUrl, s3Url, err := helper.UploadDemoPage(videoData, s3Conf, nil)
	if err != nil {
		t.Error(err)
		t.Fail()
		return
	}
	t.Logf("cloudfront: %s\n", cfUrl)
	t.Logf("s3: %s\n", s3Url)
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