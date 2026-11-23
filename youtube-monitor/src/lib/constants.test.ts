import {
	getBgmVolume,
	getRoomConfigName,
	isDebugEnabled,
	parseDebugFlag,
} from './constants'

describe('parseDebugFlag', () => {
	test('accepts only the literal strings true and false', () => {
		expect(parseDebugFlag('true')).toBe(true)
		expect(parseDebugFlag('false')).toBe(false)
		expect(() => parseDebugFlag(undefined)).toThrow('invalid NEXT_PUBLIC_DEBUG')
		expect(() => parseDebugFlag('1')).toThrow('invalid NEXT_PUBLIC_DEBUG')
	})
})

describe('environment-derived settings are evaluated at use time', () => {
	afterEach(() => {
		vi.unstubAllEnvs()
	})

	test('isDebugEnabled and getBgmVolume follow NEXT_PUBLIC_DEBUG', () => {
		vi.stubEnv('NEXT_PUBLIC_DEBUG', 'true')
		expect(isDebugEnabled()).toBe(true)
		expect(getBgmVolume()).toBe(0.1)

		vi.stubEnv('NEXT_PUBLIC_DEBUG', 'false')
		expect(isDebugEnabled()).toBe(false)
		expect(getBgmVolume()).toBe(0.3)
	})

	test('getRoomConfigName rejects an invalid NEXT_PUBLIC_ROOM_CONFIG only when called', () => {
		vi.stubEnv('NEXT_PUBLIC_ROOM_CONFIG', 'PROD')
		expect(getRoomConfigName()).toBe('PROD')

		vi.stubEnv('NEXT_PUBLIC_ROOM_CONFIG', '')
		expect(() => getRoomConfigName()).toThrow('invalid NEXT_PUBLIC_ROOM_CONFIG')
	})
})
