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
		shell: rgba(216, 229, 251, 0.78),
		panel: rgba(232, 240, 255, 0.78),
		panelStrong: rgba(242, 247, 255, 0.86),
		panelDark: rgba(48, 54, 82, 0.86),
		border: rgba(218, 231, 255, 0.72),
		highlight: rgba(188, 202, 255, 0.48),
		command: rgba(58, 70, 110, 0.12),
		tickerChip: rgba(245, 248, 255, 0.9),
		animationOpacity: 0.13,
	},
	day: {
		shell: rgba(245, 245, 250, 0.78),
		panel: rgba(255, 255, 255, 0.74),
		panelStrong: rgba(255, 255, 255, 0.84),
		panelDark: rgba(45, 43, 54, 0.86),
		border: rgba(255, 255, 255, 0.68),
		highlight: rgba(221, 229, 255, 0.42),
		command: rgba(0, 0, 0, 0.1),
		tickerChip: rgba(255, 255, 255, 0.9),
		animationOpacity: 0.1,
	},
	sunset: {
		shell: rgba(250, 225, 213, 0.8),
		panel: rgba(255, 238, 227, 0.78),
		panelStrong: rgba(255, 246, 238, 0.86),
		panelDark: rgba(73, 53, 57, 0.86),
		border: rgba(255, 217, 194, 0.72),
		highlight: rgba(255, 194, 158, 0.46),
		command: rgba(116, 62, 46, 0.12),
		tickerChip: rgba(255, 249, 243, 0.9),
		animationOpacity: 0.13,
	},
	twilight: {
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
		shell: rgba(37, 43, 82, 0.84),
		panel: rgba(56, 64, 110, 0.84),
		panelStrong: rgba(69, 78, 130, 0.88),
		panelDark: rgba(26, 28, 55, 0.9),
		border: rgba(181, 194, 255, 0.34),
		highlight: rgba(150, 171, 255, 0.38),
		command: rgba(12, 14, 34, 0.32),
		tickerChip: rgba(70, 79, 126, 0.94),
		animationOpacity: 0.1,
	},
	midnight: {
		shell: rgba(29, 33, 59, 0.88),
		panel: rgba(46, 50, 77, 0.88),
		panelStrong: rgba(56, 61, 90, 0.9),
		panelDark: rgba(22, 24, 42, 0.92),
		border: rgba(166, 176, 214, 0.3),
		highlight: rgba(135, 145, 190, 0.32),
		command: rgba(9, 11, 25, 0.34),
		tickerChip: rgba(56, 61, 90, 0.94),
		animationOpacity: 0.075,
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
