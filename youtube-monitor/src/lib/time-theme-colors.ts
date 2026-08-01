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
	twilight: {
		// ライト・ダーク双方の文字を受け渡す中間面なので、明度は維持します。
		shell: rgba(117, 115, 134),
		panel: rgba(116, 116, 134),
		panelStrong: rgba(120, 115, 133),
		panelDark: rgba(52, 49, 68, 0.9),
		border: rgba(202, 197, 224, 0.56),
		highlight: rgba(188, 181, 220, 0.38),
		command: rgba(35, 30, 53, 0.2),
		tickerChip: rgba(115, 116, 134),
		animationOpacity: 0.11,
	},
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

// 明暗両方のテキストで4.5:1以上になる不透明な中間面です。
export const CONTRAST_BRIDGE_COLORS = AMBIENT_THEME_COLORS.twilight

export const AMBIENT_TEXT_COLORS: Readonly<
	Record<TextTone, { primary: RgbaColor; muted: RgbaColor; notice: RgbaColor }>
> = {
	dark: {
		primary: rgba(0, 0, 0),
		muted: rgba(1, 1, 4),
		notice: rgba(1, 0, 4),
	},
	light: {
		primary: rgba(255, 255, 255),
		muted: rgba(254, 254, 255),
		notice: rgba(253, 253, 255),
	},
}

export const AMBIENT_BGM_TEXT_COLOR = rgba(247, 244, 255)
