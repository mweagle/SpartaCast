package infra

import (
	"fmt"
	"os"

	"github.com/aws/aws-sdk-go/aws/session"
	sparta "github.com/mweagle/Sparta"
	spartaCF "github.com/mweagle/Sparta/aws/cloudformation"
	spartaIAM "github.com/mweagle/Sparta/aws/iam"
	gocf "github.com/mweagle/go-cloudformation"
	"github.com/sirupsen/logrus"
)

// CloudTrailDecorator returns the decorator that provisions
// the infrastructure
type CloudTrailDecorator struct {
	stepFunctionResourceName string
	s3BucketResourceName     string
	lambdaFuncs              []*sparta.LambdaAWSInfo
}

// Description satisfies the Describable interface...How to
// connect this to cloudTrail...then step function?
func (id *CloudTrailDecorator) Description(targetNodeName string) (*sparta.DescriptionInfo, error) {

	descNodes := make([]*sparta.DescriptionTriplet, 0)
	descNodes = append(descNodes,
		&sparta.DescriptionTriplet{
			SourceNodeName: "S3",
			DisplayInfo: &sparta.DescriptionDisplayInfo{
				SourceIcon: &sparta.DescriptionIcon{
					Category: "Storage",
					Name:     "Amazon-Simple-Storage-Service-S3.svg",
				},
			},
			TargetNodeName: "CloudTrail"},
		&sparta.DescriptionTriplet{
			SourceNodeName: "CloudTrail",
			DisplayInfo: &sparta.DescriptionDisplayInfo{
				SourceIcon: &sparta.DescriptionIcon{
					Category: "Management & Governance",
					Name:     "AWS-CloudTrail_light-bg.svg",
				},
			},
			TargetNodeName: "EventBridge"},
		&sparta.DescriptionTriplet{
			SourceNodeName: "EventBridge",
			DisplayInfo: &sparta.DescriptionDisplayInfo{
				SourceIcon: &sparta.DescriptionIcon{
					Category: "Application Integration",
					Name:     "Amazon-EventBridge_light-bg.svg",
				},
			},
			TargetNodeName: "StepFunction"},
		&sparta.DescriptionTriplet{
			SourceNodeName: "StepFunction",
			DisplayInfo: &sparta.DescriptionDisplayInfo{
				SourceIcon: &sparta.DescriptionIcon{
					Category: "_Group Icons",
					Name:     "AWS-Step-Function_light-bg.svg",
				},
			},
			TargetNodeName: targetNodeName})

	for _, eachLambda := range id.lambdaFuncs {
		descNodes = append(descNodes, eachLambda.NewDescriptionTriplet("StepFunction", false))
	}
	return &sparta.DescriptionInfo{
		Nodes: descNodes,
		Name:  "CloudTrailWorkflowHooks",
	}, nil
}

