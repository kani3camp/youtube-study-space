/**
 * ビルド・起動時に NEXT_PUBLIC_DEBUG / NEXT_PUBLIC_ROOM_CONFIG を検証する。
 * 許可値は src/lib/constants.ts の parseDebugFlag / parseRoomConfigName と一致させる
 * （next-public-env.test.ts が一致を検証する）。
 * src 側は読み込み時に評価せず利用時に検証するため、設定誤りはここで早期に失敗させる。
 */
const allowedDebugValues = ['true', 'false']
const allowedRoomConfigNames = ['PROD', 'DEV']

/** @param {Record<string, string | undefined>} env */
function validatePublicEnv(env) {
	if (!allowedDebugValues.includes(env.NEXT_PUBLIC_DEBUG ?? '')) {
		throw new Error(`invalid NEXT_PUBLIC_DEBUG: ${env.NEXT_PUBLIC_DEBUG}`)
	}
	if (!allowedRoomConfigNames.includes(env.NEXT_PUBLIC_ROOM_CONFIG ?? '')) {
		throw new Error(
			`invalid NEXT_PUBLIC_ROOM_CONFIG: ${env.NEXT_PUBLIC_ROOM_CONFIG}`,
		)
	}
}

module.exports = { validatePublicEnv }
