package pkg

import (
	"testing"
	"wistia-s3/tests"
	_ "wistia-s3/tests"
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

	video, err := helper.MoveToS3("7bg0z4stnx", s3Conf)
	if err != nil {
		t.Error(err)
		t.Fail()
		return
	}

	t.Log(tests.ToJSON(video))

	t.Log("PASS")
}