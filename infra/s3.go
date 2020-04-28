package infra

import (
	"strings"

	"github.com/aws/aws-sdk-go/aws/session"
	sparta "github.com/mweagle/Sparta"
	gocf "github.com/mweagle/go-cloudformation"
	"github.com/sirupsen/logrus"
)

// S3BucketDecorator returns the decorator that provisions
// the infrastructure
type S3BucketDecorator struct {
	publicPrefixPath     string
	s3BucketResourceName string
}

// DecorateService satisfies the decorator interface
func (s3bd *S3BucketDecorator) DecorateService(context map[string]interface{},
	serviceName string,
	template *gocf.Template,
	S3Bucket string,
	S3Key string,
	buildID string,
	awsSession *session.Session,
	noop bool,
	logger *logrus.Logger) error {
	cfResource := template.AddResource(s3bd.s3BucketResourceName, &gocf.S3Bucket{})
	cfResource.DeletionPolicy = "Retain"

	// BucketPolicy entry...
	feedBucketPolicy := &gocf.S3BucketPolicy{
		Bucket: gocf.Ref(s3bd.s3BucketResourceName).String(),
		PolicyDocument: sparta.ArbitraryJSONObject{
			"Version": "2012-10-17",
			"Statement": []sparta.ArbitraryJSONObject{
				sparta.ArbitraryJSONObject{
					"Sid":       "EnablePublicAccessToKeyspace",
					"Effect":    "Allow",
					"Principal": "*",
					"Action":    "s3:GetObject",
					"Resource": gocf.Join("",
						gocf.GetAtt(s3bd.s3BucketResourceName, "Arn"),
						gocf.String("/"),
						gocf.String(strings.TrimRight(s3bd.publicPrefixPath, "/")),
						gocf.String("/*")),
				},
			},
		},
	}
	bucketPolicyResourceName := sparta.CloudFormationResourceName(s3bd.s3BucketResourceName, "BucketPolicy")
	cfResource = template.AddResource(bucketPolicyResourceName, feedBucketPolicy)
	cfResource.DependsOn = []string{s3bd.s3BucketResourceName}
	return nil

}

// NewS3Decorator returns an instance of the CloudTrailDecorator
// instance
func NewS3Decorator(publicPrefixPath string,
	s3BucketResourceName string) (*S3BucketDecorator, error) {

	return &S3BucketDecorator{
		publicPrefixPath:     publicPrefixPath,
		s3BucketResourceName: s3BucketResourceName,
	}, nil
}
