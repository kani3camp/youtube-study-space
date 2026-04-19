function loadCommonModule({
	mPlusFontFamily,
	sourceCodeProFontFamily = 'Source Code Pro',
}: {
	mPlusFontFamily: string
	sourceCodeProFontFamily?: string
}) {
	jest.resetModules()
	jest.doMock('next/font/google', () => ({
		M_PLUS_Rounded_1c: jest.fn(() => ({
			style: { fontFamily: mPlusFontFamily },
			className: 'mock-font-class',
		})),
		Source_Code_Pro: jest.fn(() => ({
			style: { fontFamily: sourceCodeProFontFamily },
			className: 'mock-source-code-pro-class',
		})),
	}))

	return require('./common') as typeof import('./common')
}

afterEach(() => {
	jest.resetModules()
	jest.unmock('next/font/google')
})

test('font exports return mock classNames from next/font', () => {
	const { fontClassName, sourceCodeProClassName } = loadCommonModule({
		mPlusFontFamily: 'M PLUS Rounded 1c',
	})

	expect(fontClassName).toBe('mock-font-class')
	expect(sourceCodeProClassName).toBe('mock-source-code-pro-class')
})

test('fontFamily normalizes an unquoted single family name', () => {
	const { fontFamily } = loadCommonModule({
		mPlusFontFamily: 'M PLUS Rounded 1c',
	})

	expect(fontFamily).toBe('"M PLUS Rounded 1c"')
})

test('fontFamily preserves an already quoted family list', () => {
	const { fontFamily } = loadCommonModule({
		mPlusFontFamily: '"M PLUS Rounded 1c", "M PLUS Rounded 1c Fallback"',
	})

	expect(fontFamily).toBe('"M PLUS Rounded 1c", "M PLUS Rounded 1c Fallback"')
})

test('sourceCodeProFontFamily is normalized for canvas and CSS usage', () => {
	const { sourceCodeProFontFamily } = loadCommonModule({
		mPlusFontFamily: 'M PLUS Rounded 1c',
		sourceCodeProFontFamily: 'Source Code Pro',
	})

	expect(sourceCodeProFontFamily).toBe('"Source Code Pro"')
})

test('numSeatsOfRoomLayouts', () => {
	const { numSeatsOfRoomLayouts } = loadCommonModule({
		mPlusFontFamily: 'M PLUS Rounded 1c',
	})

	expect(numSeatsOfRoomLayouts([])).toBe(0)
	expect(
		numSeatsOfRoomLayouts([
			roomLayoutWithSeats([
				{ id: 1, x: 0, y: 0, rotate: 0 },
				{ id: 2, x: 0, y: 0, rotate: 0 },
			]),
		]),
	).toBe(2)
})

const roomLayoutWithSeats = (
	seats: { id: number; x: number; y: number; rotate: number }[],
) => ({
	floor_image: '',
	version: 0,
	font_size_ratio: 1,
	room_shape: {
		height: 0,
		width: 0,
	},
	seat_shape: {
		height: 0,
		width: 0,
	},
	partition_shapes: [],
	seats,
	partitions: [],
})
