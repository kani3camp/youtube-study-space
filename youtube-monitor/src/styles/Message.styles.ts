import { css } from '@emotion/react'
import { fontFamily } from '../lib/common'
import { Constants } from '../lib/constants'

export const shape = css`
    height: ${Constants.messageBarHeight}px;
    width: calc(${Constants.screenWidth}px - ${Constants.sideBarWidth}px - ${Constants.tickerWidth}px);
    position: absolute;
    bottom: 0;
    left: 0;
`

export const message = css`
	height: 80%;
	width: 100%;
	padding: 0 5%;
	text-align: center;
	font-size: 1.4rem;
	display: flex;
	flex-direction: row;
	color: var(--ambient-text-primary);
`

export const verticalMessage = css`
	position: absolute;
	inset: 0;
	display: block;
	height: 100%;
	width: 100%;
	padding: 0;
	box-sizing: border-box;
`

export const pageInfo = css`
	width: 45%;
	height: 100%;
	display: flex;
	align-items: center;
	justify-content: center;
`

export const verticalPageInfo = css`
	position: absolute;
	top: 136px;
	left: 300px;
	display: flex;
	align-items: center;
	justify-content: center;
	width: 220px;
	height: 56px;
	gap: 8px;
	padding: 0 12px;
	box-sizing: border-box;
	border: 1px solid var(--ambient-border);
	border-radius: 12px;
	background: var(--ambient-panel-bg);
	backdrop-filter: blur(0.45rem);
	white-space: nowrap;
`

export const pageIndex = css`
	display: inline-block;
`

export const verticalPageIndex = css`
	display: inline-flex;
	align-items: baseline;
	justify-content: center;
	gap: 8px;
`

export const verticalPageLabel = css`
	font-size: 0.65rem;
	font-weight: 600;
`

export const verticalPageNumber = css`
	font-size: 0.95rem;
	font-variant-numeric: tabular-nums;
	line-height: 1;
`

export const memberOnly = css`
    font-family: ${fontFamily};
    width: 2.5rem;
    margin-left: 1rem;
    padding: 0.1rem;
    display: inline-block;
    font-size: 0.6rem;
    color: white;
    background-color: #2ba640;
	border-radius: 0.3rem;
`

export const verticalMemberOnly = css`
	position: absolute;
	top: 6px;
	left: 236px;
	display: flex;
	align-items: center;
	justify-content: center;
	width: 136px;
	height: 44px;
	margin: 0;
	padding: 0 8px;
	box-sizing: border-box;
	font-size: 0.55rem;
	white-space: nowrap;
	border-radius: 999px;
`

export const numStudyingPeople = css`
	width: 45%;
	height: 100%;
	display: inline-block;
	background-color: var(--ambient-panel-strong-bg);
	border: 1px solid var(--ambient-border);
	box-sizing: border-box;
	border-radius: 0.6rem;
	transition:
		background-color var(--ambient-background-transition-duration, 30s) linear,
		border-color var(--ambient-background-transition-duration, 30s) linear;
`

export const verticalNumStudyingPeople = css`
	position: absolute;
	top: 136px;
	left: 736px;
	display: flex;
	align-items: center;
	justify-content: center;
	width: 320px;
	height: 56px;
	padding: 0 12px;
	box-sizing: border-box;
	border: 1px solid var(--ambient-border);
	border-radius: 12px;
	background: var(--ambient-panel-bg);
	backdrop-filter: blur(0.45rem);
	font-size: 0.85rem;
	font-weight: 700;
	white-space: nowrap;
`

export const verticalShape = css`
	position: absolute;
	inset: 0;
	height: auto;
	width: auto;
	pointer-events: none;
`
