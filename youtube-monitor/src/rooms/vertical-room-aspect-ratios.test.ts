import { calculateRoomSize } from '../lib/room-size'
import { verticalLayout } from '../lib/vertical-layout'

jest.mock('next/font/google', () => ({
	M_PLUS_Rounded_1c: jest.fn(() => ({
		style: { fontFamily: 'M PLUS Rounded 1c' },
		className: 'mock-font-class',
	})),
	Source_Code_Pro: jest.fn(() => ({
		style: { fontFamily: 'Source Code Pro' },
		className: 'mock-source-code-pro-class',
	})),
}))

const originalNextPublicDebug = process.env.NEXT_PUBLIC_DEBUG
const originalNextPublicChannelGl = process.env.NEXT_PUBLIC_CHANNEL_GL
const originalNextPublicRoomConfig = process.env.NEXT_PUBLIC_ROOM_CONFIG

beforeAll(() => {
	process.env.NEXT_PUBLIC_DEBUG = 'true'
	process.env.NEXT_PUBLIC_CHANNEL_GL = 'false'
	process.env.NEXT_PUBLIC_ROOM_CONFIG = 'PROD'
})

afterAll(() => {
	if (originalNextPublicDebug === undefined) {
		delete process.env.NEXT_PUBLIC_DEBUG
	} else {
		process.env.NEXT_PUBLIC_DEBUG = originalNextPublicDebug
	}
	if (originalNextPublicChannelGl === undefined) {
		delete process.env.NEXT_PUBLIC_CHANNEL_GL
	} else {
		process.env.NEXT_PUBLIC_CHANNEL_GL = originalNextPublicChannelGl
	}
	if (originalNextPublicRoomConfig === undefined) {
		delete process.env.NEXT_PUBLIC_ROOM_CONFIG
	} else {
		process.env.NEXT_PUBLIC_ROOM_CONFIG = originalNextPublicRoomConfig
	}
})

describe('vertical room viewport', () => {
	it('fits every configured room at full content width without cropping', () => {
		const { prodAllRooms, testAllRooms } =
			require('./rooms-config') as typeof import('./rooms-config')
		const roomConfigs = [prodAllRooms, testAllRooms]
		const layouts = roomConfigs.flatMap((config) =>
			Object.values(config).flat(),
		)

		for (const layout of layouts) {
			const roomSize = calculateRoomSize(
				layout.room_shape,
				verticalLayout.roomViewport,
			)

			expect(roomSize.width).toBe(verticalLayout.roomViewport.width)
			expect(roomSize.height).toBeLessThanOrEqual(
				verticalLayout.roomViewport.height,
			)
		}
	})
})
