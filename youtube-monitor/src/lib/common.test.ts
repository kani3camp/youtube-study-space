async function loadCommonModule({
	mPlusFontFamily,
	sourceCodeProFontFamily = 'Source Code Pro',
}: {
	mPlusFontFamily: string
	sourceCodeProFontFamily?: string
}) {
	vi.resetModules()
	vi.doMock('next/font/google', () => ({
		M_PLUS_Rounded_1c: vi.fn(() => ({
			style: { fontFamily: mPlusFontFamily },
			className: 'mock-font-class',
		})),
		Source_Code_Pro: vi.fn(() => ({
			style: { fontFamily: sourceCodeProFontFamily },
			className: 'mock-source-code-pro-class',
		})),
	}))

	return import('./common')
}

afterEach(() => {
	vi.resetModules()
	vi.doUnmock('next/font/google')
})

test('font exports return mock classNames from next/font', async () => {
	const { fontClassName, sourceCodeProClassName } = await loadCommonModule({
		mPlusFontFamily: 'M PLUS Rounded 1c',
	})

	expect(fontClassName).toBe('mock-font-class')
	expect(sourceCodeProClassName).toBe('mock-source-code-pro-class')
})

test('fontFamily normalizes an unquoted single family name', async () => {
	const { fontFamily } = await loadCommonModule({
		mPlusFontFamily: 'M PLUS Rounded 1c',
	})

	expect(fontFamily).toBe('"M PLUS Rounded 1c"')
})

test('fontFamily preserves an already quoted family list', async () => {
	const { fontFamily } = await loadCommonModule({
		mPlusFontFamily: '"M PLUS Rounded 1c", "M PLUS Rounded 1c Fallback"',
	})

	expect(fontFamily).toBe('"M PLUS Rounded 1c", "M PLUS Rounded 1c Fallback"')
})

test('sourceCodeProFontFamily is normalized for canvas and CSS usage', async () => {
	const { sourceCodeProFontFamily } = await loadCommonModule({
		mPlusFontFamily: 'M PLUS Rounded 1c',
		sourceCodeProFontFamily: 'Source Code Pro',
	})

	expect(sourceCodeProFontFamily).toBe('"Source Code Pro"')
})

test('numSeatsOfRoomLayouts', async () => {
	const { numSeatsOfRoomLayouts } = await loadCommonModule({
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
