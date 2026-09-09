import type { RoomSize } from './room-size'

export const CANVAS_WIDTH = 1080
export const CANVAS_HEIGHT = 1920
export const YOUTUBE_TOP_SAFE_AREA_HEIGHT = 180
export const CONTENT_HORIZONTAL_PADDING = 20
export const SECTION_GAP = 16
export const STATUS_HEIGHT = 64
export const TIMER_HEIGHT = 100
export const COMMANDS_HEIGHT = 80

const CONTENT_WIDTH = CANVAS_WIDTH - CONTENT_HORIZONTAL_PADDING * 2

export const verticalLayout = {
	canvas: {
		width: CANVAS_WIDTH,
		height: CANVAS_HEIGHT,
	},
	topSafeArea: {
		height: YOUTUBE_TOP_SAFE_AREA_HEIGHT,
	},
	room: {
		width: CANVAS_WIDTH,
	},
	content: {
		horizontalPadding: CONTENT_HORIZONTAL_PADDING,
		width: CONTENT_WIDTH,
		gap: SECTION_GAP,
	},
	status: {
		height: STATUS_HEIGHT,
	},
	timer: {
		height: TIMER_HEIGHT,
	},
	commands: {
		height: COMMANDS_HEIGHT,
	},
} as const

export const getVerticalRoomViewport = (source: RoomSize): RoomSize => ({
	width: verticalLayout.room.width,
	height: (source.height / source.width) * verticalLayout.room.width,
})

export const getVerticalSectionPositions = (roomHeight: number) => {
	const roomY = verticalLayout.topSafeArea.height
	const statusY = roomY + roomHeight + verticalLayout.content.gap
	const timerY =
		statusY + verticalLayout.status.height + verticalLayout.content.gap
	const commandsY =
		timerY + verticalLayout.timer.height + verticalLayout.content.gap

	return {
		topSafeArea: {
			y: 0,
			height: verticalLayout.topSafeArea.height,
		},
		room: {
			y: roomY,
			height: roomHeight,
		},
		status: {
			y: statusY,
			height: verticalLayout.status.height,
		},
		timer: {
			y: timerY,
			height: verticalLayout.timer.height,
		},
		commands: {
			y: commandsY,
			height: verticalLayout.commands.height,
		},
		importantContentBottom: commandsY + verticalLayout.commands.height,
	}
}
