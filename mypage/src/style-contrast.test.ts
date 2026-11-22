import { readFileSync } from 'node:fs'
import { describe, expect, it } from 'vitest'

// style.css の配色トークンが WCAG 2.x のコントラスト比を満たすことを検証する。
// 本文テキストは 4.5:1、フォーカスリングなど非テキストの UI 要素は 3:1 を下限とする。

const css = readFileSync(new URL('./style.css', import.meta.url), 'utf8')

type RGBA = { r: number; g: number; b: number; a: number }

function tokenValue(name: string): string {
	const match = css.match(new RegExp(`${name}:\\s*([^;]+);`))
	if (!match) throw new Error(`token ${name} is not defined in style.css`)
	const value = match[1].trim()
	const ref = value.match(/^var\((--[\w-]+)\)$/)
	return ref ? tokenValue(ref[1]) : value
}

function oklchToSRGB(value: string): RGBA {
	const match = value.match(
		/^oklch\(\s*([\d.]+)%\s+([\d.]+)\s+([\d.]+)\s*(?:\/\s*([\d.]+))?\s*\)$/,
	)
	if (!match) throw new Error(`unsupported color: ${value}`)
	const l = Number(match[1]) / 100
	const c = Number(match[2])
	const h = (Number(match[3]) * Math.PI) / 180
	const a = match[4] === undefined ? 1 : Number(match[4])
	const okA = c * Math.cos(h)
	const okB = c * Math.sin(h)
	const l_ = (l + 0.3963377774 * okA + 0.2158037573 * okB) ** 3
	const m_ = (l - 0.1055613458 * okA - 0.0638541728 * okB) ** 3
	const s_ = (l - 0.0894841775 * okA - 1.291485548 * okB) ** 3
	const clamp = (v: number) => Math.min(1, Math.max(0, v))
	return {
		r: clamp(4.0767416621 * l_ - 3.3077115913 * m_ + 0.2309699292 * s_),
		g: clamp(-1.2684380046 * l_ + 2.6097574011 * m_ - 0.3413193965 * s_),
		b: clamp(-0.0041960863 * l_ - 0.7034186147 * m_ + 1.707614701 * s_),
		a,
	}
}

// 線形 sRGB 上で半透明色を下地へ合成する（ブラウザの合成とは厳密には異なるが、
// 下限の判定には十分な近似）。
function over(top: RGBA, bottom: RGBA): RGBA {
	const mix = (t: number, b: number) => t * top.a + b * (1 - top.a)
	return {
		r: mix(top.r, bottom.r),
		g: mix(top.g, bottom.g),
		b: mix(top.b, bottom.b),
		a: 1,
	}
}

function luminance({ r, g, b }: RGBA): number {
	return 0.2126 * r + 0.7152 * g + 0.0722 * b
}

function contrast(fg: RGBA, bg: RGBA): number {
	const [hi, lo] = [luminance(fg), luminance(bg)].sort((x, y) => y - x)
	return (hi + 0.05) / (lo + 0.05)
}

const color = (token: string) => oklchToSRGB(tokenValue(token))
const badgeOn = (cardBg: string) =>
	over(oklchToSRGB('oklch(99.2% 0.004 305 / 0.8)'), color(cardBg))

describe('配色トークンのコントラスト', () => {
	const textPairs: Array<[string, string, string]> = [
		['基本文字 / ページ背景', '--color-ink', '--color-page'],
		['基本文字 / カード', '--color-ink', '--color-surface'],
		['基本文字 / 作業中カード', '--color-ink', '--color-working-bg'],
		['基本文字 / 休憩中カード', '--color-ink', '--color-break-bg'],
		['補足文字 / ページ背景', '--color-muted', '--color-page'],
		['補足文字 / カード', '--color-muted', '--color-surface'],
		['補足文字 / 作業中カード', '--color-muted', '--color-working-bg'],
		['補足文字 / 休憩中カード', '--color-muted', '--color-break-bg'],
		['補足文字 / お知らせ帯', '--color-muted', '--color-notice-bg'],
		['時間単位 / カード', '--color-ink-soft', '--color-surface'],
		['古い値の注記 / カード', '--color-stale-ink', '--color-surface'],
		['主ボタン・「作」マークの文字', '--color-surface', '--color-accent'],
		['アクセント文字 / カード', '--color-accent', '--color-surface'],
		['今日の合計 / カード', '--color-accent-ink-deep', '--color-surface'],
		['アバター文字', '--color-accent-ink', '--color-accent-soft'],
		['リンク / ページ背景', '--color-link', '--color-page'],
		['リンク / カード', '--color-link', '--color-surface'],
		['リンク / 作業中カード', '--color-link', '--color-working-bg'],
		['リンク / お知らせ帯', '--color-link', '--color-notice-bg'],
		['リンク（ホバー） / カード', '--color-link-hover', '--color-surface'],
		[
			'作業中ラベル / 作業中カード',
			'--color-working-ink',
			'--color-working-bg',
		],
		[
			'休憩中ラベル・リンク / 休憩中カード',
			'--color-break-ink',
			'--color-break-bg',
		],
	]

	it.each(textPairs)('%s は 4.5:1 以上', (_label, fg, bg) => {
		expect(contrast(color(fg), color(bg))).toBeGreaterThanOrEqual(4.5)
	})

	it('状態バッジの文字は半透明の下地を合成しても 4.5:1 以上', () => {
		expect(
			contrast(color('--color-working-ink'), badgeOn('--color-working-bg')),
		).toBeGreaterThanOrEqual(4.5)
		expect(
			contrast(color('--color-break-ink'), badgeOn('--color-break-bg')),
		).toBeGreaterThanOrEqual(4.5)
	})

	it.each([
		'--color-page',
		'--color-surface',
		'--color-working-bg',
		'--color-break-bg',
	])('フォーカスリングは %s に対して 3:1 以上', (bg) => {
		expect(contrast(color('--color-focus'), color(bg))).toBeGreaterThanOrEqual(
			3,
		)
	})
})
