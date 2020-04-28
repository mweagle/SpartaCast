package lambda

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/eduncan911/podcast"
	"github.com/jmespath/go-jmespath"
	sparta "github.com/mweagle/Sparta"
	spartaCF "github.com/mweagle/Sparta/aws/cloudformation"
	gocf "github.com/mweagle/go-cloudformation"
	"github.com/sirupsen/logrus"
)

type feedMetadata struct {
	feed    *Feed
	entries []*Item
}

func readFeedMetadata(awsSession *session.Session,
	bucket string,
	logger *logrus.Logger) (*feedMetadata, error) {
	var entryMap sync.Map
	var entryErrorMap sync.Map

	selfURL := func(keyname string) string {
		return fmt.Sprintf("https://%s.s3.%s.amazonaws.com/%s/feed/%s",
			bucket,
			*awsSession.Config.Region,
			PublicKeyPath,
			keyname)
	}

	feedMetadata := &feedMetadata{
		entries: []*Item{},
	}

	var wg sync.WaitGroup
	// Unmarshal the feed...
	wg.Add(1)
	go func(awsSession *session.Session) {
		feedItem := Feed{}
		unmarshalErr := unmarshalSpartaCastConfigFromS3(awsSession,
			bucket,
			FeedConfigName,
			&feedItem,
			logger)
		if unmarshalErr != nil {
			entryErrorMap.LoadOrStore(FeedConfigName, unmarshalErr)
		} else {
			// Create the URL to the item, but for that we need to know
			// the full bucket URI...
			// FFS
			feedItem.Link = selfURL(FeedConfigName)
			feedMetadata.feed = &feedItem
		}
		logger.WithFields(logrus.Fields{
			"feedKey":  FeedConfigName,
			"feedItem": feedMetadata.feed,
		}).Debug("Unmarshalled feed")
		wg.Done()
	}(awsSession)

	// Read all the entries in the metadata key prefix and add them

	s3Svc := s3.New(awsSession)
	listObjectsInput := &s3.ListObjectsInput{
		Bucket: aws.String(bucket),
		Prefix: aws.String(fmt.Sprintf("%s/%s/",
			PublicKeyPath,
			KeyComponentMetadata)),
	}
	for {
		listObjectsOutputResp, listObjectsOutputRespErr := s3Svc.ListObjects(listObjectsInput)
		if listObjectsOutputRespErr != nil {
			return nil, listObjectsOutputRespErr
		}
		if listObjectsOutputResp.Contents != nil {
			for _, eachObject := range listObjectsOutputResp.Contents {
				if *eachObject.Key != FeedConfigName &&
					strings.HasSuffix(*eachObject.Key, ".json") {
					wg.Add(1)
					go func(awsSession *session.Session, obj *s3.Object) {
						taskItem := SpartaCastTask{}
						unmarshalErr := unmarshalFromS3Object(awsSession,
							bucket,
							*obj.Key,
							&taskItem,
							logger)
						if unmarshalErr != nil {
							entryErrorMap.LoadOrStore(*obj.Key, unmarshalErr)
						} else {
							logger.WithFields(logrus.Fields{
								"keyName": *obj.Key,
								"entry":   taskItem,
							}).Error("Failed to add entry")
							taskItem.Item.SelfLink = selfURL(*obj.Key)
							entryMap.LoadOrStore(*obj.Key, &taskItem)
						}
						wg.Done()
					}(awsSession, eachObject)
				}
			}
		}

		if *listObjectsOutputResp.IsTruncated == false {
			break
		}
		listObjectsInput.Marker = listObjectsOutputResp.Marker
	}
	wg.Wait()

	// Unmarshal everything...
	errors := []string{}
	entryErrorMap.Range(func(keyName interface{}, err interface{}) bool {
		errors = append(errors,
			fmt.Sprintf("Keypath <%s> failed with error %v",
				keyName,
				err))
		return true
	})
	if len(errors) != 0 {
		return nil, fmt.Errorf("Failed to unmarshal: %#v", errors)
	}
	//
	entryMap.Range(func(keyName interface{}, task interface{}) bool {
		taskEntry, _ := task.(*SpartaCastTask)
		feedMetadata.entries = append(feedMetadata.entries, taskEntry.Item)
		return true
	})
	// Sort everything...
	// Great, sort the feed by timestamp...how to get that?
	sort.Slice(feedMetadata.entries, func(lhs int, rhs int) bool {
		leftEntry := feedMetadata.entries[lhs]
		rightEntry := feedMetadata.entries[rhs]

		parsedLeftDate, _ := time.Parse(time.RFC3339, leftEntry.PubDate)
		parsedRightDate, _ := time.Parse(time.RFC3339, rightEntry.PubDate)

		return parsedLeftDate.Unix() < parsedRightDate.Unix()
	})

	return feedMetadata, nil
}

