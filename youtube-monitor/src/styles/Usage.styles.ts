import { css } from '@emotion/react'
import {
	Constants,
	sidebarCardHorizontalInsetPx,
	sidebarCardVerticalInsetPx,
} from '../lib/constants'

export const shape = css`
    height: ${Constants.usageHeight}px;
    width: ${Constants.sideBarWidth}px;
    position: absolute;
    top: ${Constants.clockHeight}px;
    right: 0;
`

export const usage = css`
    font-size: 1rem;
    text-align: center;
    color: var(--ambient-text-primary);
    box-sizing: border-box;
    height: calc(100% - ${sidebarCardVerticalInsetPx}px);
    width: calc(100% - ${sidebarCardHorizontalInsetPx}px);
    padding: 0.4rem;
    border-radius: 0.6rem;
    background-color: var(--ambient-panel-bg);
    border: 1px solid var(--ambient-border);
    backdrop-filter: blur(0.5rem);
    display: flex;
    flex-direction: column;
    justify-content: center;
    gap: 0.15rem;
`

export const description = css`
	margin: 0.2rem;
	font-weight: bold;
`

export const command = css`
	margin: 0rem;
`

export const commandCode = css`
	font-weight: 700;
	font-size: 0.8rem;
	letter-spacing: 0.01em;
	font-variant-ligatures: none;
	display: inline-block;
	background-color: var(--ambient-command-bg);
	transition: background-color
		var(--ambient-background-transition-duration, 30s) linear;
	border-radius: 0.22rem;
	padding: 0.12rem 0.5rem;
	margin: 0rem 0.18rem;
	line-height: 1.35;
`

export const commandDesc = css`
	font-size: 0.8rem;
`

export const verticalShape = css`
	position: relative;
	top: auto;
	right: auto;
	height: 100%;
	width: 100%;
`

export const verticalUsage = css`
	position: relative;
	height: 100%;
	width: 100%;
	padding: 0 24px;
	border: 0;
	border-radius: 0;
	background: transparent;
	backdrop-filter: none;
	display: flex;
	align-items: center;
	justify-content: center;
`

export const verticalCommands = css`
	display: flex;
	align-items: center;
	justify-content: center;
	gap: 24px;
	width: 100%;
`

export const verticalCommand = css`
	display: flex;
	align-items: center;
	justify-content: center;
	gap: 16px;
	white-space: nowrap;

	& + & {
		margin-left: 24px;
		padding-left: 48px;
		border-left: 1px solid var(--ambient-border);
	}
`

export const verticalCommandCode = css`
	margin: 0;
	display: inline-flex;
	align-items: center;
	justify-content: center;
	min-width: 88px;
	height: 52px;
	padding: 0 12px;
	box-sizing: border-box;
	border-radius: 10px;
	font-size: 0.85rem;
`

export const verticalCommandDesc = css`
	font-size: 0.78rem;
`
