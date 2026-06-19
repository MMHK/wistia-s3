package pkg

import (
	"testing"
	"wistia-s3/tests"
)

func TestE2E_FindNotUploadedVideos(t *testing.T) {
	conf := new(Config)
	conf.MarginWithENV()

	wistiaHelper := NewWistiaHelper(conf.WistiaConf)
	wistiaVideos, err := wistiaHelper.ListAllVideos()
	if err != nil {
		t.Error(err)
		t.Fail()
		return
	}

	dbHelper := NewDBHelper(conf.DBConf)
	migratedVideos, err := dbHelper.GetAllVideoInfo()
	if err != nil {
		t.Error(err)
		t.Fail()
		return
	}

	migratedSet := make(map[string]bool)
	for _, v := range migratedVideos {
		migratedSet[v.HashId] = true
	}

	var notMigrated []*WistiaRespVideo
	for _, v := range wistiaVideos {
		if !migratedSet[v.HashId] {
			notMigrated = append(notMigrated, v)
		}
	}

	for _, v := range notMigrated {
		t.Logf("hashId=%s name=%s status=%s archived=%v duration=%.2f created=%s",
			v.HashId, v.Name, v.Status, v.Archived, v.Duration, v.Created)
	}

	t.Logf("Total Wistia videos: %d", len(wistiaVideos))
	t.Logf("Migrated (in BoltDB): %d", len(migratedVideos))
	t.Logf("Not uploaded: %d", len(notMigrated))
	t.Log(tests.ToJSON(notMigrated))
	t.Log("PASS")
}
