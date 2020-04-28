package main

import (
	"math/rand"
	"os"
	"time"

	sparta "github.com/mweagle/Sparta"
	spartaCF "github.com/mweagle/Sparta/aws/cloudformation"
	step "github.com/mweagle/Sparta/aws/step"
	infra "github.com/mweagle/SpartaCast/infra"
	"github.com/mweagle/SpartaCast/lambda"
)

func init() {
	rand.Seed(time.Now().Unix())
}

////////////////////////////////////////////////////////////////////////////////
// Main
func main() {
	userStackName := spartaCF.UserScopedStackName("SpartaCast")

	s3BucketResourceName := sparta.CloudFormationResourceName("EventBucket", userStackName)
	stateMachineResourceName := sparta.CloudFormationResourceName("SpartaCastStep", userStackName)
	var lambdaFunctions []*sparta.LambdaAWSInfo

	// Ok, so how to get a dynamic name into the stack?
	providers := lambda.Providers(s3BucketResourceName)
	awsLambdas := make(map[string]*sparta.LambdaAWSInfo)
	for eachKey, eachProvider := range providers {
		lambdaFn, lambdaFnErr := sparta.NewAWSLambdaFromProvider(eachProvider)
		if lambdaFnErr != nil {
			panic("Failed to create lambda func")
		}
		lambdaFunctions = append(lambdaFunctions, lambdaFn)
		awsLambdas[eachKey] = lambdaFn
	}

	// The final state is the generate feed state
	lambdaFeedGenerateState := step.NewLambdaTaskState(lambda.HandleFeedTaskName,
		awsLambdas[lambda.HandleFeedTaskName])
	lambdaFeedGenerateState.Next(step.NewSuccessState("Feed Generated!"))

	// Handle an Episode being uploaded
	lambdaS3StateChangeTask := step.NewLambdaTaskState(lambda.HandleEpisodeS3StateChangeTask,
		awsLambdas[lambda.HandleEpisodeS3StateChangeTask])

	// Start with a choice on whether the input is feed.md or
	// an episode.md entry
	keyNameChoices := []step.ChoiceBranch{
		&step.Not{
			Comparison: &step.StringEquals{
				Variable: "$.detail.requestParameters.key",
				Value:    "feed.md",
			},
			Next: lambdaS3StateChangeTask,
		},
	}
	initialBranchState := step.NewChoiceState("BranchOnUploadType",
		keyNameChoices...).
		WithDefault(lambdaFeedGenerateState)

	// The next state is the check task state...
	lambdaPollyTaskState := step.NewLambdaTaskState(lambda.HandlePollyTaskName,
		awsLambdas[lambda.HandlePollyTaskName])
	lambdaS3StateChangeTask.Next(lambdaPollyTaskState)

	// Create the wait state that waits based on the Selector and then
	// returns to the polly task
	waitState := step.NewDynamicWaitDurationState("waitForPolly", "$.WaitDuration")
	lambdaPollyTaskState.Next(waitState)

	// After the polly task, put in the choice...
	lambdaChoices := []step.ChoiceBranch{
		&step.Not{
			Comparison: &step.StringEquals{
				Variable: "$.SynthesisTask.TaskStatus",
				Value:    "completed",
			},
			Next: lambdaPollyTaskState,
		},
	}
	choiceState := step.NewChoiceState("CheckPollyTaskStatus",
		lambdaChoices...).
		WithDefault(lambdaFeedGenerateState)
	waitState.Next(choiceState)

	// Create the machine...
	startMachine := step.NewStateMachine("SpartaCast", initialBranchState)

	idCloudTrailDecorator, _ := infra.NewCloudTrailDecorator(lambdaFunctions,
		stateMachineResourceName,
		s3BucketResourceName)
	idS3Decorator, _ := infra.NewS3Decorator(lambda.PublicKeyPath, s3BucketResourceName)

	// Setup the hook to annotate
	workflowHooks := &sparta.WorkflowHooks{
		ServiceDecorators: []sparta.ServiceDecoratorHookHandler{
			startMachine.StateMachineNamedDecorator(stateMachineResourceName),
			idS3Decorator,
			idCloudTrailDecorator,
		},
	}

	err := sparta.MainEx(userStackName,
		"Convert markdown to a Polly synthesized podcast",
		lambdaFunctions,
		nil,
		nil,
		workflowHooks,
		false)
	if err != nil {
		os.Exit(1)
	}
}
