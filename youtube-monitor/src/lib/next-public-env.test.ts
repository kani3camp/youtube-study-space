import { validatePublicEnv } from '../../next-public-env'
import { parseDebugFlag, parseRoomConfigName } from './constants'

describe('validatePublicEnv', () => {
	test('accepts the values that the runtime parsers accept', () => {
		for (const debug of ['true', 'false']) {
			for (const roomConfig of ['PROD', 'DEV']) {
				expect(() =>
					validatePublicEnv({
						NEXT_PUBLIC_DEBUG: debug,
						NEXT_PUBLIC_ROOM_CONFIG: roomConfig,
					}),
				).not.toThrow()
				expect(() => parseDebugFlag(debug)).not.toThrow()
				expect(() => parseRoomConfigName(roomConfig)).not.toThrow()
			}
		}
	})

	test.each([undefined, '', 'TRUE', 'yes'])(
		'rejects invalid NEXT_PUBLIC_DEBUG %s like parseDebugFlag',
		(debug) => {
			expect(() =>
				validatePublicEnv({
					NEXT_PUBLIC_DEBUG: debug,
					NEXT_PUBLIC_ROOM_CONFIG: 'DEV',
				}),
			).toThrow('invalid NEXT_PUBLIC_DEBUG')
			expect(() => parseDebugFlag(debug)).toThrow()
		},
	)

	test.each([undefined, '', 'dev', 'STAGING'])(
		'rejects invalid NEXT_PUBLIC_ROOM_CONFIG %s like parseRoomConfigName',
		(roomConfig) => {
			expect(() =>
				validatePublicEnv({
					NEXT_PUBLIC_DEBUG: 'false',
					NEXT_PUBLIC_ROOM_CONFIG: roomConfig,
				}),
			).toThrow('invalid NEXT_PUBLIC_ROOM_CONFIG')
			expect(() => parseRoomConfigName(roomConfig)).toThrow()
		},
	)
})
