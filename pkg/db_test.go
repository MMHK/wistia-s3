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

func TestDBHelper_FindVideoInfo(t *testing.T) {
	conf := new(Config)
	conf.MarginWithENV()

	dbHelper := NewDBHelper(conf.DBConf)

	video, err := dbHelper.FindVideoInfo("3rfrljngd1")
	if err != nil {
		t.Error(err)
		t.Fail()
		return
	}
	t.Log(tests.ToJSON(video))
}

func TestDBHelper_WistiaCatalog(t *testing.T) {
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

	err = dbHelper.SaveWistiaCatalogVideo(hashId, videoInfo)
	if err != nil {
		t.Error(err)
		t.Fail()
		return
	}
	t.Log("SaveWistiaCatalogVideo: OK")

	found, err := dbHelper.FindWistiaCatalogVideo(hashId)
	if err != nil {
		t.Error(err)
		t.Fail()
		return
	}
	if found.HashId != hashId {
		t.Errorf("expected hashId %s, got %s", hashId, found.HashId)
	}
	t.Logf("FindWistiaCatalogVideo: OK - %s", found.Name)

	all, err := dbHelper.GetAllWistiaCatalogVideos()
	if err != nil {
		t.Error(err)
		t.Fail()
		return
	}
	t.Logf("GetAllWistiaCatalogVideos: OK - count=%d", len(all))

	t.Log("PASS")
}

func TestDBHelper_WistiaSyncMeta(t *testing.T) {
	conf := new(Config)
	conf.MarginWithENV()

	dbHelper := NewDBHelper(conf.DBConf)

	meta := &WistiaSyncMeta{
		LastSyncAt: "2026-06-19T10:00:00Z",
		TotalCount: 100,
		PageCount:  2,
	}
	err := dbHelper.SaveWistiaSyncMeta(meta)
	if err != nil {
		t.Error(err)
		t.Fail()
		return
	}
	t.Log("SaveWistiaSyncMeta: OK")

	found, err := dbHelper.GetWistiaSyncMeta()
	if err != nil {
		t.Error(err)
		t.Fail()
		return
	}
	if found.TotalCount != 100 {
		t.Errorf("expected TotalCount 100, got %d", found.TotalCount)
	}
	if found.PageCount != 2 {
		t.Errorf("expected PageCount 2, got %d", found.PageCount)
	}
	t.Logf("GetWistiaSyncMeta: OK - lastSync=%s, total=%d", found.LastSyncAt, found.TotalCount)

	t.Log("PASS")
}
