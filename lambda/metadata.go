package lambda

import (
	"context"
	"encoding/json"
	"io/ioutil"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/polly"
	"github.com/aws/aws-sdk-go/service/s3"
	sparta "github.com/mweagle/Sparta"
	"github.com/sirupsen/logrus"
)

// SpartaCastTask is the struct that is passed between
// the PollyTaskCheck, Wait, and Choice states. The
// Choice state will go to the FeedState if the Successful
// property is true, otherwise it'll go back to the WaitState
type SpartaCastTask struct {
	Bucket        string
	Key           string
	SynthesisTask *polly.SynthesisTask
	Item          *Item
	WaitDuration  int64
}

func logInputEvent(ctx context.Context, input interface{}) {
	// Switch on the S3 state change...
	logger, _ := ctx.Value(sparta.ContextKeyLogger).(*logrus.Logger)
	logger.WithFields(logrus.Fields{
		"request": input,
	}).Debug("Input Event")
}

func unmarshalFromS3Object(awsSession *session.Session,
	bucket string,
	key string,
	target interface{},
	logger *logrus.Logger) error {

	// Get the episode, put it back, tell polly to synth it...
	s3Svc := s3.New(awsSession)
	s3GetObjectParams := &s3.GetObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	}
	s3GetObjectResp, s3GetObjectRespErr := s3Svc.GetObject(s3GetObjectParams)
	if s3GetObjectRespErr != nil {
		return s3GetObjectRespErr
	}

	allBytes, allBytesErr := ioutil.ReadAll(s3GetObjectResp.Body)
	if allBytesErr != nil {
		return allBytesErr
	}
	return json.Unmarshal(allBytes, target)
}

func unmarshalSpartaCastConfigFromS3(awsSession *session.Session,
	bucket string,
	key string,
	target interface{},
	logger *logrus.Logger) error {

	// Get the episode, put it back, tell polly to synth it...
	s3Svc := s3.New(awsSession)
	s3GetObjectParams := &s3.GetObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	}
	s3GetObjectResp, s3GetObjectRespErr := s3Svc.GetObject(s3GetObjectParams)
	if s3GetObjectRespErr != nil {
		return s3GetObjectRespErr
	}
	// Parse the object into something useful...
	parseErr := ParseSpartaConfigSpec(s3GetObjectResp.Body,
		&target,
		logger)
	logger.WithFields(logrus.Fields{
		"targetItem": target,
		"bucket":     bucket,
		"key":        key,
		"err":        parseErr,
	}).Debug("Unmarshal from S3 result")
	return parseErr
}
