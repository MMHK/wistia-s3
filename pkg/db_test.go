package pkg

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"path/filepath"
	"strings"
	"testing"
	"wistia-s3/tests"
	_ "wistia-s3/tests"
)

func TestBDHelper_SaveVideoInfo(t *testing.T) {
	conf := new(Config)
	conf.MarginWithENV()

	dbHelper := NewDBHelper(conf.DBConf)

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

func TestDBHelper_RefreshAllVideo(t *testing.T) {
	conf := new(Config)
	conf.MarginWithENV()

	s3, err := NewS3Storage(conf.Storage.S3)
	if err != nil {
		t.Error(err)
		t.Fail()
		return
	}
	files, err := s3.ListFiles("media")
	if err != nil {
		t.Error(err)
		t.Fail()
		return
	}

	dbHelper := NewDBHelper(conf.DBConf)

	for _, row := range files {
		if strings.HasSuffix(row, "index.json") {
			url := fmt.Sprintf("https://s3.%s.amazonaws.com/%s/%s", conf.Storage.S3.Region, conf.Storage.S3.Bucket, row)
			resp, err := http.Get(url)
			if err != nil {
				t.Error(err)
				continue
			}
			defer resp.Body.Close()

			hashId := filepath.Base(strings.Replace(row, "/index.json", "", 1))
			t.Log(hashId)
			err = dbHelper.SaveVideoInfo(hashId, resp.Body)
			if err != nil {
				t.Error(err)
				continue
			}
		}
	}
}

func TestBDHelper_GetAllVideoINfo(t *testing.T) {
	conf := new(Config)
	conf.MarginWithENV()

	dbHelper := NewDBHelper(conf.DBConf)

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
