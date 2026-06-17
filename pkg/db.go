package pkg

import (
	"encoding/json"
	"fmt"
	"github.com/boltdb/bolt"
	"io"
)


type DBConfig struct {
	FilePath string
}

type DBHelper struct {
	Conf *DBConfig
}

func NewDBHelper(conf *DBConfig) *DBHelper {
	return &DBHelper{
		Conf: conf,
	}
}

func (this *DBHelper) SaveVideoInfo (hashId string, r io.Reader) error {
	db, err := bolt.Open(this.Conf.FilePath, 0600, nil)
	if err != nil {
		Log.Error("failed to open BoltDB for SaveVideoInfo", "error", err, "path", this.Conf.FilePath, "hash", hashId)
		return err
	}
	defer db.Close()

	err = db.Update(func(tx *bolt.Tx) error {
		bucket, err := tx.CreateBucketIfNotExists([]byte("media"))
		if err != nil {
			Log.Error("failed to create media bucket", "error", err, "hash", hashId)
			return err
		}
		bin, err := io.ReadAll(r)
		if err != nil {
			Log.Error("failed to read video info data", "error", err, "hash", hashId)
			return err
		}
		err = bucket.Put([]byte(hashId), bin)
		if err != nil {
			Log.Error("failed to put video info into media bucket", "error", err, "hash", hashId)
			return err
		}
		return nil
	})
	if err != nil {
		Log.Error("SaveVideoInfo transaction failed", "error", err, "hash", hashId)
		return err
	}

	return nil
}

func (this *DBHelper) GetAllVideoInfo() ([]*WistiaRespVideo, error) {
	list := make([]*WistiaRespVideo, 0)

	db, err := bolt.Open(this.Conf.FilePath, 0600, nil)
	if err != nil {
		Log.Error("failed to open BoltDB for GetAllVideoInfo", "error", err, "path", this.Conf.FilePath)
		return list, err
	}
	defer db.Close()


	err = db.Update(func(tx *bolt.Tx) error {
		bucket, err := tx.CreateBucketIfNotExists([]byte("media"))
		if err != nil {
			Log.Error("failed to create media bucket for GetAllVideoInfo", "error", err)
			return err
		}

		c := bucket.Cursor()
		for k, v := c.First(); k != nil; k, v = c.Next() {
			var info WistiaRespVideo
			err = json.Unmarshal(v, &info)
			if err != nil {
				Log.Error("failed to unmarshal video info, skipping entry", "error", err, "key", string(k))
				continue
			}
			list = append(list, &info)
		}

		return nil
	})
	if err != nil {
		Log.Error("GetAllVideoInfo transaction failed", "error", err)
		return list, err
	}

	return list, nil
}

func (this *DBHelper) FindVideoInfo(hashId string) (*WistiaRespVideo, error) {
	db, err := bolt.Open(this.Conf.FilePath, 0600, nil)
	if err != nil {
		Log.Error("failed to open BoltDB for FindVideoInfo", "error", err, "path", this.Conf.FilePath, "hash", hashId)
		return nil, err
	}
	defer db.Close()

	var info WistiaRespVideo

	err = db.Update(func(tx *bolt.Tx) error {
		bucket, err := tx.CreateBucketIfNotExists([]byte("media"))
		if err != nil {
			Log.Error("failed to create media bucket for FindVideoInfo", "error", err, "hash", hashId)
			return err
		}

		bin := bucket.Get([]byte(hashId))
		return json.Unmarshal(bin, &info)
	})
	if err != nil {
		Log.Error("FindVideoInfo transaction failed", "error", err, "hash", hashId)
		return nil, err
	}

	return &info, nil
}

func (this *DBHelper) SaveVideoIndex(hashId string, data *DashScopeIndexResult) error {
	db, err := bolt.Open(this.Conf.FilePath, 0600, nil)
	if err != nil {
		Log.Error("failed to open BoltDB for SaveVideoIndex", "error", err, "path", this.Conf.FilePath, "hash", hashId)
		return err
	}
	defer db.Close()

	err = db.Update(func(tx *bolt.Tx) error {
		bucket, err := tx.CreateBucketIfNotExists([]byte("index"))
		if err != nil {
			Log.Error("failed to create index bucket", "error", err, "hash", hashId)
			return err
		}
		bin, err := json.Marshal(data)
		if err != nil {
			Log.Error("failed to marshal index data", "error", err, "hash", hashId)
			return err
		}
		err = bucket.Put([]byte(hashId), bin)
		if err != nil {
			Log.Error("failed to put index into index bucket", "error", err, "hash", hashId)
			return err
		}
		return nil
	})
	if err != nil {
		Log.Error("SaveVideoIndex transaction failed", "error", err, "hash", hashId)
		return err
	}

	return nil
}

func (this *DBHelper) FindVideoIndex(hashId string) (*DashScopeIndexResult, error) {
	db, err := bolt.Open(this.Conf.FilePath, 0600, nil)
	if err != nil {
		Log.Error("failed to open BoltDB for FindVideoIndex", "error", err, "path", this.Conf.FilePath, "hash", hashId)
		return nil, err
	}
	defer db.Close()

	var info DashScopeIndexResult

	err = db.Update(func(tx *bolt.Tx) error {
		bucket, err := tx.CreateBucketIfNotExists([]byte("index"))
		if err != nil {
			Log.Error("failed to create index bucket for FindVideoIndex", "error", err, "hash", hashId)
			return err
		}

		bin := bucket.Get([]byte(hashId))
		if bin == nil {
			return fmt.Errorf("index not found for %s", hashId)
		}
		return json.Unmarshal(bin, &info)
	})
	if err != nil {
		Log.Error("FindVideoIndex transaction failed", "error", err, "hash", hashId)
		return nil, err
	}

	return &info, nil
}