func createFeed(awsSession *session.Session,
	metadata *feedMetadata,
	bucketName string,
	logger *logrus.Logger) error {
	pubDate, _ := time.Parse(time.RFC3339, metadata.feed.PubDate)

	pc := podcast.New(
		metadata.feed.Title,
		metadata.feed.Link,
		metadata.feed.Description,
		&pubDate,
		nil,
	)
	pc.AddAuthor(metadata.feed.AuthorName, metadata.feed.AuthorEmail)
	pc.IAuthor = metadata.feed.AuthorName
	pc.IOwner = &podcast.Author{
		Name:  metadata.feed.AuthorName,
		Email: metadata.feed.AuthorEmail,
	}
	pc.AddImage(metadata.feed.Image)
	pc.IImage = &podcast.IImage{
		HREF: metadata.feed.Image,
	}
	pc.WebMaster = metadata.feed.WebMaster
	pc.AddCategory(metadata.feed.Category, []string{metadata.feed.Subcategory})
	pc.IExplicit = metadata.feed.IExplicit
	pc.IAuthor = metadata.feed.IAuthor

	// Write it to a buffer...
	for _, eachEntry := range metadata.entries {
		// create an Item
		entryPubDate, _ := time.Parse(time.RFC3339, eachEntry.PubDate)
		item := podcast.Item{
			Title:       eachEntry.Title,
			Link:        eachEntry.SelfLink,
			Description: eachEntry.Description,
			PubDate:     &entryPubDate,
		}
		item.AddEnclosure(eachEntry.EnclosureLink,
			podcast.MP3,
			eachEntry.EnclosureByteLength)
		item.AddImage(eachEntry.Image)
		item.IImage = &podcast.IImage{
			HREF: eachEntry.Image,
		}
		item.ISummary = &podcast.ISummary{
			Text: eachEntry.Description,
		}
		item.Link = "https://gosparta.io"

		// add the Item and check for validation errors
		_, addItemErr := pc.AddItem(item)
		if addItemErr != nil {
			logger.WithFields(logrus.Fields{
				"error": addItemErr,
				"entry": eachEntry,
			}).Warn("Failed to add entry")
		}
	}

	// Write that to S3...
	byteSink := new(bytes.Buffer)
	encodeErr := pc.Encode(byteSink)
	if encodeErr != nil {
		return encodeErr
	}

	// Ship it...
	s3PutObjectInput := &s3.PutObjectInput{
		Bucket:      aws.String(bucketName),
		Key:         aws.String(fmt.Sprintf("%s/%s/feed.xml", PublicKeyPath, KeyComponentFeed)),
		Body:        bytes.NewReader(byteSink.Bytes()),
		ContentType: aws.String("application/rss+xml"),
	}
	s3Svc := s3.New(awsSession)
	s3PutObjectResp, s3PutObjectRespErr := s3Svc.PutObject(s3PutObjectInput)
	if s3PutObjectRespErr != nil {
		return s3PutObjectRespErr
	}
	logger.WithFields(logrus.Fields{
		"feed": *s3PutObjectResp,
	}).Info("Feed created")
	return nil
}

////////////////////////////////////////////////////////////////////////////////
/*
  ___           _
 | __|__ ___ __| |
 | _/ -_) -_) _` |
 |_|\___\___\__,_|
*/
////////////////////////////////////////////////////////////////////////////////
// Handle the S3 state change function
type handleFeedTask struct {
	s3BucketResourceName string
}

func (lambda *handleFeedTask) Name() string {
	return HandleFeedTaskName
}

func (lambda *handleFeedTask) Handler() interface{} {
	// How to get the bucket unless it's in the message?
	// Thinking...
	handler := func(ctx context.Context, input json.RawMessage) error {
		////////////////////////////////////////////////////////////////////////
		// Setup some props
		logger, _ := ctx.Value(sparta.ContextKeyLogger).(*logrus.Logger)
		if logger == nil {
			return fmt.Errorf("Failed to extract Logger instance")
		}
		awsSession, _ := ctx.Value(sparta.ContextKeyAWSSession).(*session.Session)
		if awsSession == nil {
			return fmt.Errorf("Failed to extract AWS Session")
		}
		////////////////////////////////////////////////////////////////////////

		// What's the input?
		ctEvent := CloudTrail{}
		task := SpartaCastTask{}
		jsonErr := json.Unmarshal(input, &ctEvent)
		if jsonErr != nil || ctEvent.Detail.RequestParameters.Key == "" {
			json.Unmarshal(input, &task)
		} else {
			task.Bucket = ctEvent.Detail.RequestParameters.BucketName
			task.Key = ctEvent.Detail.RequestParameters.Key
		}

		var data interface{}
		unmarshalErr := json.Unmarshal(input, &data)
		if unmarshalErr != nil {
			return unmarshalErr
		}
		// Ok, so this is either a SpartaTask or a CloudWatchEvent...
		// We'll use JMESPath to get the bucket s.t. we can do the
		// rest of the work...
		// (1) Try a SpartaTask
		bucketName := ""
		taskResult, taskResultErr := jmespath.Search("Bucket", data)
		if taskResult == nil || taskResultErr != nil {
			ctEventResult, _ := jmespath.Search("detail.requestParameters.bucketName", data)
			bucketName = fmt.Sprintf("%v", ctEventResult)
		} else {
			bucketName = fmt.Sprintf("%v", taskResult)
		}
		logger.WithFields(logrus.Fields{
			"bucketName": bucketName,
			"task":       task,
		}).Info("Feed task activated with bucket")

		// Great, so we have the bucket and we just need to create the feed. So let's go ahead
		// and make that...
		feedMetadata, feedMetadataErr := readFeedMetadata(awsSession, bucketName, logger)
		if feedMetadataErr != nil {
			return feedMetadataErr
		}
		return createFeed(awsSession, feedMetadata, bucketName, logger)
	}
	return handler
}

func (lambda *handleFeedTask) Role() interface{} {
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
			Actions:  []string{"s3:ListBucket"},
			Resource: gocf.GetAtt(lambda.s3BucketResourceName, "Arn"),
		},
	)

	return role
}

// newHandleFeedTask returns an s3 event handler
func newHandleFeedTask(s3BucketResourceName string) sparta.AWSLambdaProvider {
	return &handleFeedTask{
		s3BucketResourceName: s3BucketResourceName,
	}
}
