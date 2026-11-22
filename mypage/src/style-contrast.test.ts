import { readFileSync } from 'node:fs'
import { describe, expect, it } from 'vitest'

// style.css の配色トークンが WCAG 2.x のコントラスト比を満たすことを検証する。
// 本文テキストは 4.5:1、フォーカスリングなど非テキストの UI 要素は 3:1 を下限とする。

const css = readFileSync(
	new URL('./style.css', import.meta.url),
	'utf8',
).replace(/\/\*[\s\S]*?\*\//g, '')

type RGBA = { r: number; g: number; b: number; a: number }

// トークンは style.css 内でちょうど 1 回だけ宣言されている前提で解決する。
// 局所的な上書きや重複宣言があると、どちらを検証しているか曖昧になるため失敗させる。
function tokenValue(name: string, seen: string[] = []): string {
	if (seen.includes(name))
		throw new Error(`circular var(): ${[...seen, name].join(' -> ')}`)
	const declarations = [
		...css.matchAll(new RegExp(`(?:^|[\\s{;])${name}:\\s*([^;]+);`, 'g')),
	]
	if (declarations.length !== 1)
		throw new Error(
			`token ${name} must be declared exactly once in style.css (found ${declarations.length})`,
		)
	const value = declarations[0][1].trim()
	const ref = value.match(/^var\((--[\w-]+)\)$/)
	return ref ? tokenValue(ref[1], [...seen, name]) : value
}

// sRGB の伝達関数（IEC 61966-2-1）。
const encode = (v: number) =>
	v <= 0.0031308 ? 12.92 * v : 1.055 * v ** (1 / 2.4) - 0.055
const decode = (v: number) =>
	v <= 0.04045 ? v / 12.92 : ((v + 0.055) / 1.055) ** 2.4

// OKLCH をガンマ補正済みの sRGB（0〜1）に変換する。
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
	const toSRGB = (linear: number) => encode(Math.min(1, Math.max(0, linear)))
	return {
		r: toSRGB(4.0767416621 * l_ - 3.3077115913 * m_ + 0.2309699292 * s_),
		g: toSRGB(-1.2684380046 * l_ + 2.6097574011 * m_ - 0.3413193965 * s_),
		b: toSRGB(-0.0041960863 * l_ - 0.7034186147 * m_ + 1.707614701 * s_),
		a,
	}
}

// ブラウザと同じく、ガンマ補正済みの sRGB 上で半透明色を下地へ合成する。
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
	return 0.2126 * decode(r) + 0.7152 * decode(g) + 0.0722 * decode(b)
}

function contrast(fg: RGBA, bg: RGBA): number {
	const [hi, lo] = [luminance(fg), luminance(bg)].sort((x, y) => y - x)
	return (hi + 0.05) / (lo + 0.05)
}

const color = (token: string) => oklchToSRGB(tokenValue(token))
const badgeOn = (cardBg: string) =>
	over(color('--color-badge-bg'), color(cardBg))

describe('配色トークンのコントラスト', () => {
	const textPairs: Array<[string, string, string]> = [
		['基本文字 / ページ背景', '--color-ink', '--color-page'],
		['基本文字 / カード', '--color-ink', '--color-surface'],
		['基本文字 / 作業中カード', '--color-ink', '--color-working-bg'],
		['基本文字 / 休憩中カード', '--color-ink', '--color-break-bg'],
		['基本文字 / お知らせ帯', '--color-ink', '--color-notice-bg'],
		['基本文字 / メンテナンスカード', '--color-ink', '--color-accent-tint'],
		['基本文字 / 選択中の日の列', '--color-ink', '--color-bar-selected-col'],
		['補足文字 / ページ背景', '--color-muted', '--color-page'],
		['補足文字 / カード', '--color-muted', '--color-surface'],
		['補足文字 / 作業中カード', '--color-muted', '--color-working-bg'],
		['補足文字 / 休憩中カード', '--color-muted', '--color-break-bg'],
		['補足文字 / お知らせ帯', '--color-muted', '--color-notice-bg'],
		['補足文字 / メンテナンスカード', '--color-muted', '--color-accent-tint'],
		['補足文字 / 未入室・未登録バッジ', '--color-muted', '--color-chip'],
		['時間単位 / カード', '--color-ink-soft', '--color-surface'],
		['古い値の注記 / カード', '--color-stale-ink', '--color-surface'],
		['古い値の注記 / 作業中カード', '--color-stale-ink', '--color-working-bg'],
		['古い値の注記 / 休憩中カード', '--color-stale-ink', '--color-break-bg'],
		['主ボタン・「作」マークの文字', '--color-surface', '--color-accent'],
		['アクセント文字 / カード', '--color-accent', '--color-surface'],
		[
			'今日ラベル / 選択中の日の列',
			'--color-accent',
			'--color-bar-selected-col',
		],
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
		[
			'休憩中リンク（ホバー） / 休憩中カード',
			'--color-break-ink-hover',
			'--color-break-bg',
		],
	]

	it.each(textPairs)('%s は 4.5:1 以上', (_label, fg, bg) => {
		expect(contrast(color(fg), color(bg))).toBeGreaterThanOrEqual(4.5)
	})

	it.each([
		['--color-working-ink', '--color-working-bg'],
		['--color-break-ink', '--color-break-bg'],
	])('状態バッジの文字 %s は半透明の下地を %s に合成しても 4.5:1 以上', (fg, cardBg) => {
		expect(contrast(color(fg), badgeOn(cardBg))).toBeGreaterThanOrEqual(4.5)
	})

	it.each([
		'--color-page',
		'--color-surface',
		'--color-working-bg',
		'--color-break-bg',
		'--color-notice-bg',
		'--color-bar-selected-col',
	])('フォーカスリングは %s に対して 3:1 以上', (bg) => {
		expect(contrast(color('--color-focus'), color(bg))).toBeGreaterThanOrEqual(
			3,
		)
	})
})
