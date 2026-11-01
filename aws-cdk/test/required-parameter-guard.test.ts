import * as cdk from 'aws-cdk-lib'
import { Template } from 'aws-cdk-lib/assertions'

import { requireNonEmptyStringParameters } from '../lib/required-parameter-guard'

describe('requireNonEmptyStringParameters', () => {
	test('removes defaults and rejects empty values', () => {
		const app = new cdk.App()
		const stack = new cdk.Stack(app, 'TestStack')
		new cdk.CfnParameter(stack, 'GoogleCloudProject', {
			type: 'String',
			default: '',
		})

		requireNonEmptyStringParameters(stack, ['GoogleCloudProject'])

		const template = Template.fromStack(stack).toJSON()
		expect(template.Parameters.GoogleCloudProject).toEqual({ Type: 'String' })
		expect(template.Rules.RequireGoogleCloudProject).toEqual({
			Assertions: [
				{
					Assert: {
						'Fn::Not': [
							{
								'Fn::Equals': [
									{ Ref: 'GoogleCloudProject' },
									'',
								],
							},
						],
					},
					AssertDescription:
						'GoogleCloudProject must be provided and must not be empty.',
				},
			],
		})
	})

	test('fails fast when a parameter id does not exist', () => {
		const app = new cdk.App()
		const stack = new cdk.Stack(app, 'TestStack')

		expect(() =>
			requireNonEmptyStringParameters(stack, ['MissingParameter']),
		).toThrow('CloudFormation parameter not found: MissingParameter')
	})
})