// DecorateService satisfies the decorator interface
func (id *CloudTrailDecorator) DecorateService(context map[string]interface{},
	serviceName string,
	template *gocf.Template,
	S3Bucket string,
	S3Key string,
	buildID string,
	awsSession *session.Session,
	noop bool,
	logger *logrus.Logger) error {

	s3BucketStorageResourceName := sparta.CloudFormationResourceName("CloudTrailStorage",
		id.stepFunctionResourceName,
		id.s3BucketResourceName)
	s3BucketStoragePolicyResourceName := sparta.CloudFormationResourceName("CloudTrailStoragePolicy",
		id.stepFunctionResourceName,
		id.s3BucketResourceName)
	trailResourceName := sparta.CloudFormationResourceName("CloudTrail",
		id.stepFunctionResourceName,
		id.s3BucketResourceName)
	eventBridgeResourceName := sparta.CloudFormationResourceName("EventBridge",
		id.stepFunctionResourceName,
		id.s3BucketResourceName)
	iamRoleResourceName := sparta.CloudFormationResourceName("IAMRole",
		id.stepFunctionResourceName,
		id.s3BucketResourceName)

	// Create the S3 storage bucket with the policy associated with it...
	cfResource := template.AddResource(s3BucketStorageResourceName, &gocf.S3Bucket{})
	cfResource.DeletionPolicy = "Retain"

	// BucketPolicy entry...
	storageBucketPolicy := &gocf.S3BucketPolicy{
		Bucket: gocf.Ref(s3BucketStorageResourceName).String(),
		PolicyDocument: sparta.ArbitraryJSONObject{
			"Version": "2012-10-17",
			"Statement": []sparta.ArbitraryJSONObject{
				sparta.ArbitraryJSONObject{
					"Sid":    "AWSCloudTrailAclCheck20150319",
					"Effect": "Allow",
					"Principal": sparta.ArbitraryJSONObject{
						"Service": "cloudtrail.amazonaws.com",
					},
					"Action":   "s3:GetBucketAcl",
					"Resource": gocf.GetAtt(s3BucketStorageResourceName, "Arn"),
				},
				{
					"Sid":    "AWSCloudTrailWrite20150319",
					"Effect": "Allow",
					"Principal": sparta.ArbitraryJSONObject{
						"Service": "cloudtrail.amazonaws.com",
					},
					"Action": "s3:PutObject",
					"Resource": gocf.Join("",
						gocf.GetAtt(s3BucketStorageResourceName, "Arn"),
						gocf.String("/AWSLogs/"),
						gocf.Ref("AWS::AccountId"),
						gocf.String("/*")),
					"Condition": sparta.ArbitraryJSONObject{
						"StringEquals": sparta.ArbitraryJSONObject{
							"s3:x-amz-acl": "bucket-owner-full-control",
						},
					},
				},
			},
		},
	}
	cfResource = template.AddResource(s3BucketStoragePolicyResourceName, storageBucketPolicy)
	cfResource.DependsOn = []string{s3BucketStorageResourceName}

	// https://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-properties-cloudtrail-trail-dataresource.html
	trailResource := &gocf.CloudTrailTrail{
		S3BucketName:               gocf.Ref(s3BucketStorageResourceName).String(),
		IsLogging:                  gocf.Bool(true),
		IncludeGlobalServiceEvents: gocf.Bool(true),
		IsMultiRegionTrail:         gocf.Bool(false),
		EventSelectors: &gocf.CloudTrailTrailEventSelectorList{
			gocf.CloudTrailTrailEventSelector{
				IncludeManagementEvents: gocf.Bool(true),
				ReadWriteType:           gocf.String("All"),
				DataResources: &gocf.CloudTrailTrailDataResourceList{
					gocf.CloudTrailTrailDataResource{
						Type: gocf.String("AWS::S3::Object"),
						Values: gocf.StringList(
							gocf.Join("",
								gocf.GetAtt(id.s3BucketResourceName, "Arn"),
								gocf.String("/")),
						),
					},
				},
			},
		},
	}
	cfResource = template.AddResource(trailResourceName, trailResource)
	cfResource.DependsOn = []string{s3BucketStorageResourceName,
		s3BucketStoragePolicyResourceName,
		id.s3BucketResourceName}

	iamRoleResource := &gocf.IAMRole{
		AssumeRolePolicyDocument: spartaIAM.AssumeRolePolicyDocumentForServicePrincipal("events.amazonaws.com"),
		Policies: &gocf.IAMRolePolicyList{
			gocf.IAMRolePolicy{
				PolicyName: gocf.String("InvokeStepMachine"),
				PolicyDocument: sparta.ArbitraryJSONObject{
					"Version": "2012-10-17",
					"Statement": []sparta.ArbitraryJSONObject{
						{
							"Sid":      "InvokeStepFunction",
							"Effect":   "Allow",
							"Action":   "states:StartExecution",
							"Resource": gocf.Ref(id.stepFunctionResourceName).String(),
						},
					},
				},
			},
		},
	}
	template.AddResource(iamRoleResourceName, iamRoleResource)

	// Create the EventBridgeRule
	additionalParams := map[string]interface{}{
		"S3BucketResourceName": id.s3BucketResourceName,
	}
	eventData, eventDataErr := os.Open("./infra/eventpattern_put.json")
	if eventDataErr != nil {
		return eventDataErr
	}
	jsonData, jsonDataErr := spartaCF.ConvertToInlineJSONTemplateExpression(eventData, additionalParams)
	if jsonDataErr != nil {
		return jsonDataErr
	}

	// Now create the EventBridge rule that maps events in the target bucket
	// to running the Step function
	eventBridgeResource := &gocf.EventsRule{
		Description: gocf.String(fmt.Sprintf("Rule to trigger %s from an S3 event in %s",
			id.stepFunctionResourceName,
			id.s3BucketResourceName)),
		EventPattern: jsonData,
		Targets: &gocf.EventsRuleTargetList{
			gocf.EventsRuleTarget{
				Arn:     gocf.Ref(id.stepFunctionResourceName).String(),
				ID:      gocf.String("SpartaCast"),
				RoleArn: gocf.GetAtt(iamRoleResourceName, "Arn").String(),
			},
		},
	}
	cfResource = template.AddResource(eventBridgeResourceName,
		eventBridgeResource)
	cfResource.DependsOn = []string{id.stepFunctionResourceName,
		iamRoleResourceName,
		trailResourceName}
	return nil

}

// NewCloudTrailDecorator returns an instance of the CloudTrailDecorator
// instance
func NewCloudTrailDecorator(lambdaFuncs []*sparta.LambdaAWSInfo,
	stepFunctionResourceName string,
	s3BucketResourceName string) (*CloudTrailDecorator, error) {

	return &CloudTrailDecorator{
		lambdaFuncs:              lambdaFuncs,
		stepFunctionResourceName: stepFunctionResourceName,
		s3BucketResourceName:     s3BucketResourceName,
	}, nil
}
