const canvasWidth = 1080
const canvasHeight = 1920
const outerPadding = 24
const sectionGap = 18
const contentWidth = canvasWidth - outerPadding * 2
const contentHeight = canvasHeight - outerPadding * 2

const fixedRows = {
	header: 116,
	room: 688,
	information: 272,
} as const

const taglineHeight =
	contentHeight -
	Object.values(fixedRows).reduce((total, height) => total + height, 0) -
	sectionGap * 3

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
	sectionGap,
	rows: {
		...fixedRows,
		tagline: taglineHeight,
	},
	headerColumns: {
		clock: 400,
		gap: 18,
		status: 614,
	},
	informationColumns: {
		timer: 340,
		gap: 18,
		details: 674,
	},
	informationRows: {
		usage: 116,
		gap: 18,
		ticker: 138,
	},
	roomViewport: {
		width: contentWidth,
		height: fixedRows.room,
	},
} as const
