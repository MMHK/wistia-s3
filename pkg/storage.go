package pkg

import (
	"errors"
	"io"
)

type S3Config struct {
	AccessKey  string `json:"access_key"`
	SecretKey  string `json:"secret_key"`
	Bucket     string `json:"bucket"`
	Region     string `json:"region"`
	PrefixPath string `json:"prefix"`
}

type StorageConfig struct {
	S3  *S3Config `json:"s3"`
}

type UploadOptions struct {
	ContentType string
	PublicRead bool
}

type IStorage interface {
	Upload(localPath string, Key string, opt *UploadOptions) (string, string, error)
	PutContent(content string, Key string, opt *UploadOptions) (string, string, error)
	PutStream(reader io.Reader, Key string, opt *UploadOptions) (string, string, error)
	GetDownloadLink(Key string) (string, error)
}

func GetStorage(conf *StorageConfig) (IStorage, error) {
	if len(conf.S3.AccessKey) > 0 {
		disk, err := NewS3Storage(conf.S3)
		if err != nil {
			Log.Error(err)
			return nil, err
		}
		return disk, nil
	}
	return nil, errors.New("storages configuration not found")
}