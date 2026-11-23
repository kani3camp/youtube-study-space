async function loadFontsModule({
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

	return import('./fonts')
}

afterEach(() => {
	vi.resetModules()
	vi.doUnmock('next/font/google')
})

test('font exports return mock classNames from next/font', async () => {
	const { fontClassName, sourceCodeProClassName } = await loadFontsModule({
		mPlusFontFamily: 'M PLUS Rounded 1c',
	})

	expect(fontClassName).toBe('mock-font-class')
	expect(sourceCodeProClassName).toBe('mock-source-code-pro-class')
})

test('fontFamily normalizes an unquoted single family name', async () => {
	const { fontFamily } = await loadFontsModule({
		mPlusFontFamily: 'M PLUS Rounded 1c',
	})

	expect(fontFamily).toBe('"M PLUS Rounded 1c"')
})

test('fontFamily preserves an already quoted family list', async () => {
	const { fontFamily } = await loadFontsModule({
		mPlusFontFamily: '"M PLUS Rounded 1c", "M PLUS Rounded 1c Fallback"',
	})

	expect(fontFamily).toBe('"M PLUS Rounded 1c", "M PLUS Rounded 1c Fallback"')
})

test('sourceCodeProFontFamily is normalized for canvas and CSS usage', async () => {
	const { sourceCodeProFontFamily } = await loadFontsModule({
		mPlusFontFamily: 'M PLUS Rounded 1c',
		sourceCodeProFontFamily: 'Source Code Pro',
	})

	expect(sourceCodeProFontFamily).toBe('"Source Code Pro"')
})

test('fontFamily escapes backslashes and double quotes inside unquoted family names', async () => {
	const { fontFamily } = await loadFontsModule({
		mPlusFontFamily: 'Weird\\Font "X"',
	})

	expect(fontFamily).toBe('"Weird\\\\Font \\"X\\""')
})
