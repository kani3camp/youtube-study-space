import { calculateRoomSize } from './room-size'

const assertFitsViewport = (
	room: { width: number; height: number },
	viewport: { width: number; height: number },
) => {
	const result = calculateRoomSize(room, viewport)
	expect(result.width).toBeLessThanOrEqual(viewport.width)
	expect(result.height).toBeLessThanOrEqual(viewport.height)
	expect(result.width / result.height).toBeCloseTo(room.width / room.height)
}

describe('calculateRoomSize', () => {
	test('fits a landscape room in a landscape viewport', () => {
		assertFitsViewport({ width: 16, height: 9 }, { width: 16, height: 9 })
	})

	test('fits a landscape room in a portrait viewport', () => {
		assertFitsViewport({ width: 16, height: 9 }, { width: 9, height: 16 })
	})

	test('fits a portrait room in a landscape viewport', () => {
		assertFitsViewport({ width: 9, height: 16 }, { width: 16, height: 9 })
	})

	test('fits a portrait room in a portrait viewport', () => {
		assertFitsViewport({ width: 9, height: 16 }, { width: 9, height: 16 })
	})

	test('preserves the room aspect ratio', () => {
		const result = calculateRoomSize(
			{ width: 1920, height: 1080 },
			{ width: 1032, height: 810 },
		)
		expect(result.width / result.height).toBeCloseTo(1920 / 1080)
	})
})
