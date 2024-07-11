package pkg

import (
	"wistia-s3/tests"
	"fmt"
	"mime"
	"os"
	"path/filepath"
	"testing"
)

func loadS3Config() *S3Config {
	remotePathPrefix := "wistia-backup"
	prefix := os.Getenv("S3_PREFIX")
	if len(prefix) > 0 {
		remotePathPrefix = prefix
	}

	return &S3Config{
		AccessKey: os.Getenv("S3_KEY"),
		SecretKey: os.Getenv("S3_SECRET"),
		Bucket: os.Getenv("S3_BUCKET"),
		Region: os.Getenv("S3_REGION"),
		PrefixPath: remotePathPrefix,
	}
}

func getStorage(t *testing.T) IStorage {
	s3, err := NewS3Storage(loadS3Config())

	if err != nil {
		t.Error(err)
		return nil
	}

	return s3
}

func TestPutStream(t *testing.T) {
	disk := getStorage(t)

	filename := tests.GetLocalPath("../tests/sample.jpeg")

	file, err := os.Open(filename)
	if err != nil {
		t.Error(err)
		t.Fail()
		return
	}
	defer file.Close()

	distPath := fmt.Sprintf("media/%s", filepath.Ext(filename))

	path, url, err := disk.PutStream(file, distPath, &UploadOptions{
		ContentType: mime.TypeByExtension(filename),
	})

	if err != nil {
		t.Error(err)
		return
	}

	t.Log(path)
	t.Log(url)
}
