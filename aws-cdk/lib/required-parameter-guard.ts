import * as cdk from 'aws-cdk-lib'

export const requireNonEmptyStringParameters = (
	stack: cdk.Stack,
	parameterIds: readonly string[],
): void => {
	for (const parameterId of parameterIds) {
		const parameter = stack.node.tryFindChild(parameterId)
		if (!(parameter instanceof cdk.CfnParameter)) {
			throw new Error(`CloudFormation parameter not found: ${parameterId}`)
		}

		new cdk.CfnRule(stack, `Require${parameterId}`, {
			assertions: [
				{
					assert: cdk.Fn.conditionNot(
						cdk.Fn.conditionEquals(parameter.valueAsString, ''),
					),
					assertDescription: `${parameterId} must be provided and must not be empty.`,
				},
			],
		})
	}
}
