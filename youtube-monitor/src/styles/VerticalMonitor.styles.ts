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
	height: 100%;
	width: 100%;
	min-width: 0;
`

export const roomSection = css`
	position: absolute;
	top: ${verticalLayout.room.y}px;
	left: ${verticalLayout.room.x}px;
	width: ${verticalLayout.room.width}px;
	height: ${verticalLayout.room.height}px;
	overflow: hidden;
	background: var(--ambient-shell-bg);
`

export const hudLayer = css`
	position: absolute;
	inset: 0;
	z-index: 4;
	pointer-events: none;
`

export const hudPanel = css`
	position: absolute;
	box-sizing: border-box;
	display: flex;
	align-items: center;
	justify-content: center;
	min-width: 0;
	overflow: hidden;
	border: 1px solid var(--ambient-border);
	border-radius: 12px;
	background: var(--ambient-panel-bg);
	backdrop-filter: blur(0.45rem);
	color: var(--ambient-text-primary);
	transition:
		background-color var(--ambient-background-transition-duration, 30s) linear,
		border-color var(--ambient-background-transition-duration, 30s) linear;
`

export const clockHud = css`
	top: ${verticalLayout.hud.clock.y}px;
	left: ${verticalLayout.hud.clock.x}px;
	width: ${verticalLayout.hud.clock.width}px;
	height: ${verticalLayout.hud.clock.height}px;
`

export const timerPanel = css`
	position: absolute;
	top: ${verticalLayout.timer.y}px;
	left: ${verticalLayout.timer.x}px;
	width: ${verticalLayout.timer.width}px;
	height: ${verticalLayout.timer.height}px;
	box-sizing: border-box;
	overflow: hidden;
	border: 1px solid var(--ambient-border);
	border-radius: 16px;
	background: var(--ambient-panel-bg);
	backdrop-filter: blur(0.5rem);
	color: var(--ambient-text-primary);
	transition:
		background-color var(--ambient-background-transition-duration, 30s) linear,
		border-color var(--ambient-background-transition-duration, 30s) linear;
`

export const usagePanel = css`
	position: absolute;
	top: ${verticalLayout.usage.y}px;
	left: ${verticalLayout.usage.x}px;
	width: ${verticalLayout.usage.width}px;
	height: ${verticalLayout.usage.height}px;
	box-sizing: border-box;
	overflow: hidden;
	border: 1px solid var(--ambient-border);
	border-radius: 16px;
	background: var(--ambient-panel-bg);
	backdrop-filter: blur(0.5rem);
	color: var(--ambient-text-primary);
	transition:
		background-color var(--ambient-background-transition-duration, 30s) linear,
		border-color var(--ambient-background-transition-duration, 30s) linear;
`
