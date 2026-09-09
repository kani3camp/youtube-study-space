import { css, keyframes } from '@emotion/react'
import { type RgbaColor, toCssRgba } from '../lib/color-contrast'
import { Constants } from '../lib/constants'
import type { TimeTheme } from '../lib/time-theme'
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
} from '../lib/time-theme-colors'
import { verticalLayout } from '../lib/vertical-layout'

const surfaceVariables = (colors: AmbientSurfaceColors) => css`
	--ambient-shell-bg: ${toCssRgba(colors.shell)};
	--ambient-panel-bg: ${toCssRgba(colors.panel)};
	--ambient-panel-strong-bg: ${toCssRgba(colors.panelStrong)};
	--ambient-panel-dark-bg: ${toCssRgba(colors.panelDark)};
	--ambient-border: ${toCssRgba(colors.border)};
	--ambient-highlight: ${toCssRgba(colors.highlight)};
	--ambient-command-bg: ${toCssRgba(colors.command)};
	--ambient-ticker-chip-bg: ${toCssRgba(colors.tickerChip)};
	--ambient-animation-opacity: ${colors.animationOpacity};
`

const textVariables = (colors: AmbientTextColors) => css`
	--ambient-text-primary: ${toCssRgba(colors.primary)};
	--ambient-text-muted: ${toCssRgba(colors.muted)};
	--ambient-notice: ${toCssRgba(colors.notice)};
`

const studyAccentVariable = (color: RgbaColor) => css`
	--ambient-study-accent: ${toCssRgba(color)};
`

const breakAccentVariable = (color: RgbaColor) => css`
	--ambient-break-accent: ${toCssRgba(color)};
`

const themeSelector = (theme: TimeTheme) => css`
	&[data-time-theme='${theme}'] {
		${surfaceVariables(AMBIENT_THEME_COLORS[theme])}
		${textVariables(AMBIENT_THEME_TEXT_COLORS[theme])}
		${studyAccentVariable(AMBIENT_THEME_STUDY_ACCENT_COLORS[theme])}
		${breakAccentVariable(AMBIENT_THEME_BREAK_ACCENT_COLORS[theme])}
	}
`

export const themedRoot = css`
	${surfaceVariables(AMBIENT_THEME_COLORS.day)}
	${textVariables(AMBIENT_THEME_TEXT_COLORS.day)}
	${studyAccentVariable(AMBIENT_THEME_STUDY_ACCENT_COLORS.day)}
	${breakAccentVariable(AMBIENT_THEME_BREAK_ACCENT_COLORS.day)}
	--ambient-bgm-text: ${toCssRgba(AMBIENT_BGM_TEXT_COLOR)};
	--ambient-background-transition-duration: 30s;

	${themeSelector('dawn')}
	${themeSelector('day')}
	${themeSelector('sunset')}
	${themeSelector('twilight')}
	${themeSelector('night')}
	${themeSelector('midnight')}

	/*
	 * twilightは前半と後半で面と文字の両方を同系色にそろえます。
	 * 前半は明るい藤色＋濃いプラム、後半は深い青紫＋淡い藤色です。
	 */
	&[data-time-theme='twilight'][data-text-tone='dark'] {
		${surfaceVariables(TWILIGHT_TONE_COLORS.dark)}
		${textVariables(TWILIGHT_TONE_TEXT_COLORS.dark)}
		${studyAccentVariable(TWILIGHT_TONE_STUDY_ACCENT_COLORS.dark)}
		${breakAccentVariable(TWILIGHT_TONE_BREAK_ACCENT_COLORS.dark)}
	}

	&[data-time-theme='twilight'][data-text-tone='light'] {
		${surfaceVariables(TWILIGHT_TONE_COLORS.light)}
		${textVariables(TWILIGHT_TONE_TEXT_COLORS.light)}
		${studyAccentVariable(TWILIGHT_TONE_STUDY_ACCENT_COLORS.light)}
		${breakAccentVariable(TWILIGHT_TONE_BREAK_ACCENT_COLORS.light)}
	}

	&[data-contrast-bridge='true'] {
		${surfaceVariables(CONTRAST_BRIDGE_COLORS)}
		--ambient-background-transition-duration: 15s;
	}

	/*
	 * 極性反転中だけは安全なほぼ黒・ほぼ白へ寄せます。
	 * 通常テーマへ戻ると、各時間帯の色付き文字へ即時に切り替わります。
	 */
	&[data-contrast-bridge='true'][data-text-tone='dark'] {
		${textVariables(CONTRAST_BRIDGE_TEXT_COLORS.dark)}
		--ambient-study-accent: var(--ambient-text-primary);
		--ambient-break-accent: var(--ambient-text-primary);
	}

	&[data-contrast-bridge='true'][data-text-tone='light'] {
		${textVariables(CONTRAST_BRIDGE_TEXT_COLORS.light)}
		--ambient-study-accent: var(--ambient-text-primary);
		--ambient-break-accent: var(--ambient-text-primary);
	}
`

const driftRight = keyframes`
	0% {
		background-position: 15% 0%, 85% 70%;
		opacity: calc(var(--ambient-animation-opacity) * 0.7);
	}
	100% {
		background-position: 65% 35%, 35% 100%;
		opacity: var(--ambient-animation-opacity);
	}
`

const driftBottom = keyframes`
	0% {
		background-position: 0% 50%, 70% 50%;
		opacity: calc(var(--ambient-animation-opacity) * 0.65);
	}
	100% {
		background-position: 70% 50%, 20% 50%;
		opacity: calc(var(--ambient-animation-opacity) * 0.9);
	}
`

const lightLayer = css`
	position: absolute;
	pointer-events: none;
	z-index: 30;
	overflow: hidden;
	color: var(--ambient-highlight);
	background-repeat: no-repeat;
	transition:
		color var(--ambient-background-transition-duration, 30s) linear,
		opacity var(--ambient-background-transition-duration, 30s) linear;

	@media (prefers-reduced-motion: reduce) {
		animation: none;
		opacity: calc(var(--ambient-animation-opacity) * 0.75);
	}
`

export const rightLight = css`
	${lightLayer};
	top: 0;
	right: 0;
	width: ${Constants.sideBarWidth}px;
	height: ${Constants.screenHeight}px;
	background-image:
		radial-gradient(circle at center, currentColor 0%, transparent 68%),
		linear-gradient(125deg, transparent 20%, currentColor 50%, transparent 80%);
	background-size: 135% 45%, 180% 30%;
	animation: ${driftRight} 78s ease-in-out infinite alternate;
`

export const bottomLight = css`
	${lightLayer};
	bottom: 0;
	left: 0;
	width: ${Constants.screenWidth - Constants.sideBarWidth}px;
	height: ${Constants.messageBarHeight}px;
	background-image:
		radial-gradient(ellipse at center, currentColor 0%, transparent 72%),
		linear-gradient(100deg, transparent 15%, currentColor 50%, transparent 85%);
	background-size: 45% 160%, 55% 100%;
	animation: ${driftBottom} 86s ease-in-out infinite alternate;
`

export const verticalRightLight = css`
	z-index: 0;
	width: 120px;
	height: ${verticalLayout.canvas.height}px;
	opacity: calc(var(--ambient-animation-opacity) * 0.5);
`

export const verticalBottomLight = css`
	z-index: 0;
	width: ${verticalLayout.canvas.width}px;
	height: 160px;
	opacity: calc(var(--ambient-animation-opacity) * 0.5);
`
