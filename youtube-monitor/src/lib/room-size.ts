export type RoomSize = {
	width: number
	height: number
}

export const calculateRoomSize = (
	room: RoomSize,
	viewport: RoomSize,
): RoomSize => {
	const frameRatio = viewport.width / viewport.height
	const roomRatio = room.width / room.height

	return roomRatio >= frameRatio
		? {
				width: viewport.width,
				height: viewport.width / roomRatio,
			}
		: {
				width: viewport.height * roomRatio,
				height: viewport.height,
			}
}
