import type { RoomLayout } from '../../types/room-layout'

export const CampRoom: RoomLayout = {
	floor_image: '/images/rooms/camp-room.png',
	font_size_ratio: 0.015,
	room_shape: {
		width: 1520,
		height: 1000,
	},
	seat_shape: {
		width: 140,
		height: 100,
	},
	partition_shapes: [],
	seats: [
		{ id: 1, x: 287, y: 242, rotate: 0 },
		{ id: 2, x: 439, y: 220, rotate: 0 },
		{ id: 3, x: 951, y: 225, rotate: 0 },
		{ id: 4, x: 1111, y: 225, rotate: 0 },
		{ id: 5, x: 870, y: 400, rotate: 0 },
		{ id: 6, x: 279, y: 650, rotate: 0 },
		{ id: 7, x: 587, y: 758, rotate: 0 },
		{ id: 8, x: 730, y: 694, rotate: 0 },
		{ id: 9, x: 1162, y: 686, rotate: 0 },
	],
	partitions: [],
}
