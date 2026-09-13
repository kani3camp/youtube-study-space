import type { RoomLayout } from '../../types/room-layout'

export const BookOfficeRoom2: RoomLayout = {
	floor_image: '/images/rooms/book-office-room-2.png',
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
		{ id: 1, x: 84, y: 481, rotate: 0 },
		{ id: 2, x: 330, y: 412, rotate: 0 },
		{ id: 3, x: 577, y: 331, rotate: 0 },
		{ id: 4, x: 1009, y: 384, rotate: 0 },
		{ id: 5, x: 1267, y: 412, rotate: 0 },
		{ id: 6, x: 530, y: 697, rotate: 0 },
		{ id: 7, x: 973, y: 785, rotate: 0 },
	],
	partitions: [],
}
