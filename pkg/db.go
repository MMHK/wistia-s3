package pkg

import (
	"encoding/json"
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
		Log.Error(err)
		return err
	}
	defer db.Close()

	err = db.Update(func(tx *bolt.Tx) error {
		bucket, err := tx.CreateBucketIfNotExists([]byte("media"))
		if err != nil {
			Log.Error(err)
			return err
		}
		bin, err := io.ReadAll(r)
		if err != nil {
			Log.Error(err)
			return err
		}
		err = bucket.Put([]byte(hashId), bin)
		if err != nil {
			Log.Error(err)
			return err
		}
		return nil
	})
	if err != nil {
		Log.Error(err)
		return err
	}

	return nil
}

func (this *DBHelper) GetAllVideoInfo() ([]*WistiaRespVideo, error) {
	list := make([]*WistiaRespVideo, 0)

	db, err := bolt.Open(this.Conf.FilePath, 0600, nil)
	if err != nil {
		Log.Error(err)
		return list, err
	}
	defer db.Close()


	err = db.View(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte("media"))

		c := bucket.Cursor()
		for k, v := c.First(); k != nil; k, v = c.Next() {
			var info WistiaRespVideo
			err = json.Unmarshal(v, &info)
			if err != nil {
				Log.Error(err)
				continue
			}
			list = append(list, &info)
		}

		return nil
	})
	if err != nil {
		Log.Error(err)
		return list, err
	}

	return list, nil
}