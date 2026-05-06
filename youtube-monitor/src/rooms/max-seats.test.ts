import {
	buildRoomLayouts,
	desiredMaxSeatsByVacancyRate,
	desiredMemberMaxSeats,
	expandSeatsWithTemporaryRooms,
} from './max-seats'

test('expandSeatsWithTemporaryRooms cycles temporary rooms until minSeats is covered', () => {
	expect(
		expandSeatsWithTemporaryRooms(12, 5, [
			roomLayoutWithSeatCount(2),
			roomLayoutWithSeatCount(3),
		]),
	).toBe(12)
})

test('desiredMaxSeatsByVacancyRate keeps basic seats when vacancy target is already covered', () => {
	expect(
		desiredMaxSeatsByVacancyRate(4, 0.5, 10, [roomLayoutWithSeatCount(5)]),
	).toBe(10)
})

test('desiredMaxSeatsByVacancyRate adds temporary seats to satisfy vacancy target', () => {
	expect(
		desiredMaxSeatsByVacancyRate(9, 0.25, 10, [
			roomLayoutWithSeatCount(1),
			roomLayoutWithSeatCount(4),
		]),
	).toBe(15)
})

test('desiredMemberMaxSeats returns 0 when membership is disabled', () => {
	expect(
		desiredMemberMaxSeats(false, false, 10, 0.5, 8, [
			roomLayoutWithSeatCount(4),
		]),
	).toBe(0)
})

test('desiredMemberMaxSeats returns basic room seats when fixed max seats is enabled', () => {
	expect(
		desiredMemberMaxSeats(true, true, 10, 0.5, 8, [roomLayoutWithSeatCount(4)]),
	).toBe(8)
})

test('buildRoomLayouts returns only basic rooms when fixed max seats is enabled', () => {
	const basicRoom = roomLayoutWithSeatCount(4)
	const temporaryRoom = roomLayoutWithSeatCount(4)

	expect(buildRoomLayouts([basicRoom], [temporaryRoom], 8, true)).toEqual([
		basicRoom,
	])
})

test('buildRoomLayouts appends temporary rooms until maxSeats is covered', () => {
	const basicRoom = roomLayoutWithSeatCount(4)
	const temporaryRooms = [
		roomLayoutWithSeatCount(2),
		roomLayoutWithSeatCount(3),
	]

	expect(buildRoomLayouts([basicRoom], temporaryRooms, 9, false)).toEqual([
		basicRoom,
		temporaryRooms[0],
		temporaryRooms[1],
	])
})

const roomLayoutWithSeatCount = (seatCount: number) => ({
	floor_image: '',
	font_size_ratio: 1,
	room_shape: {
		height: 0,
		width: 0,
	},
	seat_shape: {
		height: 0,
		width: 0,
	},
	partition_shapes: [],
	seats: Array.from({ length: seatCount }, (_, index) => ({
		id: index + 1,
		x: 0,
		y: 0,
		rotate: 0,
	})),
	partitions: [],
})
