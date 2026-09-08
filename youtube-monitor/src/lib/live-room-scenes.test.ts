import { resolveLiveRoomScenesEnabled } from './live-room-scenes'

describe('resolveLiveRoomScenesEnabled', () => {
	test('stays disabled until router query state is ready', () => {
		expect(
			resolveLiveRoomScenesEnabled({
				defaultEnabled: true,
				queryValue: undefined,
				debugEnabled: false,
				routerReady: false,
			}),
		).toBe(false)
	})

	test('uses the environment default when no override is present', () => {
		expect(
			resolveLiveRoomScenesEnabled({
				defaultEnabled: true,
				queryValue: undefined,
				debugEnabled: false,
				routerReady: true,
			}),
		).toBe(true)
		expect(
			resolveLiveRoomScenesEnabled({
				defaultEnabled: false,
				queryValue: undefined,
				debugEnabled: false,
				routerReady: true,
			}),
		).toBe(false)
	})

	test.each(['off', 'false', '0'])(
		'allows a production-safe URL kill switch: %s',
		(queryValue) => {
			expect(
				resolveLiveRoomScenesEnabled({
					defaultEnabled: true,
					queryValue,
					debugEnabled: false,
					routerReady: true,
				}),
			).toBe(false)
		},
	)

	test.each(['on', 'true', '1'])(
		'allows URL force-on only in debug builds: %s',
		(queryValue) => {
			expect(
				resolveLiveRoomScenesEnabled({
					defaultEnabled: false,
					queryValue,
					debugEnabled: false,
					routerReady: true,
				}),
			).toBe(false)
			expect(
				resolveLiveRoomScenesEnabled({
					defaultEnabled: false,
					queryValue,
					debugEnabled: true,
					routerReady: true,
				}),
			).toBe(true)
		},
	)
})
