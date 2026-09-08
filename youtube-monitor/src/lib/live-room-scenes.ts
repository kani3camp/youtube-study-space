export const LIVE_ROOM_SCENES_ENABLED_BY_DEFAULT =
	process.env.NEXT_PUBLIC_LIVE_ROOM_SCENES_ENABLED === 'true'

const OFF_VALUES = new Set(['off', 'false', '0'])
const ON_VALUES = new Set(['on', 'true', '1'])

export function resolveLiveRoomScenesEnabled({
	defaultEnabled,
	queryValue,
	debugEnabled,
	routerReady,
}: {
	defaultEnabled: boolean
	queryValue: string | string[] | undefined
	debugEnabled: boolean
	routerReady: boolean
}): boolean {
	if (!routerReady) {
		return false
	}
	if (typeof queryValue !== 'string') {
		return defaultEnabled
	}

	const normalized = queryValue.trim().toLowerCase()
	if (OFF_VALUES.has(normalized)) {
		return false
	}
	if (debugEnabled && ON_VALUES.has(normalized)) {
		return true
	}
	return defaultEnabled
}
