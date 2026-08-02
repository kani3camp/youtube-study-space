import { css } from '@emotion/react'
import { Constants } from '../lib/constants'

export const shape = css`
    height: ${Constants.messageBarHeight}px;
    width: ${Constants.tickerWidth}px;
    position: absolute;
    bottom: 0;
    left: calc(${Constants.screenWidth}px - ${Constants.sideBarWidth}px - ${Constants.tickerWidth}px);
    pointer-events: none; /* 下地と同じレイヤに重ならないように */
    display: flex;
    align-items: center;
    overflow: hidden;
`

export const container = css`
	height: 80%;
	width: 100%;
	backdrop-filter: blur(3px) saturate(1.05);
	box-sizing: border-box;
	overflow: hidden;
	pointer-events: auto;
	padding: 0 10px;
	font-size: 0.8rem;
	color: var(--ambient-text-primary);
	mask-image: linear-gradient(
		to right,
		transparent 0%,
		black 6%,
		black 94%,
		transparent 100%
	);
`

export const marquee = css`
	height: 100%;
	width: 100%;
	display: flex;
	align-items: center;
`

export const genreItem = css`
	display: inline-flex;
	align-items: center;
	margin: 0 0.6rem;
	padding: 0.28rem 0.6rem;
	border-radius: 100vh;
	background-color: var(--ambient-panel-strong-bg);
	background-image: linear-gradient(
		180deg,
		rgba(255, 255, 255, 0.14) 0%,
		transparent 100%
	);
	border: 1px solid var(--ambient-border);
	box-shadow: 0 2px 6px rgba(0, 0, 0, 0.08);
	gap: 0.5rem;
	white-space: nowrap;
	color: var(--ambient-text-primary);
	transition:
		background-color var(--ambient-background-transition-duration, 30s) linear,
		border-color var(--ambient-background-transition-duration, 30s) linear;
`

export const rankBadge = css`
	display: inline-flex;
	align-items: center;
	justify-content: center;
	min-width: 1.4rem;
	height: 1.4rem;
	padding: 0 0.2rem;
	border-radius: 9999px;
	background: linear-gradient(135deg, #fad888 0%, #fbb759 100%);
	color: #3b2c00;
	font-weight: 700;
	box-shadow:
		inset 0 1px 0 rgba(255, 255, 255, 0.6),
		0 1px 2px rgba(0, 0, 0, 0.08);
	font-variant-numeric: tabular-nums;
`

export const genre = css`
	font-weight: 700;
	font-size: 0.95rem;
`

export const count = css`
	display: inline-flex;
	align-items: center;
	gap: 0.25rem;
	padding: 0.1rem 0.4rem;
	border-radius: 999px;
	background: var(--ambient-command-bg);
	color: var(--ambient-text-primary);
	font-weight: 600;
	transition: background-color
		var(--ambient-background-transition-duration, 30s) linear;
`

export const peopleIcon = css`
	font-size: 0.9rem;
	line-height: 1;
`

export const examplesWrapper = css`
	display: inline-flex;
	align-items: center;
	gap: 0.2rem;
	margin-left: 0.2rem;
`

export const exampleChip = css`
	display: inline-flex;
	align-items: center;
	padding: 0.1rem 0.25rem;
	border-radius: 0.3rem;
	background: var(--ambient-ticker-chip-bg);
	color: var(--ambient-text-primary);
	font-size: 0.7rem;
	transition: background-color
		var(--ambient-background-transition-duration, 30s) linear;
`

export const updatedAt = css`
	padding: 0.08rem 0.3rem;
	border-radius: 0.3rem;
	background: var(--ambient-panel-strong-bg);
	color: var(--ambient-text-primary);
	opacity: 0.9;
	white-space: nowrap;
	font-size: 0.5rem;
	transition: background-color
		var(--ambient-background-transition-duration, 30s) linear;
`

export const verticalShape = css`
	position: relative;
	left: auto;
	bottom: auto;
	display: block;
	height: 100%;
	width: 100%;
	min-width: 0;
	overflow: hidden;
`

export const verticalContainer = css`
	display: flex;
	flex-direction: column;
	height: 100%;
	width: 100%;
	min-width: 0;
	padding: 22px 24px 18px;
	box-sizing: border-box;
	gap: 12px;
	backdrop-filter: none;
	mask-image: none;
	overflow: hidden;
`

export const verticalMarqueeViewport = css`
	display: flex;
	align-items: center;
	flex: 1;
	width: 100%;
	min-width: 0;
	overflow: hidden;
	border-radius: 12px;
`

export const verticalMarquee = css`
	min-width: 0;
	overflow: hidden;
`

export const verticalGenreItem = css`
	margin: 0 0.45rem;
`

export const verticalUpdatedAt = css`
	align-self: flex-end;
	margin-right: 4px;
	font-size: 0.48rem;
`

export const verticalEmptyState = css`
	display: flex;
	align-items: center;
	justify-content: center;
	height: 100%;
	width: 100%;
	color: var(--ambient-text-muted);
	font-size: 0.82rem;
	text-align: center;
`
