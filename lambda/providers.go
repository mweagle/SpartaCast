package lambda

import (
	sparta "github.com/mweagle/Sparta"
)

const (
	// HandleEpisodeS3StateChangeTask is the name of the handler that responds to PutObject
	// on the episode value.
	HandleEpisodeS3StateChangeTask = "HandleEpisodeS3StateChangeTask"
	// HandlePollyTaskName is the name of the handler that responds to PutObject
	HandlePollyTaskName = "HandlePollyTask"
	// HandleFeedTaskName is the name of the handler that responds to PutObject
	HandleFeedTaskName = "HandleFeedTask"
)

// Providers returns a map of function name to provider
func Providers(s3BucketResourceName string) map[string]sparta.AWSLambdaProvider {

	providerList := []sparta.AWSLambdaProvider{
		newHandleEpisodeS3EventTask(s3BucketResourceName),
		newHandlePollyTask(s3BucketResourceName),
		newHandleFeedTask(s3BucketResourceName),
	}
	providers := make(map[string]sparta.AWSLambdaProvider)
	for _, eachEntry := range providerList {
		providers[eachEntry.Name()] = eachEntry
	}
	return providers
}
