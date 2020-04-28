package lambda

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/polly"
	"github.com/aws/aws-sdk-go/service/s3"
	sparta "github.com/mweagle/Sparta"
	spartaCF "github.com/mweagle/Sparta/aws/cloudformation"
	gocf "github.com/mweagle/go-cloudformation"
	"github.com/sirupsen/logrus"
	"golang.org/x/sync/errgroup"
)

type pollyParallelTask func() error
type pollyParallelTaskConstructor func(input *SpartaCastTask,
	awsSession *session.Session,
	logger *logrus.Logger) pollyParallelTask

func newDeleteObsoleteOutputTask(input *SpartaCastTask,
	awsSession *session.Session,
	logger *logrus.Logger) pollyParallelTask {

	return func() error {
		s3Svc := s3.New(awsSession)

		// It's done...get the old item if it exists and delete it...
		manifestKey := fmt.Sprintf("%s/%s/%s.json",
			PublicKeyPath,
			KeyComponentMetadata,
			input.Key)
		existingSpartaCastTask := SpartaCastTask{}
		unmarshalErr := unmarshalFromS3Object(awsSession,
			input.Bucket,
			manifestKey,
			&existingSpartaCastTask,
			logger)
		logger.WithFields(logrus.Fields{
			"inputKey":   input.Key,
			"error":      unmarshalErr,
			"legacyItem": existingSpartaCastTask,
		}).Info("Purging oobsoleted entry")

		if unmarshalErr == nil &&
			existingSpartaCastTask.SynthesisTask.OutputUri != nil &&
			*existingSpartaCastTask.SynthesisTask.OutputUri != "" {
			keyPath, keyPathErr := keyPathFromS3URI(*existingSpartaCastTask.SynthesisTask.OutputUri,
				input.Bucket)
			if keyPathErr == nil {
				deleteInputParams := &s3.DeleteObjectInput{
					Bucket: aws.String(input.Bucket),
					Key:    aws.String(keyPath),
				}
				s3DeleteObjResp, s3DeleteObjRespErr := s3Svc.DeleteObject(deleteInputParams)
				logger.WithFields(logrus.Fields{
					"request":   deleteInputParams,
					"s3Resp":    s3DeleteObjResp,
					"s3RespErr": s3DeleteObjRespErr,
				}).Debug("Results of newDeleteObsoleteOutputTask")
			}
		}
		return nil
	}
}

func newSetOutputMediaTypeTask(input *SpartaCastTask,
	awsSession *session.Session,
	logger *logrus.Logger) pollyParallelTask {
	return func() error {

		s3Svc := s3.New(awsSession)
		keyPath, keyPathErr := keyPathFromS3URI(*input.SynthesisTask.OutputUri,
			input.Bucket)
		if keyPathErr != nil {
			return keyPathErr
		}
		s3HeadObjectParams := &s3.HeadObjectInput{
			Bucket: aws.String(input.Bucket),
			Key:    aws.String(keyPath),
		}
		s3HeadResp, s3HeadRespErr := s3Svc.HeadObject(s3HeadObjectParams)
		logger.WithFields(logrus.Fields{
			"s3HeadResp":         s3HeadResp.Metadata,
			"s3HeadRespErr":      s3HeadRespErr,
			"s3HeadObjectParams": s3HeadObjectParams,
		}).Debug("S3 HeadObject")

		if s3HeadRespErr != nil {
			return s3HeadRespErr
		}
		if s3HeadResp.Metadata == nil {
			s3HeadResp.Metadata = make(map[string]*string)
		}

		s3HeadResp.Metadata["Content-Type"] = aws.String(ContentTypeMP3)
		copySource := fmt.Sprintf("%s/%s", input.Bucket, keyPath)
		s3CopyObjectInput := &s3.CopyObjectInput{
			Bucket:            aws.String(input.Bucket),
			CopySource:        aws.String(copySource),
			Key:               aws.String(keyPath),
			ContentType:       aws.String(ContentTypeMP3),
			Metadata:          s3HeadResp.Metadata,
			MetadataDirective: aws.String(s3.MetadataDirectiveReplace),
		}
		s3CopyObjectResp, s3CopyObjectRespErr := s3Svc.CopyObject(s3CopyObjectInput)
		logger.WithFields(logrus.Fields{
			"metadata":            s3HeadResp.Metadata,
			"s3CopyObjectResp":    s3CopyObjectResp,
			"s3CopyObjectRespErr": s3CopyObjectRespErr,
		}).Debug("Results of newSetOutputMediaTypeTask")
		return s3CopyObjectRespErr
	}
}

