import {
	compositeColors,
	getContrastRatio,
	getRelativeLuminance,
	interpolateColors,
	type RgbaColor,
	rgba,
} from './color-contrast'
import type { TextTone, TimeTheme } from './time-theme'
import {
	AMBIENT_BGM_TEXT_COLOR,
	AMBIENT_THEME_BREAK_ACCENT_COLORS,
	AMBIENT_THEME_COLORS,
	AMBIENT_THEME_STUDY_ACCENT_COLORS,
	AMBIENT_THEME_TEXT_COLORS,
	type AmbientSurfaceColors,
	type AmbientTextColors,
	CONTRAST_BRIDGE_COLORS,
	CONTRAST_BRIDGE_TEXT_COLORS,
	TWILIGHT_TONE_BREAK_ACCENT_COLORS,
	TWILIGHT_TONE_COLORS,
	TWILIGHT_TONE_STUDY_ACCENT_COLORS,
	TWILIGHT_TONE_TEXT_COLORS,
} from './time-theme-colors'

const UNDERLAYS = [
	rgba(255, 255, 255),
	rgba(0, 0, 0),
	rgba(217, 222, 232),
	rgba(45, 51, 69),
]

const READING_SURFACES: (keyof AmbientSurfaceColors)[] = [
	'shell',
	'panel',
	'panelStrong',
	'tickerChip',
]

const assertContrast = (
	text: RgbaColor,
	surface: RgbaColor,
	minimum: number,
) => {
	for (const underlay of UNDERLAYS) {
		const compositedSurface = compositeColors(surface, underlay)
		expect(getContrastRatio(text, compositedSurface)).toBeGreaterThanOrEqual(
			minimum,
		)
	}
}

const getSurfaceColors = (
	theme: TimeTheme,
	tone: TextTone,
): AmbientSurfaceColors =>
	theme === 'twilight'
		? TWILIGHT_TONE_COLORS[tone]
		: AMBIENT_THEME_COLORS[theme]

const getTextColors = (theme: TimeTheme, tone: TextTone): AmbientTextColors =>
	theme === 'twilight'
		? TWILIGHT_TONE_TEXT_COLORS[tone]
		: AMBIENT_THEME_TEXT_COLORS[theme]

const getBreakAccentColor = (theme: TimeTheme, tone: TextTone): RgbaColor =>
	theme === 'twilight'
		? TWILIGHT_TONE_BREAK_ACCENT_COLORS[tone]
		: AMBIENT_THEME_BREAK_ACCENT_COLORS[theme]

const getStudyAccentColor = (theme: TimeTheme, tone: TextTone): RgbaColor =>
	theme === 'twilight'
		? TWILIGHT_TONE_STUDY_ACCENT_COLORS[tone]
		: AMBIENT_THEME_STUDY_ACCENT_COLORS[theme]

const assertAllTextContrast = (
	textColors: AmbientTextColors,
	surface: RgbaColor,
	minimum: number,
) => {
	for (const textColor of Object.values(textColors)) {
		assertContrast(textColor, surface, minimum)
	}
}

const assertNestedSurfaceContrast = (
	text: RgbaColor,
	outerSurface: RgbaColor,
	innerSurface: RgbaColor,
	minimum: number,
) => {
	for (const underlay of UNDERLAYS) {
		const outer = compositeColors(outerSurface, underlay)
		const inner = compositeColors(innerSurface, outer)
		expect(getContrastRatio(text, inner)).toBeGreaterThanOrEqual(minimum)
	}
}

const assertInterpolatedContrast = (
	from: AmbientSurfaceColors,
	to: AmbientSurfaceColors,
	textColors: AmbientTextColors,
	minimum: number,
) => {
	for (const surfaceName of READING_SURFACES) {
		for (let step = 0; step <= 20; step++) {
			assertAllTextContrast(
				textColors,
				interpolateColors(
					from[surfaceName] as RgbaColor,
					to[surfaceName] as RgbaColor,
					step / 20,
				),
				minimum,
			)
		}
	}
}

