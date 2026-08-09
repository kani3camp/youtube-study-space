const canvasWidth = 1080
const canvasHeight = 1920

export const verticalLayout = {
	canvas: {
		width: canvasWidth,
		height: canvasHeight,
	},
	room: {
		x: 0,
		y: 0,
		width: 1080,
		height: 720,
	},
	hud: {
		clock: {
			x: 24,
			y: 136,
			width: 208,
			height: 56,
		},
		page: {
			x: 300,
			y: 136,
			width: 220,
			height: 56,
		},
		member: {
			x: 536,
			y: 142,
			width: 136,
			height: 44,
		},
		workers: {
			x: 736,
			y: 136,
			width: 320,
			height: 56,
		},
	},
	timer: {
		x: 20,
		y: 740,
		width: 1040,
		height: 120,
	},
	usage: {
		x: 20,
		y: 876,
		width: 1040,
		height: 96,
	},
	importantContentBottom: 972,
	roomViewport: {
		width: 1080,
		height: 720,
	},
} as const
