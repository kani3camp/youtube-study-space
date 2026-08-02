import { css } from '@emotion/react'
import {
	Constants,
	sidebarCardHorizontalInsetPx,
	sidebarCardVerticalInsetPx,
} from '../lib/constants'

export const shape = css`
    height: ${Constants.clockHeight}px;
    width: ${Constants.sideBarWidth}px;
    position: absolute;
    top: 0;
    right: 0;
`

export const clockStyle = css`
    height: calc(100% - ${sidebarCardVerticalInsetPx}px);
    width: calc(100% - ${sidebarCardHorizontalInsetPx}px);
    border-radius: 0.6rem;
    background-color: var(--ambient-panel-strong-bg);
    border: 1px solid var(--ambient-border);
    color: var(--ambient-text-primary);
    padding: 0.2rem;
    box-sizing: border-box;
`

export const dateStringStyle = css`
	font-size: 0.6rem;
	text-align: center;
`

export const timeStringStyle = css`
	font-size: 1.4rem;
	text-align: center;
	font-weight: 800;
	line-height: 1.6rem;
`

export const verticalShape = css`
	position: relative;
	top: auto;
	right: auto;
	height: 100%;
	width: 100%;
`

export const verticalClockStyle = css`
	position: relative;
	height: 100%;
	width: 100%;
	padding: 14px 28px;
	border: 0;
	border-radius: 0;
	background: transparent;
	display: flex;
	flex-direction: column;
	align-items: center;
	justify-content: center;
	gap: 2px;
`

export const verticalDateStringStyle = css`
	font-size: 0.58rem;
	line-height: 1.4;
`

export const verticalTimeStringStyle = css`
	font-size: 1.55rem;
	line-height: 1.2;
`