const assertInterpolatedAccentContrast = (
	getAccentColor: (theme: TimeTheme, tone: TextTone) => RgbaColor,
	fromTheme: TimeTheme,
	fromTone: TextTone,
	toTheme: TimeTheme,
	toTone: TextTone,
	minimum: number,
) => {
	const fromSurface = getSurfaceColors(fromTheme, fromTone).panel
	const toSurface = getSurfaceColors(toTheme, toTone).panel
	const accentColors = [
		getAccentColor(fromTheme, fromTone),
		getAccentColor(toTheme, toTone),
	]

	for (let step = 0; step <= 20; step++) {
		const surface = interpolateColors(fromSurface, toSurface, step / 20)
		for (const accentColor of accentColors) {
			assertContrast(accentColor, surface, minimum)
		}
	}
}

describe('WCAG color utilities', () => {
	test('black/white の相対輝度とコントラスト比を計算する', () => {
		expect(getRelativeLuminance(rgba(0, 0, 0))).toBe(0)
		expect(getRelativeLuminance(rgba(255, 255, 255))).toBe(1)
		expect(getContrastRatio(rgba(0, 0, 0), rgba(255, 255, 255))).toBe(21)
	})

	test('半透明の前景色をsource-overで合成する', () => {
		expect(compositeColors(rgba(255, 255, 255, 0.5), rgba(0, 0, 0))).toEqual({
			r: 127.5,
			g: 127.5,
			b: 127.5,
			a: 1,
		})
	})
})

