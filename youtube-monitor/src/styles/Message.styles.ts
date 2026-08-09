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
	grid-template-columns: minmax(0, 1fr) auto;
	align-items: center;
	gap: 24px;
	height: 100%;
	width: 100%;
	padding: 0 0 0 24px;
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
	position: relative;
	display: flex;
	align-items: center;
	justify-content: flex-start;
	width: auto;
	height: 100%;
	gap: 20px;
	padding: 0;
	box-sizing: border-box;
	white-space: nowrap;
`

export const pageIndex = css`
	display: inline-block;
`

export const verticalPageIndex = css`
	display: inline-flex;
	align-items: baseline;
	justify-content: center;
	gap: 10px;
`

export const verticalPageLabel = css`
	font-size: 0.62rem;
	font-weight: 600;
`

export const verticalPageNumber = css`
	font-size: 0.76rem;
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
	position: relative;
	display: flex;
	align-items: center;
	justify-content: center;
	width: auto;
	height: 34px;
	margin: 0;
	padding: 0 14px;
	box-sizing: border-box;
	font-size: 0.48rem;
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
	position: relative;
	display: flex;
	align-items: center;
	justify-content: flex-end;
	width: auto;
	height: 100%;
	padding: 0;
	box-sizing: border-box;
	border: 0;
	border-radius: 0;
	background: transparent;
	font-size: 0.72rem;
	font-weight: 700;
	white-space: nowrap;
`

export const verticalShape = css`
	position: relative;
	inset: auto;
	height: 100%;
	width: 100%;
`
