package main

import (
	"github.com/pulumi/pulumi-aws/sdk/v4/go/aws/apigateway"
	"github.com/pulumi/pulumi-aws/sdk/v4/go/aws/iam"
	"github.com/pulumi/pulumi-aws/sdk/v4/go/aws/lambda"
	"github.com/pulumi/pulumi-gcp/sdk/v6/go/gcp/firestore"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

func main() {
	deployAWS()
	deployGCP()
}

func deployAWS() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		
		lambdaRole, err := iam.NewRole(ctx, "my-first-golang-lambda-function-role-cb8uw4th", &iam.RoleArgs{
			AssumeRolePolicy: pulumi.String(`{
				"Version": "2012-10-17",
				"Statement": [
					{
						"Effect": "Allow",
						"Action": "logs:CreateLogGroup",
						"Resource": "arn:aws:logs:ap-northeast-1:652333062396:*"
					},
					{
						"Effect": "Allow",
						"Action": [
							"logs:CreateLogStream",
							"logs:PutLogEvents"
						],
						"Resource": [
							"arn:aws:logs:ap-northeast-1:652333062396:log-group:/aws/lambda/my-first-golang-lambda-function:*"
						]
					},
					{
						"Effect": "Allow",
						"Action": [
							"iam:PassRole"
						],
						"Resource": [
							"arn:aws:iam::652333062396:role/service-role/my-first-golang-lambda-function-role-cb8uw4th"
						]
					}
				]
			}`),
		})
		if err != nil {
			return err
		}
		
		// Attach the IAM policy to the Lambda role
		_, err = iam.NewRolePolicyAttachment(ctx, "lambdaPolicyAttachment", &iam.RolePolicyAttachmentArgs{
			PolicyArn: pulumi.String("arn:aws:iam::aws:policy/PowerUserAccess"),
			Role:      lambdaRole.Name,
		})
		if err != nil {
			return err
		}
		
		// Create the Lambda function
		lambdaFunction, err := lambda.NewFunction(ctx, "exampleLambda", &lambda.FunctionArgs{
			Runtime:    pulumi.String("nodejs14.x"),
			Code:       pulumi.NewFileArchive("lambda/index.js"),
			Timeout:    pulumi.Int(10),
			MemorySize: pulumi.Int(128),
			Handler:    pulumi.String("index.handler"),
			Role:       lambdaRole.Arn,
		})
		if err != nil {
			return err
		}
		
		// Create the API Gateway REST API
		apiGateway, err := apigateway.NewRestApi(ctx, "exampleApiGateway", nil)
		if err != nil {
			return err
		}
		
		// Create a resource for the Lambda function
		apiGatewayResource, err := apigateway.NewResource(ctx, "exampleResource", &apigateway.ResourceArgs{
			ParentId: apiGateway.RootResourceId,
			PathPart: pulumi.String("{proxy+}"),
			RestApi:  apiGateway.ID(),
		})
		if err != nil {
			return err
		}
		
		// Create an API Gateway integration with the Lambda function
		_, err = apigateway.NewIntegration(ctx, "exampleIntegration", &apigateway.IntegrationArgs{
			IntegrationHttpMethod: pulumi.String("ANY"),
			HttpMethod:            pulumi.String("ANY"),
			Type:                  pulumi.String("AWS_PROXY"),
			Uri:                   lambdaFunction.InvokeArn,
			ResourceId:            apiGatewayResource.ID(),
			RestApi:               apiGateway.ID(),
		})
		if err != nil {
			return err
		}
		
		// Create a deployment of the API Gateway
		apiGatewayDeployment, err := apigateway.NewDeployment(ctx, "exampleDeployment", &apigateway.DeploymentArgs{
			RestApi: apiGateway.ID(),
		}, pulumi.DependsOn([]pulumi.Resource{apiGatewayResource}))
		if err != nil {
			return err
		}
		
		return nil
	})
}

func deployGCP() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		db, err := firestore.NewDatabase(ctx, "db", &firestore.DatabaseArgs{
			LocationId: pulumi.String("asia-southeast2"),
			Type:       pulumi.String("FIRESTORE_NATIVE"),
			Project:    pulumi.String("test-youtube-study-space"),
		})
		if err != nil {
			return err
		}
		ctx.Export("dbName", db.Name)
		
		return nil
	})
}