describe('ambient theme contrast', () => {
	test.each<[TimeTheme, TextTone]>([
		['dawn', 'dark'],
		['day', 'dark'],
		['sunset', 'dark'],
		['twilight', 'dark'],
		['twilight', 'light'],
		['night', 'light'],
		['midnight', 'light'],
	])('%s の作業アクセントがパネル上で4.5:1以上になる', (theme, tone) => {
		assertContrast(
			getStudyAccentColor(theme, tone),
			getSurfaceColors(theme, tone).panel,
			4.5,
		)
	})

	test.each<[TimeTheme, TextTone, TimeTheme, TextTone]>([
		['dawn', 'dark', 'day', 'dark'],
		['day', 'dark', 'sunset', 'dark'],
		['sunset', 'dark', 'twilight', 'dark'],
		['twilight', 'light', 'night', 'light'],
		['night', 'light', 'midnight', 'light'],
	])('%s (%s) → %s (%s) の背景遷移中も作業アクセントが3:1以上を保つ', (fromTheme, fromTone, toTheme, toTone) => {
		assertInterpolatedAccentContrast(
			getStudyAccentColor,
			fromTheme,
			fromTone,
			toTheme,
			toTone,
			3,
		)
	})

	test.each<[TimeTheme, TextTone]>([
		['dawn', 'dark'],
		['day', 'dark'],
		['sunset', 'dark'],
		['twilight', 'dark'],
		['twilight', 'light'],
		['night', 'light'],
		['midnight', 'light'],
	])('%s の休憩アクセントがパネル上で4.5:1以上になる', (theme, tone) => {
		assertContrast(
			getBreakAccentColor(theme, tone),
			getSurfaceColors(theme, tone).panel,
			4.5,
		)
	})

	test.each<[TimeTheme, TextTone, TimeTheme, TextTone]>([
		['dawn', 'dark', 'day', 'dark'],
		['day', 'dark', 'sunset', 'dark'],
		['sunset', 'dark', 'twilight', 'dark'],
		['twilight', 'light', 'night', 'light'],
		['night', 'light', 'midnight', 'light'],
	])('%s (%s) → %s (%s) の背景遷移中も休憩アクセントが3:1以上を保つ', (fromTheme, fromTone, toTheme, toTone) => {
		assertInterpolatedAccentContrast(
			getBreakAccentColor,
			fromTheme,
			fromTone,
			toTheme,
			toTone,
			3,
		)
	})

	test.each<[TimeTheme, TextTone]>([
		['dawn', 'dark'],
		['day', 'dark'],
		['sunset', 'dark'],
		['night', 'light'],
		['midnight', 'light'],
	])('%s の色付き文字が読み取り面で4.5:1以上になる', (theme, tone) => {
		for (const surfaceName of READING_SURFACES) {
			assertAllTextContrast(
				getTextColors(theme, tone),
				getSurfaceColors(theme, tone)[surfaceName] as RgbaColor,
				4.5,
			)
		}
	})

	test.each<TextTone>([
		'dark',
		'light',
	])('twilight の %s 専用面と文字が4.5:1以上になる', (tone) => {
		for (const surfaceName of READING_SURFACES) {
			assertAllTextContrast(
				TWILIGHT_TONE_TEXT_COLORS[tone],
				TWILIGHT_TONE_COLORS[tone][surfaceName] as RgbaColor,
				4.5,
			)
		}
	})

	test.each<TextTone>([
		'dark',
		'light',
	])('コントラストブリッジは %s トーンで4.5:1以上になる', (tone) => {
		for (const surfaceName of READING_SURFACES) {
			assertAllTextContrast(
				CONTRAST_BRIDGE_TEXT_COLORS[tone],
				CONTRAST_BRIDGE_COLORS[surfaceName] as RgbaColor,
				4.5,
			)
		}
	})

	test.each<[TimeTheme, TextTone]>([
		['dawn', 'dark'],
		['day', 'dark'],
		['sunset', 'dark'],
		['twilight', 'dark'],
		['twilight', 'light'],
		['night', 'light'],
		['midnight', 'light'],
	])('%s のコマンド背景で色付き文字が3:1以上になる', (theme, tone) => {
		const surfaces = getSurfaceColors(theme, tone)
		assertNestedSurfaceContrast(
			getTextColors(theme, tone).primary,
			surfaces.panel,
			surfaces.command,
			3,
		)
	})

	test.each<[TimeTheme, TextTone]>([
		['dawn', 'dark'],
		['day', 'dark'],
		['sunset', 'dark'],
		['twilight', 'dark'],
		['twilight', 'light'],
		['night', 'light'],
		['midnight', 'light'],
	])('%s のBGMカードは固定明色文字で4.5:1以上になる', (theme, tone) => {
		assertContrast(
			AMBIENT_BGM_TEXT_COLOR,
			getSurfaceColors(theme, tone).panelDark,
			4.5,
		)
	})

	test.each<[TimeTheme, TextTone, TimeTheme, TextTone]>([
		['dawn', 'dark', 'day', 'dark'],
		['day', 'dark', 'sunset', 'dark'],
		['sunset', 'dark', 'twilight', 'dark'],
		['twilight', 'light', 'night', 'light'],
		['night', 'light', 'midnight', 'light'],
	])('%s → %s の同極性遷移で対象テーマの文字が3:1以上を保つ', (fromTheme, fromTone, toTheme, toTone) => {
		assertInterpolatedContrast(
			getSurfaceColors(fromTheme, fromTone),
			getSurfaceColors(toTheme, toTone),
			getTextColors(toTheme, toTone),
			3,
		)
	})

	test('twilight前半 → ブリッジは安全なdark文字で3:1以上を保つ', () => {
		assertInterpolatedContrast(
			TWILIGHT_TONE_COLORS.dark,
			CONTRAST_BRIDGE_COLORS,
			CONTRAST_BRIDGE_TEXT_COLORS.dark,
			3,
		)
	})

	test('ブリッジ → twilight後半は藤色のlight文字で3:1以上を保つ', () => {
		assertInterpolatedContrast(
			CONTRAST_BRIDGE_COLORS,
			TWILIGHT_TONE_COLORS.light,
			TWILIGHT_TONE_TEXT_COLORS.light,
			3,
		)
	})

	test.each<[TimeTheme, TextTone, TimeTheme, TextTone]>([
		['midnight', 'light', 'dawn', 'dark'],
		['dawn', 'dark', 'night', 'light'],
		['night', 'light', 'day', 'dark'],
		['day', 'dark', 'midnight', 'light'],
	])('%s → %s の直接反転はブリッジ前後で3:1以上を保つ', (fromTheme, fromTone, toTheme, toTone) => {
		assertInterpolatedContrast(
			getSurfaceColors(fromTheme, fromTone),
			CONTRAST_BRIDGE_COLORS,
			CONTRAST_BRIDGE_TEXT_COLORS[fromTone],
			3,
		)
		assertInterpolatedContrast(
			CONTRAST_BRIDGE_COLORS,
			getSurfaceColors(toTheme, toTone),
			getTextColors(toTheme, toTone),
			3,
		)
	})
})
