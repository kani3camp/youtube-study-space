import type { RoomLayout } from '../../types/room-layout'

export const MemberBoxRooms3: RoomLayout = {
	floor_image: '/images/member_box_rooms3.png',
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
		{ id: 1, x: 50, y: 328, rotate: 0 },
		{ id: 2, x: 440, y: 328, rotate: 0 },
		{ id: 3, x: 820, y: 275, rotate: 0 },
		{ id: 4, x: 1270, y: 275, rotate: 0 },
		{ id: 5, x: 80, y: 848, rotate: 0 },
		{ id: 6, x: 430, y: 848, rotate: 0 },
		{ id: 7, x: 760, y: 840, rotate: 0 },
		{ id: 8, x: 1020, y: 850, rotate: 0 },
		{ id: 9, x: 1290, y: 840, rotate: 0 },
	],
	partitions: [],
}
