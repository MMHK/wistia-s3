package pkg

import (
	"io"
	"os"
	"testing"
	"wistia-s3/tests"
)

func TestWistiaHelper_GetVideoDetail(t *testing.T) {
	conf := new(WistiaConf)
	conf.MarginWithENV()

	helper := NewWistiaHelper(conf)

	video, err := helper.GetVideoDetail("7bg0z4stnx")
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