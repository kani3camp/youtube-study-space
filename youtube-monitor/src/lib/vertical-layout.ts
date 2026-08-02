const canvasWidth = 1080
const canvasHeight = 1920
const outerPadding = 24
const gap = 24
const contentWidth = canvasWidth - outerPadding * 2
const contentHeight = canvasHeight - outerPadding * 2

const fixedRows = {
	header: 120,
	room: 600,
	metrics: 260,
	join: 280,
} as const

const taglineHeight =
	contentHeight -
	Object.values(fixedRows).reduce((total, height) => total + height, 0) -
	gap * 4

export const verticalLayout = {
	canvas: {
		width: canvasWidth,
		height: canvasHeight,
	},
	outerPadding,
	content: {
		width: contentWidth,
		height: contentHeight,
	},
	gap,
	rows: {
		...fixedRows,
		tagline: taglineHeight,
	},
	headerColumns: {
		clock: 400,
		gap,
		status: 608,
	},
	metricColumns: {
		timer: 340,
		gap,
		ticker: 668,
	},
	roomViewport: {
		width: contentWidth,
		height: fixedRows.room,
	},
} as const
