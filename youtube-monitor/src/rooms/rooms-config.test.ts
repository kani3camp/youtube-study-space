import { getAllRooms, resolveRoomConfig, roomConfigs } from './rooms-config'

describe('getAllRooms', () => {
	afterEach(() => vi.unstubAllEnvs())

	test('follows NEXT_PUBLIC_ROOM_CONFIG on every call and caches per config name', () => {
		vi.stubEnv('NEXT_PUBLIC_ROOM_CONFIG', 'DEV')
		const dev = getAllRooms()
		expect(dev).toEqual(resolveRoomConfig(roomConfigs.DEV))
		expect(getAllRooms()).toBe(dev)

		vi.stubEnv('NEXT_PUBLIC_ROOM_CONFIG', 'PROD')
		const prod = getAllRooms()
		expect(prod).toEqual(resolveRoomConfig(roomConfigs.PROD))
		expect(prod).not.toBe(dev)

		vi.stubEnv('NEXT_PUBLIC_ROOM_CONFIG', 'DEV')
		expect(getAllRooms()).toBe(dev)
	})

	test('throws for an invalid config name', () => {
		vi.stubEnv('NEXT_PUBLIC_ROOM_CONFIG', 'INVALID')
		expect(() => getAllRooms()).toThrow('invalid NEXT_PUBLIC_ROOM_CONFIG')
	})
})
