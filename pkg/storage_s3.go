package pkg

import (
	"context"
	"fmt"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/service/s3/s3manager"
	"io"
	"mime"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type S3Storage struct {
	Conf    *S3Config
	session *session.Session
}

func LoadS3ConfigWithEnv() *S3Config {
	remotePathPrefix := "email2db"
	prefix := os.Getenv("S3_PREFIX")
	if len(prefix) > 0 {
		remotePathPrefix = prefix
	}

	return &S3Config{
		AccessKey:        os.Getenv("S3_KEY"),
		SecretKey:        os.Getenv("S3_SECRET"),
		Bucket:           os.Getenv("S3_BUCKET"),
		Region:           os.Getenv("S3_REGION"),
		CloudFrontDomain: os.Getenv("S3_CLOUDFRONT_DOMAIN"),
		PrefixPath:       remotePathPrefix,
	}
}

func NewS3Storage(conf *S3Config) (IStorage, error) {
	sess, err := session.NewSession(&aws.Config{
		Region:      aws.String(conf.Region),
		Credentials: credentials.NewStaticCredentials(conf.AccessKey, conf.SecretKey, ""),
	})
	if err != nil {
		Log.Error(err)
		return nil, err
	}

	return &S3Storage{
		Conf:    conf,
		session: sess,
	}, nil
}

func (this *S3Storage) Upload(localPath string, Key string, opt *UploadOptions) (path string, url string, err error) {
	file, err := os.Open(localPath)
	if err != nil {
		return "", "", err
	}

	defer file.Close()

	uploader := s3manager.NewUploader(this.session)
	path = filepath.ToSlash(filepath.Join(this.Conf.PrefixPath, Key))

	var publicflag *string
	if opt.PublicRead {
		publicflag = aws.String("public-read")
	}

	info, err := uploader.Upload(&s3manager.UploadInput{
		Bucket:      aws.String(this.Conf.Bucket),
		Key:         aws.String(path),
		Body:        file,
		ACL:         publicflag,
		ContentType: aws.String(mime.TypeByExtension(localPath)),
	})

	return path, info.Location, err
}

func (this *S3Storage) PutContent(content string, Key string, opt *UploadOptions) (path string, url string, err error) {
	return this.PutStream(strings.NewReader(content), Key, opt)
}

func (this *S3Storage) PutStream(reader io.Reader, Key string, opt *UploadOptions) (path string, url string, err error) {
	uploader := s3manager.NewUploader(this.session)

	contentType := "application/octet-stream"
	if len(opt.ContentType) > 0 {
		contentType = opt.ContentType
	}

	path = filepath.ToSlash(filepath.Join(this.Conf.PrefixPath, Key))

	var publicflag *string
	if opt.PublicRead {
		publicflag = aws.String("public-read")
	}

	// 创建一个带有超时的上下文
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute * 30)
	defer cancel()

	info, err := uploader.UploadWithContext(ctx, &s3manager.UploadInput{
		Bucket:      aws.String(this.Conf.Bucket),
		Key:         aws.String(path),
		Body:        reader,
		ACL:         publicflag,
		ContentType: aws.String(contentType),
	})

	if err != nil {
		Log.Error(err)
		return path, "", err
	}

	return path, info.Location, err
}

func (this *S3Storage) ListFiles(prefix string) ([]string, error) {
	svc := s3.New(this.session)
	result, err := svc.ListObjectsV2(&s3.ListObjectsV2Input{
		Bucket: aws.String(this.Conf.Bucket),
		Prefix: aws.String(fmt.Sprintf("%s/%s", this.Conf.PrefixPath, prefix)),
	})
	if err != nil {
		Log.Error(err)
		return nil, err
	}

	list := make([]string, 0)
	for _, item := range result.Contents {
		list = append(list, *item.Key)
	}

	return list, nil
}

func (this *S3Storage) GetDownloadLink(Key string) (string, error) {
	svc := s3.New(this.session)

	req, _ := svc.GetObjectRequest(&s3.GetObjectInput{
		Bucket: aws.String(this.Conf.Bucket),
		Key:    aws.String(Key),
	})
	return req.Presign(30 * time.Minute)
}
