import {
	classifySeatAppearanceSchemaVersion,
	seatAppearanceSchemaReadCapability,
} from './seat-appearance-schema'

test.each([
	{ value: undefined, expected: 'v1' },
	{ value: 0, expected: 'v1' },
	{ value: 1, expected: 'v1' },
	{ value: 2, expected: 'v2' },
	{ value: 3, expected: 'unsupported' },
	{ value: '2', expected: 'unsupported' },
])('classifies schema-version $value as $expected', ({ value, expected }) => {
	expect(classifySeatAppearanceSchemaVersion(value)).toBe(expected)
})

test('advertises V1/V2 dual-read capability', () => {
	expect(seatAppearanceSchemaReadCapability).toBe('1,2')
})
