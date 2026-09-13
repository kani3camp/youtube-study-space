export const seatAppearanceV2SchemaVersion = 2
export const seatAppearanceSchemaReadCapability = '1,2'

export type SeatAppearanceSchemaKind = 'v1' | 'v2' | 'unsupported'

export function classifySeatAppearanceSchemaVersion(
	value: unknown,
): SeatAppearanceSchemaKind {
	if (value === seatAppearanceV2SchemaVersion) {
		return 'v2'
	}
	if (value === undefined || value === 0 || value === 1) {
		return 'v1'
	}
	return 'unsupported'
}
