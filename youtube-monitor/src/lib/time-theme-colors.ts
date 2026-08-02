import { type RgbaColor, rgba } from './color-contrast'
import type { TextTone, TimeTheme } from './time-theme'

export type AmbientSurfaceColors = {
	shell: RgbaColor
	panel: RgbaColor
	panelStrong: RgbaColor
	panelDark: RgbaColor
	border: RgbaColor
	highlight: RgbaColor
	command: RgbaColor
	tickerChip: RgbaColor
	animationOpacity: number
}

export type AmbientTextColors = {
	primary: RgbaColor
	muted: RgbaColor
	notice: RgbaColor
}

export const AMBIENT_THEME_STUDY_ACCENT_COLORS: Readonly<
	Record<TimeTheme, RgbaColor>
> = {
	dawn: rgba(122, 46, 36),
	day: rgba(118, 40, 24),
	sunset: rgba(116, 43, 36),
	// twilight前半の既定値。後半はTWILIGHT_TONE_STUDY_ACCENT_COLORS.lightを使います。
	twilight: rgba(73, 29, 25),
	night: rgba(255, 199, 188),
	midnight: rgba(255, 208, 200),
}

export const TWILIGHT_TONE_STUDY_ACCENT_COLORS: Readonly<
	Record<TextTone, RgbaColor>
> = {
	dark: AMBIENT_THEME_STUDY_ACCENT_COLORS.twilight,
	light: rgba(255, 208, 200),
}

export const AMBIENT_THEME_BREAK_ACCENT_COLORS: Readonly<
	Record<TimeTheme, RgbaColor>
> = {
	dawn: rgba(23, 90, 67),
	day: rgba(15, 85, 56),
	sunset: rgba(33, 75, 56),
	// twilight前半の既定値。後半はTWILIGHT_TONE_BREAK_ACCENT_COLORS.lightを使います。
	twilight: rgba(22, 48, 32),
	night: rgba(183, 247, 197),
	midnight: rgba(196, 249, 208),
}

export const TWILIGHT_TONE_BREAK_ACCENT_COLORS: Readonly<
	Record<TextTone, RgbaColor>
> = {
	dark: AMBIENT_THEME_BREAK_ACCENT_COLORS.twilight,
	light: rgba(196, 249, 208),
}

export const TWILIGHT_TONE_COLORS: Readonly<
	Record<TextTone, AmbientSurfaceColors>
> = {
	dark: {
		// 薄暮前半。明るい藤色の面で濃いプラム色の文字を見せます。
		shell: rgba(146, 138, 174, 0.94),
		panel: rgba(156, 147, 185, 0.96),
		panelStrong: rgba(171, 159, 199, 0.97),
		panelDark: rgba(48, 42, 72, 0.94),
		border: rgba(220, 208, 246, 0.62),
		highlight: rgba(210, 178, 246, 0.36),
		command: rgba(52, 39, 82, 0.16),
		tickerChip: rgba(178, 165, 204, 0.98),
		animationOpacity: 0.1,
	},
	light: {
		// 薄暮後半。深い青紫の面へ移り、淡い藤色の文字で夜へつなぎます。
		shell: rgba(65, 57, 98, 0.94),
		panel: rgba(74, 64, 111, 0.96),
		panelStrong: rgba(86, 74, 128, 0.97),
		panelDark: rgba(31, 27, 52, 0.95),
		border: rgba(174, 157, 220, 0.44),
		highlight: rgba(164, 139, 231, 0.34),
		command: rgba(18, 14, 38, 0.28),
		tickerChip: rgba(92, 79, 136, 0.98),
		animationOpacity: 0.09,
	},
}

export const AMBIENT_THEME_COLORS: Readonly<
	Record<TimeTheme, AmbientSurfaceColors>
