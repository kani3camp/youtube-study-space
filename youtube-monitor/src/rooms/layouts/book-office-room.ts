import type { RoomLayout } from '../../types/room-layout'

export const BookOfficeRoom: RoomLayout = {
	floor_image: '/images/rooms/book-office-room.png',
	font_size_ratio: 0.017,
	room_shape: {
		width: 1520,
		height: 1000,
	},
	seat_shape: {
		width: 230,
		height: 150,
	},
	partition_shapes: [],
	seats: [
		{ id: 1, x: 67, y: 331, rotate: 0 },
		{ id: 2, x: 427, y: 331, rotate: 0 },
		{ id: 3, x: 759, y: 331, rotate: 0 },
		{ id: 4, x: 1239, y: 331, rotate: 0 },
		{ id: 5, x: 104, y: 837, rotate: 0 },
		{ id: 6, x: 474, y: 837, rotate: 0 },
		{ id: 7, x: 820, y: 837, rotate: 0 },
		{ id: 8, x: 1210, y: 837, rotate: 0 },
	],
	partitions: [],
}
