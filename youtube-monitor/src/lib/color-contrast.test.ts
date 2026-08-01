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
	AMBIENT_TEXT_COLORS,
	AMBIENT_THEME_COLORS,
	type AmbientSurfaceColors,
	CONTRAST_BRIDGE_COLORS,
	TWILIGHT_TONE_COLORS,
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

const getTextColors = (tone: TextTone) =>
	Object.values(AMBIENT_TEXT_COLORS[tone])

const getSurfaceColors = (
	theme: TimeTheme,
	tone: TextTone,
): AmbientSurfaceColors =>
	theme === 'twilight'
		? TWILIGHT_TONE_COLORS[tone]
		: AMBIENT_THEME_COLORS[theme]

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
	tone: TextTone,
	minimum: number,
) => {
	for (const surfaceName of READING_SURFACES) {
		for (let step = 0; step <= 20; step++) {
			assertContrast(
				AMBIENT_TEXT_COLORS[tone].primary,
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
		['night', 'light'],
		['midnight', 'light'],
	])('%s の読み取り面で通常トーンが4.5:1以上になる', (theme, tone) => {
		for (const surfaceName of READING_SURFACES) {
			for (const textColor of getTextColors(tone)) {
				assertContrast(
					textColor,
					AMBIENT_THEME_COLORS[theme][surfaceName] as RgbaColor,
					4.5,
				)
			}
		}
	})

	test.each<TextTone>([
		'dark',
		'light',
	])('twilight の %s 専用面で全テキストが4.5:1以上になる', (tone) => {
		for (const surfaceName of READING_SURFACES) {
			for (const textColor of getTextColors(tone)) {
				assertContrast(
					textColor,
					TWILIGHT_TONE_COLORS[tone][surfaceName] as RgbaColor,
					4.5,
				)
			}
		}
	})

	test.each<TextTone>([
		'dark',
		'light',
	])('コントラストブリッジは %s トーンの全テキストで4.5:1以上になる', (tone) => {
		for (const surfaceName of READING_SURFACES) {
			for (const textColor of getTextColors(tone)) {
				assertContrast(
					textColor,
					CONTRAST_BRIDGE_COLORS[surfaceName] as RgbaColor,
					4.5,
				)
			}
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
	])('%s のコマンド背景で %s トーンが3:1以上になる', (theme, tone) => {
		const colors = getSurfaceColors(theme, tone)
		assertNestedSurfaceContrast(
			AMBIENT_TEXT_COLORS[tone].primary,
			colors.panel,
			colors.command,
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

	test('sunset → twilight前半はdarkトーンのまま3:1以上を保つ', () => {
		assertInterpolatedContrast(
			AMBIENT_THEME_COLORS.sunset,
			TWILIGHT_TONE_COLORS.dark,
			'dark',
			3,
		)
	})

	test('twilight前半 → ブリッジはdarkトーンのまま3:1以上を保つ', () => {
		assertInterpolatedContrast(
			TWILIGHT_TONE_COLORS.dark,
			CONTRAST_BRIDGE_COLORS,
			'dark',
			3,
		)
	})

	test('ブリッジ → twilight後半はlightトーンのまま3:1以上を保つ', () => {
		assertInterpolatedContrast(
			CONTRAST_BRIDGE_COLORS,
			TWILIGHT_TONE_COLORS.light,
			'light',
			3,
		)
	})

	test('twilight後半 → nightはlightトーンのまま3:1以上を保つ', () => {
		assertInterpolatedContrast(
			TWILIGHT_TONE_COLORS.light,
			AMBIENT_THEME_COLORS.night,
			'light',
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
			fromTone,
			3,
		)
		assertInterpolatedContrast(
			CONTRAST_BRIDGE_COLORS,
			getSurfaceColors(toTheme, toTone),
			toTone,
			3,
		)
	})
})
