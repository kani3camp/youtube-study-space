import { css } from '@emotion/react'
import { fontFamily } from '../lib/common'
import {
	Constants,
	sidebarCardHorizontalInsetPx,
	sidebarCardVerticalInsetPx,
} from '../lib/constants'

export const shape = css`
    height: ${Constants.timerHeight}px;
    width: ${Constants.sideBarWidth}px;
    position: absolute;
    top: ${Constants.clockHeight + Constants.usageHeight + Constants.menuHeight}px;
    right: 0;
`

export const timer = css`
    height: calc(100% - ${sidebarCardVerticalInsetPx}px);
    width: calc(100% - ${sidebarCardHorizontalInsetPx}px);
    border-radius: 0.6rem;
    font-size: 0.9rem;
    text-align: center;
    color: var(--ambient-text-primary);
    background-color: var(--ambient-panel-bg);
    border: 1px solid var(--ambient-border);
    box-sizing: border-box;
    display: flex;
    flex-direction: column;
    align-items: center;
    justify-content: center;
    gap: 0.2rem;
`

export const progressBarContainer = css`
	width: 180px;
	height: 180px;
	margin: 0 auto;
`

export const progressInner = css`
	height: 100%;
	display: flex;
	flex-direction: column;
	align-items: center;
	justify-content: center;
	gap: 0.2rem;
`

export const stateRow = css`
	display: flex;
	align-items: center;
	justify-content: center;
	gap: 0.3rem;
	line-height: 1.1;
`

export const stateLabel = css`
	font-size: 0.95rem;
	vertical-align: middle;
	font-weight: bold;
`

export const studyIcon = css`
	color: var(--ambient-study-accent);
	transition: color 150ms linear;
`

export const breakIcon = css`
	color: var(--ambient-break-accent);
	transition: color 150ms linear;
`

export const stateLabelStudy = css`
	color: var(--ambient-study-accent);
	transition: color 150ms linear;
`

export const stateLabelBreak = css`
	color: var(--ambient-break-accent);
	transition: color 150ms linear;
`

export const statePlaceholder = css`
	font-size: 1.1rem;
	font-weight: bold;
	opacity: 0.45;
`

export const remaining = css`
    font-family: ${fontFamily};
    font-size: 1.25rem;
    line-height: 1;
    font-weight: bold;
    letter-spacing: 0.03em;
    font-variant-numeric: tabular-nums;
    display: inline-flex;
    align-items: baseline;
    justify-content: center;
`

export const remainingMinutes = css`
	width: 2ch;
	text-align: right;
`

export const remainingDivider = css`
	width: 0.6ch;
	text-align: center;
`

export const remainingSeconds = css`
	width: 2ch;
	text-align: left;
`

export const remainingPlaceholder = css`
	display: inline-block;
	width: 4.6ch;
	text-align: center;
	opacity: 0.45;
`

export const nextRow = css`
	font-size: 0.65rem;
	text-align: center;
	line-height: 1.2;
`

export const verticalShape = css`
	position: relative;
	top: auto;
	right: auto;
	height: 100%;
	width: 100%;
`

export const verticalTimer = css`
	position: relative;
	height: 100%;
	width: 100%;
	padding: 0 40px 0 24px;
	border: 0;
	border-radius: 0;
	background: transparent;
	display: flex;
	flex-direction: row;
	align-items: center;
	justify-content: center;
`

export const verticalTimerBar = css`
	display: flex;
	align-items: center;
	justify-content: flex-start;
	width: 100%;
	height: 100%;
	gap: 28px;
`

export const verticalTimerSummary = css`
	display: flex;
	align-items: center;
	flex: 0 0 232px;
	gap: 14px;
`

export const verticalTimerIcon = css`
	display: flex;
	align-items: center;
	justify-content: center;
	width: 42px;
	height: 42px;
`

export const verticalTimerDetails = css`
	display: flex;
	flex-direction: column;
	justify-content: center;
	min-width: 0;
	width: 176px;
`

export const verticalStateLabel = css`
	font-size: 0.7rem;
	font-weight: 700;
	line-height: 1.15;
`

export const verticalRemaining = css`
	font-family: ${fontFamily};
	font-size: 1.35rem;
	font-weight: 800;
	font-variant-numeric: tabular-nums;
	line-height: 1.15;
	letter-spacing: 0.03em;
`

export const verticalTimerPlaceholder = css`
	font-size: 1.25rem;
	font-weight: 700;
	opacity: 0.45;
`

export const verticalProgressTrack = css`
	position: relative;
	flex: 1 1 auto;
	min-width: 0;
	height: 22px;
	overflow: hidden;
	border-radius: 999px;
	background: rgba(255, 255, 255, 0.35);
`

export const verticalProgressValue = css`
	height: 100%;
	border-radius: inherit;
	transition:
		width 100ms linear,
		background-color 150ms linear;
`

export const verticalProgressValueStudy = css`
	background: var(--ambient-study-accent);
`

export const verticalProgressValueBreak = css`
	background: var(--ambient-break-accent);
`
