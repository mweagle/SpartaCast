package lambda

import (
	"context"
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/polly"
	sparta "github.com/mweagle/Sparta"
	spartaCF "github.com/mweagle/Sparta/aws/cloudformation"
	gocf "github.com/mweagle/go-cloudformation"
	"github.com/sirupsen/logrus"
)

func valOrDefault(value string, defVal string) *string {
	if value == "" {
		value = defVal
	}
	if value == "" {
		return nil
	}
	return aws.String(value)
}

////////////////////////////////////////////////////////////////////////////////
/*
  ___      _             _
 | __|_ __(_)___ ___  __| |___
 | _|| '_ \ (_-</ _ \/ _` / -_)
 |___| .__/_/__/\___/\__,_\___|
     |_|
*/
////////////////////////////////////////////////////////////////////////////////
// Handle the S3 state change function
type handleEpisodeS3StateChangeTask struct {
	s3BucketResourceName string
}

func (lambda *handleEpisodeS3StateChangeTask) Name() string {
	return HandleEpisodeS3StateChangeTask
}

func (lambda *handleEpisodeS3StateChangeTask) Handler() interface{} {
	handler := func(ctx context.Context, ctEvent CloudTrail) (*SpartaCastTask, error) {
		logInputEvent(ctx, ctEvent)

		// Super, write a "got it" receipt back, but to do that we need a struct for the message
		logger, _ := ctx.Value(sparta.ContextKeyLogger).(*logrus.Logger)

		// Get the content
		awsSession, _ := ctx.Value(sparta.ContextKeyAWSSession).(*session.Session)
		if awsSession == nil {
			return nil, fmt.Errorf("Failed to extract AWS Session")
		}

		// Parse the input. If it's a feed.md, then it's a feed,
		// otherwise it's an episode...
		configEntry := Item{}
		configEntryErr := unmarshalSpartaCastConfigFromS3(awsSession,
			ctEvent.Detail.RequestParameters.BucketName,
			ctEvent.Detail.RequestParameters.Key,
			&configEntry,
			logger,
		)
		if configEntryErr != nil {
			return nil, configEntryErr
		}

		// Tell Polly to create it and dump it in the /public folder
		// It's SSML iff it starts with a <speak> tag....
		isSSML := strings.HasPrefix("<speak>", configEntry.Episode)
		logger.WithFields(logrus.Fields{
			"value":  configEntry.Episode,
			"isSSML": isSSML,
		}).Info("User Text")

		outputKeyPrefix := fmt.Sprintf("%s/%s/%s",
			PublicKeyPath,
			KeyComponentFeed,
			ctEvent.Detail.RequestParameters.Key)

		pollyService := polly.New(awsSession)
		textType := polly.TextTypeText
		if isSSML {
			textType = polly.TextTypeSsml
		}
		pollyInput := &polly.StartSpeechSynthesisTaskInput{
			OutputFormat:       aws.String(polly.OutputFormatMp3),
			OutputS3BucketName: aws.String(ctEvent.Detail.RequestParameters.BucketName),
			OutputS3KeyPrefix:  aws.String(outputKeyPrefix),
			VoiceId:            valOrDefault(configEntry.PollyVoiceID, polly.VoiceIdJoanna),
			Engine:             valOrDefault(configEntry.PollyEngineType, polly.EngineNeural),
			LanguageCode:       valOrDefault(configEntry.PollyLanguageCode, polly.LanguageCodeEnUs),
			Text:               aws.String(configEntry.Episode),
			TextType:           aws.String(textType),
		}

		pollyResp, pollyRespErr := pollyService.StartSpeechSynthesisTask(pollyInput)
		if pollyRespErr != nil {
			return nil, pollyRespErr
		}

		// Pass the info along, but ignore the user content
		configEntry.Episode = ""
		taskStatus := &SpartaCastTask{
			SynthesisTask: pollyResp.SynthesisTask,
			Bucket:        ctEvent.Detail.RequestParameters.BucketName,
			Item:          &configEntry,
			Key:           ctEvent.Detail.RequestParameters.Key,
		}
		// Return the SpartaCastTask item along the State machine
		return taskStatus, nil
	}
	return handler
}

func (lambda *handleEpisodeS3StateChangeTask) Role() interface{} {
	role := sparta.IAMRoleDefinition{}

	role.Privileges = append(role.Privileges,
		sparta.IAMRolePrivilege{
			Actions: []string{"s3:Get*",
				"s3:Put*",
				"s3:Head*",
			},
			Resource: spartaCF.S3AllKeysArnForBucket(gocf.Ref(lambda.s3BucketResourceName)),
		},
		sparta.IAMRolePrivilege{
			Actions:  []string{"polly:*"},
			Resource: "*",
		})

	return role
}

// newHandleEpisodeS3EventTask returns an s3 event handler
func newHandleEpisodeS3EventTask(s3BucketResourceName string) sparta.AWSLambdaProvider {
	return &handleEpisodeS3StateChangeTask{
		s3BucketResourceName: s3BucketResourceName,
	}
}
