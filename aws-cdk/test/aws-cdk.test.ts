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

	test('disables retry for all 1 minute rule targets', () => {
		const resources = template.toJSON().Resources as Record<
			string,
			{
				Type: string
				Properties?: {
					ScheduleExpression?: string
					Targets?: Array<{
						RetryPolicy?: {
							MaximumRetryAttempts?: number
						}
					}>
				}
			}
		>
		const [oneMinuteRule] = Object.values(resources).filter(
			(resource) =>
				resource.Type === 'AWS::Events::Rule' &&
				resource.Properties?.ScheduleExpression === 'rate(1 minute)',
		)
		const targets = oneMinuteRule?.Properties?.Targets

		expect(oneMinuteRule).toBeDefined()
		expect(targets).toBeDefined()
		expect(targets).toHaveLength(2)
		for (const target of targets ?? []) {
			expect(target.RetryPolicy).toEqual({
				MaximumRetryAttempts: 0,
			})
		}
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
