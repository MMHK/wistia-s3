package pkg

import (
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/cloudfront"
	"strconv"
	"time"
)

type CloudFrontHelper struct {
	distID string
	svc    *cloudfront.CloudFront
}

func NewCloudFrontHelper(conf *S3Config) *CloudFrontHelper {
	if conf.CloudFrontDistID == "" {
		return nil
	}
	sess, err := session.NewSession(&aws.Config{
		Region:      aws.String(conf.Region),
		Credentials: credentials.NewStaticCredentials(conf.AccessKey, conf.SecretKey, ""),
	})
	if err != nil {
		Log.Error("failed to create CloudFront session", "dist_id", conf.CloudFrontDistID, "error", err)
		return nil
	}
	return &CloudFrontHelper{
		distID: conf.CloudFrontDistID,
		svc:    cloudfront.New(sess),
	}
}

func (this *CloudFrontHelper) InvalidatePaths(paths []string) error {
	if this == nil {
		return nil
	}
	items := make([]*string, len(paths))
	for i, p := range paths {
		items[i] = aws.String(p)
	}
	input := &cloudfront.CreateInvalidationInput{
		DistributionId: aws.String(this.distID),
		InvalidationBatch: &cloudfront.InvalidationBatch{
			CallerReference: aws.String(strconv.FormatInt(time.Now().UnixNano(), 10)),
			Paths: &cloudfront.Paths{
				Quantity: aws.Int64(int64(len(paths))),
				Items:    items,
			},
		},
	}
	output, err := this.svc.CreateInvalidation(input)
	if err != nil {
		return err
	}
	Log.Info("CloudFront invalidation created", "dist_id", this.distID, "invalidation_id", *output.Invalidation.Id)
	return nil
}
