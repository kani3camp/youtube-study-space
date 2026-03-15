import * as cdk from 'aws-cdk-lib'
import { Match, Template } from 'aws-cdk-lib/assertions'

import { AwsCdkStack } from '../lib/aws-cdk-stack'

const createTemplate = () => {
	const app = new cdk.App()
	const stack = new AwsCdkStack(app, 'TestStack')

	return Template.fromStack(stack)
}

describe('AwsCdkStack', () => {
	const template = createTemplate()

	test('keeps the daily batch schedule invariant', () => {
		template.hasResourceProperties('AWS::Scheduler::Schedule', {
			Name: 'daily-batch-00-00-jst',
			ScheduleExpression: 'cron(0 15 * * ? *)',
			State: 'ENABLED',
			FlexibleTimeWindow: {
				Mode: 'OFF',
			},
			Target: {
				RetryPolicy: {
					MaximumRetryAttempts: 0,
				},
			},
		})
	})

	test('disables retry for the 1 minute organize database target', () => {
		template.hasResourceProperties('AWS::Events::Rule', {
			ScheduleExpression: 'rate(1 minute)',
			Targets: Match.arrayWith([
				Match.objectLike({
					RetryPolicy: {
						MaximumRetryAttempts: 0,
					},
				}),
			]),
		})
	})

	test('keeps API Gateway protected by an API key', () => {
		template.hasResourceProperties('AWS::ApiGateway::Method', {
			HttpMethod: 'POST',
			ApiKeyRequired: true,
			Integration: {
				Type: 'AWS_PROXY',
			},
		})
		template.hasOutput('ApiEndpointUrl', {})
	})

	test('keeps the Fargate task definition runtime invariant', () => {
		template.hasResourceProperties('AWS::ECS::TaskDefinition', {
			Cpu: '256',
			Memory: '512',
			RuntimePlatform: {
				CpuArchitecture: 'ARM64',
				OperatingSystemFamily: 'LINUX',
			},
		})
	})

	test('exposes the required batch outputs', () => {
		template.hasOutput('BatchClusterArn', {})
		template.hasOutput('DailyBatchTaskDefinitionArn', {})
		template.hasOutput('BatchSecurityGroupId', {})
		template.hasOutput('BatchVpcId', {})
		template.hasOutput('DailyBatchStateMachineArn', {})
		template.hasOutput(
			'BatchPublicSubnetIds',
			Match.objectLike({
				Export: {
					Name: 'BatchPublicSubnetIds',
				},
			}),
		)
	})
})
