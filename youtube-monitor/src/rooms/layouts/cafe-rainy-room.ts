import type { RoomLayout } from '../../types/room-layout'

export const CafeRainyRoom: RoomLayout = {
	floor_image: '/images/rooms/cafe-rainy-room.png',
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
		{ id: 1, x: 263, y: 611, rotate: 0 },
		{ id: 2, x: 443, y: 533, rotate: 0 },
		{ id: 3, x: 632, y: 439, rotate: 0 },
		{ id: 4, x: 794, y: 367, rotate: 0 },
		{ id: 5, x: 899, y: 500, rotate: 0 },
		{ id: 6, x: 1096, y: 574, rotate: 0 },
		{ id: 7, x: 303, y: 760, rotate: 0 },
		{ id: 8, x: 644, y: 873, rotate: 0 },
		{ id: 9, x: 779, y: 653, rotate: 0 },
		{ id: 10, x: 838, y: 873, rotate: 0 },
		{ id: 11, x: 1236, y: 732, rotate: 0 },
	],
	partitions: [],
}
