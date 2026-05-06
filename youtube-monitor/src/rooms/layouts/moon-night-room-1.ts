import type { RoomLayout } from '../../types/room-layout'

export const MoonNightRoom1: RoomLayout = {
	floor_image: '/images/rooms/moon-night-room-1.png',
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
		{ id: 1, x: 325, y: 276, rotate: 0 },
		{ id: 2, x: 608, y: 276, rotate: 0 },
		{ id: 3, x: 869, y: 276, rotate: 0 },
		{ id: 4, x: 1272, y: 226, rotate: 0 },
		{ id: 5, x: 235, y: 567, rotate: 0 },
		{ id: 6, x: 565, y: 567, rotate: 0 },
		{ id: 7, x: 895, y: 567, rotate: 0 },
		{ id: 8, x: 1201, y: 567, rotate: 0 },
		{ id: 9, x: 118, y: 876, rotate: 0 },
		{ id: 10, x: 515, y: 876, rotate: 0 },
		{ id: 11, x: 912, y: 876, rotate: 0 },
		{ id: 12, x: 1270, y: 876, rotate: 0 },
	],
	partitions: [],
}
