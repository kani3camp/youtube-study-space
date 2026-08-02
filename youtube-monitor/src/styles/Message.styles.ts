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
	position: relative;
	display: grid;
	grid-template-rows: 1fr 1fr;
	height: 100%;
	width: 100%;
	padding: 0;
	box-sizing: border-box;
	font-size: 1rem;
`

export const pageInfo = css`
	width: 45%;
	height: 100%;
	display: flex;
	align-items: center;
	justify-content: center;
`

export const verticalPageInfo = css`
	position: relative;
	width: 100%;
	height: 100%;
	gap: 12px;
	padding: 0 24px;
	box-sizing: border-box;
	white-space: nowrap;

	&::after {
		position: absolute;
		bottom: 0;
		left: 8%;
		height: 1px;
		width: 84%;
		background-color: var(--ambient-border);
		content: "";
	}
`

export const pageIndex = css`
	display: inline-block;
`

export const verticalPageIndex = css`
	display: inline-flex;
	align-items: baseline;
	justify-content: center;
	gap: 12px;
`

export const verticalPageLabel = css`
	font-size: 0.82rem;
	font-weight: 600;
`

export const verticalPageNumber = css`
	font-size: 1.08rem;
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
	width: auto;
	margin-left: 0;
	padding: 3px 8px;
	font-size: 0.36rem;
	white-space: nowrap;
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
	display: flex;
	align-items: center;
	justify-content: center;
	width: 100%;
	height: 100%;
	padding: 0 24px;
	box-sizing: border-box;
	border: 0;
	border-radius: 0;
	background: transparent;
	font-size: 1rem;
	font-weight: 600;
	white-space: nowrap;
`

export const verticalShape = css`
	position: relative;
	left: auto;
	bottom: auto;
	height: 100%;
	width: 100%;
`