> = {
	dawn: {
		// 冷たい青紫の空気に、朝日を思わせる淡い桃色の光を混ぜます。
		shell: rgba(211, 224, 242, 0.8),
		panel: rgba(235, 241, 249, 0.82),
		panelStrong: rgba(246, 248, 252, 0.88),
		panelDark: rgba(48, 54, 82, 0.88),
		border: rgba(210, 228, 242, 0.75),
		highlight: rgba(255, 205, 178, 0.42),
		command: rgba(58, 70, 110, 0.12),
		tickerChip: rgba(245, 248, 255, 0.92),
		animationOpacity: 0.12,
	},
	day: {
		// 澄んだ空色と自然光。ライトテーマの中で最も明るく清潔に見せます。
		shell: rgba(200, 225, 239, 0.8),
		panel: rgba(241, 249, 253, 0.82),
		panelStrong: rgba(250, 253, 255, 0.9),
		panelDark: rgba(40, 48, 58, 0.88),
		border: rgba(210, 237, 248, 0.75),
		highlight: rgba(166, 220, 242, 0.45),
		command: rgba(20, 75, 95, 0.1),
		tickerChip: rgba(248, 253, 255, 0.92),
		animationOpacity: 0.1,
	},
	sunset: {
		// オレンジ一色にせず、珊瑚色と桃色の間に寄せます。
		shell: rgba(245, 216, 202, 0.82),
		panel: rgba(253, 235, 223, 0.82),
		panelStrong: rgba(255, 246, 237, 0.9),
		panelDark: rgba(73, 53, 57, 0.88),
		border: rgba(255, 207, 184, 0.74),
		highlight: rgba(255, 166, 123, 0.46),
		command: rgba(116, 62, 46, 0.12),
		tickerChip: rgba(255, 247, 240, 0.92),
		animationOpacity: 0.13,
	},
	// twilightは文字トーン別の色をCSS側で上書きします。
	twilight: TWILIGHT_TONE_COLORS.dark,
	night: {
		// 活動時間の夜。青紫を明確にし、midnightより少し鮮やかに保ちます。
		shell: rgba(36, 48, 82, 0.86),
		panel: rgba(51, 64, 104, 0.88),
		panelStrong: rgba(63, 78, 122, 0.9),
		panelDark: rgba(24, 31, 58, 0.92),
		border: rgba(158, 181, 255, 0.36),
		highlight: rgba(143, 168, 255, 0.38),
		command: rgba(12, 14, 34, 0.32),
		tickerChip: rgba(66, 78, 122, 0.95),
		animationOpacity: 0.09,
	},
	midnight: {
		// 深夜は単に暗くするだけでなく、彩度と光量を落として静けさを出します。
		shell: rgba(24, 31, 52, 0.9),
		panel: rgba(36, 43, 67, 0.91),
		panelStrong: rgba(46, 54, 80, 0.92),
		panelDark: rgba(18, 22, 38, 0.94),
		border: rgba(139, 151, 194, 0.3),
		highlight: rgba(105, 119, 168, 0.3),
		command: rgba(9, 11, 25, 0.34),
		tickerChip: rgba(46, 54, 80, 0.95),
		animationOpacity: 0.065,
	},
}

export const AMBIENT_THEME_TEXT_COLORS: Readonly<
	Record<TimeTheme, AmbientTextColors>
> = {
	dawn: {
		primary: rgba(43, 32, 74),
		muted: rgba(48, 37, 66),
		notice: rgba(59, 29, 53),
	},
	day: {
		primary: rgba(16, 46, 60),
		muted: rgba(26, 46, 55),
		notice: rgba(13, 47, 59),
	},
	sunset: {
		primary: rgba(62, 27, 48),
		muted: rgba(58, 39, 49),
		notice: rgba(66, 25, 44),
	},
	// twilight前半の既定値。後半はTWILIGHT_TONE_TEXT_COLORS.lightを使います。
	twilight: {
		primary: rgba(36, 21, 50),
		muted: rgba(31, 24, 48),
		notice: rgba(42, 20, 54),
	},
	night: {
		primary: rgba(243, 240, 255),
		muted: rgba(225, 223, 242),
		notice: rgba(233, 223, 255),
	},
	midnight: {
		primary: rgba(232, 236, 248),
		muted: rgba(214, 221, 234),
		notice: rgba(223, 230, 248),
	},
}

export const TWILIGHT_TONE_TEXT_COLORS: Readonly<
	Record<TextTone, AmbientTextColors>
> = {
	dark: AMBIENT_THEME_TEXT_COLORS.twilight,
	light: {
		primary: rgba(245, 240, 255),
		muted: rgba(232, 224, 242),
		notice: rgba(241, 225, 251),
	},
}

// 極性反転中だけ使用します。中間面の両側で4.5:1以上を守るため、
// ここだけは色味を極めて薄くし、安全性を優先します。
export const CONTRAST_BRIDGE_TEXT_COLORS: Readonly<
	Record<TextTone, AmbientTextColors>
> = {
	dark: {
		primary: rgba(4, 0, 6),
		muted: rgba(3, 0, 5),
		notice: rgba(5, 0, 7),
	},
	light: {
		primary: rgba(255, 255, 255),
		muted: rgba(255, 254, 255),
		notice: rgba(255, 253, 255),
	},
}

// 文字極性を切り替える瞬間だけ使う、明暗双方で4.5:1以上の中間面です。
export const CONTRAST_BRIDGE_COLORS: AmbientSurfaceColors = {
	shell: rgba(117, 115, 134),
	panel: rgba(116, 116, 134),
	panelStrong: rgba(120, 115, 133),
	panelDark: rgba(52, 49, 68, 0.9),
	border: rgba(202, 197, 224, 0.56),
	highlight: rgba(188, 181, 220, 0.38),
	command: rgba(35, 30, 53, 0.2),
	tickerChip: rgba(115, 116, 134),
	animationOpacity: 0.1,
}

export const AMBIENT_BGM_TEXT_COLOR = rgba(247, 244, 255)
