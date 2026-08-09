import { css } from '@emotion/react'
import { verticalLayout } from '../lib/vertical-layout'

export const canvas = css`
	height: ${verticalLayout.canvas.height}px;
	width: ${verticalLayout.canvas.width}px;
	margin: 0;
	position: relative;
	isolation: isolate;
	box-sizing: border-box;
	overflow: hidden;
`

export const backgroundOverlay = css`
	position: absolute;
	inset: 0;
	z-index: 0;
	pointer-events: none;
	background: var(--ambient-shell-bg);
	opacity: 0.3;
`

export const layout = css`
	position: relative;
	z-index: 1;
	display: flex;
	flex-direction: column;
	height: 100%;
	width: 100%;
	min-width: 0;
`

export const topSafeArea = css`
	flex: 0 0 ${verticalLayout.topSafeArea.height}px;
	width: 100%;
	pointer-events: none;
`

export const roomSection = css`
	flex: none;
	width: ${verticalLayout.room.width}px;
	min-height: 0;
	background: var(--ambient-shell-bg);
`

export const controlsStack = css`
	display: flex;
	flex: none;
	flex-direction: column;
	gap: ${verticalLayout.content.gap}px;
	width: ${verticalLayout.content.width}px;
	margin: ${verticalLayout.content.gap}px auto 0;
`

export const verticalPanel = css`
	box-sizing: border-box;
	min-width: 0;
	overflow: hidden;
	border: 1px solid var(--ambient-border);
	border-radius: 14px;
	background: var(--ambient-panel-bg);
	backdrop-filter: blur(0.5rem);
	color: var(--ambient-text-primary);
	transition:
		background-color var(--ambient-background-transition-duration, 30s) linear,
		border-color var(--ambient-background-transition-duration, 30s) linear;
`

export const statusPanel = css`
	display: grid;
	grid-template-columns: 176px 1px minmax(0, 1fr);
	align-items: center;
	height: ${verticalLayout.status.height}px;
	padding: 0 20px;
`

export const statusDivider = css`
	width: 1px;
	height: 32px;
	background: var(--ambient-border);
`

export const timerPanel = css`
	height: ${verticalLayout.timer.height}px;
`

export const commandsPanel = css`
	height: ${verticalLayout.commands.height}px;
`