func newCreateMetadataTask(input *SpartaCastTask,
	awsSession *session.Session,
	logger *logrus.Logger) pollyParallelTask {

	return func() error {
		// Get the metadata for the item s.t. we can include the enclosure length
		keyPath, keyPathErr := keyPathFromS3URI(*input.SynthesisTask.OutputUri,
			input.Bucket)
		if keyPathErr != nil {
			return keyPathErr
		}
		s3Svc := s3.New(awsSession)
		s3HeadObjectInput := &s3.HeadObjectInput{
			Bucket: aws.String(input.Bucket),
			Key:    aws.String(keyPath),
		}
		s3HeadObjectResp, s3HeadObjectRespErr := s3Svc.HeadObject(s3HeadObjectInput)
		if s3HeadObjectRespErr != nil {
			return s3HeadObjectRespErr
		}
		manifestKey := fmt.Sprintf("%s/%s/%s.json",
			PublicKeyPath,
			KeyComponentMetadata,
			input.Key)

		// Super...write this summary back to the root of the bucket s.t.
		// we can use an Athena query to fetch everything...
		// TODO -  Update all the items info...
		input.Item.PubDate = time.Now().Format(time.RFC3339)
		input.Item.EnclosureLink = *input.SynthesisTask.OutputUri
		input.Item.EnclosureByteLength = *s3HeadObjectResp.ContentLength
		jsonBytes, _ := json.Marshal(&input)

		putObjectInput := &s3.PutObjectInput{
			Bucket: aws.String(input.Bucket),
			Key:    aws.String(manifestKey),
			Body:   bytes.NewReader(jsonBytes),
		}
		putObjectResp, putObjectRespErr := s3Svc.PutObject(putObjectInput)
		logger.WithFields(logrus.Fields{
			"putObjectResp":    putObjectResp,
			"putObjectRespErr": putObjectRespErr,
		}).Info("Results of newCreateMetadataTask")

		return putObjectRespErr
	}
}

////////////////////////////////////////////////////////////////////////////////
/*
  ___     _ _
 | _ \___| | |_  _
 |  _/ _ \ | | || |
 |_| \___/_|_|\_, |
              |__/
*/
////////////////////////////////////////////////////////////////////////////////
// Handle the S3 state change function
type handlePollyTask struct {
	s3BucketResourceName string
}

func (lambda *handlePollyTask) Name() string {
	return HandlePollyTaskName
}

func (lambda *handlePollyTask) Handler() interface{} {
	handler := func(ctx context.Context, input SpartaCastTask) (*SpartaCastTask, error) {
		////////////////////////////////////////////////////////////////////////
		// Preconditions
		////////////////////////////////////////////////////////////////////////
		logger, _ := ctx.Value(sparta.ContextKeyLogger).(*logrus.Logger)
		logger.WithFields(logrus.Fields{
			"input": input,
		}).Debug("SpartaCastTask")
		awsSession, _ := ctx.Value(sparta.ContextKeyAWSSession).(*session.Session)
		if awsSession == nil {
			return nil, fmt.Errorf("Failed to extract AWS Session")
		}
		////////////////////////////////////////////////////////////////////////
		// Preconditions
		////////////////////////////////////////////////////////////////////////

		getSpeechSynthesisTaskInput := &polly.GetSpeechSynthesisTaskInput{
			TaskId: input.SynthesisTask.TaskId,
		}
		pollySvc := polly.New(awsSession)
		getTaskResp, getTaskRespErr := pollySvc.GetSpeechSynthesisTask(getSpeechSynthesisTaskInput)
		if getTaskRespErr != nil {
			return nil, getTaskRespErr
		}
		// Update it...
		input.SynthesisTask = getTaskResp.SynthesisTask
		logger.WithFields(logrus.Fields{
			"input": input,
		}).Info("Updated Task Status")

		switch *input.SynthesisTask.TaskStatus {
		case polly.TaskStatusFailed:
			return nil, fmt.Errorf("Failed to synthesize speech (TaskID: %s)", *input.SynthesisTask.TaskId)
		case polly.TaskStatusInProgress,
			polly.TaskStatusScheduled:
			input.WaitDuration = input.WaitDuration * 2
			if input.WaitDuration <= 0 {
				input.WaitDuration = 10
			}
			if input.WaitDuration > 60 {
				input.WaitDuration = 60
			}
			return &input, nil
		}
		// Run the tasks...
		parallelTasks := []pollyParallelTaskConstructor{
			newDeleteObsoleteOutputTask,
			newSetOutputMediaTypeTask,
			newCreateMetadataTask,
		}
		var taskGroup errgroup.Group
		for _, eachTask := range parallelTasks {
			goFunc := eachTask(&input, awsSession, logger)
			taskGroup.Go(goFunc)
		}
		taskErr := taskGroup.Wait()
		if taskErr != nil {
			return nil, taskErr
		}
		return &input, nil
	}
	return handler
}

func (lambda *handlePollyTask) Role() interface{} {
	role := sparta.IAMRoleDefinition{}

	role.Privileges = append(role.Privileges,
		sparta.IAMRolePrivilege{
			Actions: []string{"s3:Put*",
				"s3:Head*",
				"s3:DeleteObject",
				"s3:Copy*"},
			Resource: spartaCF.S3AllKeysArnForBucket(gocf.Ref(lambda.s3BucketResourceName)),
		},
		sparta.IAMRolePrivilege{
			Actions:  []string{"polly:*"},
			Resource: "*",
		})

	return role
}

// newHandlePollyTask returns an s3 event handler
func newHandlePollyTask(s3BucketResourceName string) sparta.AWSLambdaProvider {
	return &handlePollyTask{
		s3BucketResourceName: s3BucketResourceName,
	}
}
