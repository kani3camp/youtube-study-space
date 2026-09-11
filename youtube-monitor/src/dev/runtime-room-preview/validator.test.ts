import { caloRegressionOverlayMap, passingOverlayMap } from './fixtures'
import type { OverlayZone, RuntimeRoomOverlayMap } from './overlay-map'
import {
	boundsOverlap,
	boundsOverlapZone,
	rotatedRectBounds,
	validateRuntimeRoom,
} from './validator'

const expectBoundsCloseTo = (
	actual: ReturnType<typeof rotatedRectBounds>,
	expected: ReturnType<typeof rotatedRectBounds>,
) => {
	expect(actual.left).toBeCloseTo(expected.left)
	expect(actual.top).toBeCloseTo(expected.top)
	expect(actual.right).toBeCloseTo(expected.right)
	expect(actual.bottom).toBeCloseTo(expected.bottom)
}

test('rotate 0 keeps the original rectangle bounds', () => {
	expectBoundsCloseTo(
		rotatedRectBounds({ x: 10, y: 20, width: 140, height: 100, rotate: 0 }),
		{ left: 10, top: 20, right: 150, bottom: 120 },
	)
})

test('positive and negative rotation use the top-left transform origin', () => {
	expectBoundsCloseTo(
		rotatedRectBounds({ x: 10, y: 20, width: 140, height: 100, rotate: 90 }),
		{ left: -90, top: 20, right: 10, bottom: 160 },
	)
	expectBoundsCloseTo(
		rotatedRectBounds({ x: 10, y: 20, width: 140, height: 100, rotate: -90 }),
		{ left: 10, top: -120, right: 110, bottom: 20 },
	)
})

test('touching bounds are allowed while a one-pixel overlap is detected', () => {
	const first = { left: 0, top: 0, right: 140, bottom: 100 }
	expect(
		boundsOverlap(first, { left: 140, top: 0, right: 280, bottom: 100 }),
	).toBe(false)
	expect(
		boundsOverlap(first, { left: 139, top: 0, right: 279, bottom: 100 }),
	).toBe(true)
})

test('touching a rectangular protected zone is allowed', () => {
	const zone: OverlayZone = {
		id: 'zone',
		label: 'zone',
		shape: { type: 'rect', x: 140, y: 0, width: 100, height: 100 },
	}
	expect(
		boundsOverlapZone({ left: 0, top: 0, right: 140, bottom: 100 }, zone),
	).toBe(false)
	expect(
		boundsOverlapZone({ left: 0, top: 0, right: 141, bottom: 100 }, zone),
	).toBe(true)
})

test('polygon zones distinguish border contact from positive-area overlap', () => {
	const polygonPoints = [
		{ x: 140, y: 0 },
		{ x: 240, y: 0 },
		{ x: 240, y: 100 },
		{ x: 140, y: 100 },
	]
	const touchingZone: OverlayZone = {
		id: 'polygon-zone',
		label: 'polygon-zone',
		shape: { type: 'polygon', points: polygonPoints },
	}
	const bounds = { left: 0, top: 0, right: 140, bottom: 100 }
	expect(boundsOverlapZone(bounds, touchingZone)).toBe(false)

	const overlappingZone: OverlayZone = {
		...touchingZone,
		shape: {
			type: 'polygon',
			points: polygonPoints.map((point) => ({
				...point,
				x: point.x - 1,
			})),
		},
	}
	expect(boundsOverlapZone(bounds, overlappingZone)).toBe(true)
})

function overlayMapWithSeats(
	seats: RuntimeRoomOverlayMap['seats'],
): RuntimeRoomOverlayMap {
	return {
		floor_image: '',
		font_size_ratio: 0.015,
		room_shape: { width: 1520, height: 1000 },
		seat_shape: { width: 140, height: 100 },
		partition_shapes: [],
		partitions: [],
		seats,
		runtime_preview: {
			id: 'test',
			name: 'test',
			description: 'test',
			runtime_profile: 'general',
			camera_profile: 'test',
			protected_visual_zones: [],
			seat_zones: [],
			main_circulation: [],
		},
	}
}

test('a seat may touch the room edge but one-pixel overflow is detected', () => {
	const exactEdge = validateRuntimeRoom(
		overlayMapWithSeats([{ id: 1, x: 1380, y: 900, rotate: 0 }]),
		'general',
	)
	expect(exactEdge.overflows).toEqual([])

	const overflow = validateRuntimeRoom(
		overlayMapWithSeats([{ id: 1, x: 1381, y: 900, rotate: 0 }]),
		'general',
	)
	expect(overflow.overflows).toEqual([1])
})

test('all colliding pairs are reported for multiple SeatBoxes', () => {
	const result = validateRuntimeRoom(
		overlayMapWithSeats([
			{ id: 1, x: 0, y: 0, rotate: 0 },
			{ id: 2, x: 139, y: 0, rotate: 0 },
			{ id: 3, x: 278, y: 0, rotate: 0 },
		]),
		'general',
	)
	expect(result.seatCollisions).toEqual([
		{ seatIds: [1, 2] },
		{ seatIds: [2, 3] },
	])
})

test('the passing fixture passes both representative runtime profiles', () => {
	expect(validateRuntimeRoom(passingOverlayMap, 'general').isValid).toBe(true)
	expect(validateRuntimeRoom(passingOverlayMap, 'member').isValid).toBe(true)
})

test('the Calo regression fixture preserves known UI incompatibilities', () => {
	const general = validateRuntimeRoom(caloRegressionOverlayMap, 'general')
	expect(general.seatCollisions).toContainEqual({ seatIds: [4, 5] })
	expect(general.protectedVisualZoneIntrusions).toContainEqual({
		seatId: 7,
		zoneId: 'PVZ-harbor-view',
	})
	expect(general.mainCirculationIntrusions).toEqual(
		expect.arrayContaining([
			{ seatId: 8, zoneId: 'MC-harbor-spine' },
			{ seatId: 9, zoneId: 'MC-harbor-spine' },
		]),
	)
	expect(general.isValid).toBe(false)

	const member = validateRuntimeRoom(caloRegressionOverlayMap, 'member')
	expect(member.seatCollisions.length).toBeGreaterThan(
		general.seatCollisions.length,
	)
	expect(member.isValid).toBe(false)
})
