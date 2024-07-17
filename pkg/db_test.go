package pkg

import (
	"bytes"
	"encoding/json"
	"testing"
	"wistia-s3/tests"
	_ "wistia-s3/tests"
)

func TestBDHelper_SaveVideoInfo(t *testing.T) {
	conf := new(Config)
	conf.MarginWithENV()

	dbHelper := NewBDHelper(conf.DBConf)

	wistiaHelper := NewWistiaHelper(conf.WistiaConf)

	hashId := "3rfrljngd1"

	videoInfo, err := wistiaHelper.GetVideoDetail(hashId)
	if err != nil {
		t.Error(err)
		t.Fail()
		return
	}

	bin, err := json.Marshal(videoInfo)
	if err != nil {
		t.Error(err)
		t.Fail()
		return
	}
	reader := bytes.NewReader(bin)

	err = dbHelper.SaveVideoInfo(hashId, reader)
	if err != nil {
		t.Error(err)
		t.Fail()
		return
	}
}

func TestBDHelper_GetAllVideoINfo(t *testing.T) {
	conf := new(Config)
	conf.MarginWithENV()

	dbHelper := NewBDHelper(conf.DBConf)

	videos, err := dbHelper.GetAllVideoInfo()
	if err != nil {
		t.Error(err)
		t.Fail()
		return
	}

	for _, row := range videos {
		t.Log(tests.ToJSON(row))
	}
}
