import {
	seatAppearanceSchemaReadCapability,
	seatAppearanceV2SchemaVersion,
} from './seat-appearance-schema'

test('contracts the supported SeatAppearance schema to V2', () => {
	expect(seatAppearanceV2SchemaVersion).toBe(2)
	expect(seatAppearanceSchemaReadCapability).toBe('2')
})
