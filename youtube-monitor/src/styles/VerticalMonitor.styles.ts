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
	opacity: 0.45;
`

export const layout = css`
	position: relative;
	z-index: 1;
	display: grid;
	grid-template-rows:
		${verticalLayout.rows.header}px
		${verticalLayout.rows.room}px
		${verticalLayout.rows.metrics}px
		${verticalLayout.rows.join}px
		${verticalLayout.rows.tagline}px;
	gap: ${verticalLayout.gap}px;
	height: 100%;
	width: 100%;
	padding: ${verticalLayout.outerPadding}px;
	box-sizing: border-box;
	min-width: 0;
`

export const verticalPanel = css`
	min-width: 0;
	height: 100%;
	box-sizing: border-box;
	overflow: hidden;
	border: 1px solid var(--ambient-border);
	border-radius: 20px;
	background-color: var(--ambient-panel-bg);
	backdrop-filter: blur(0.5rem);
	color: var(--ambient-text-primary);
	transition:
		background-color var(--ambient-background-transition-duration, 30s) linear,
		border-color var(--ambient-background-transition-duration, 30s) linear;
`

export const headerGrid = css`
	display: grid;
	grid-template-columns:
		${verticalLayout.headerColumns.clock}px
		${verticalLayout.headerColumns.status}px;
	gap: ${verticalLayout.headerColumns.gap}px;
	min-width: 0;
`

export const roomSection = css`
	min-width: 0;
	height: 100%;
	border-radius: 20px;
	overflow: hidden;
	background-color: var(--ambient-shell-bg);
	box-shadow: 0 8px 24px rgba(42, 35, 30, 0.12);
`

export const metricsGrid = css`
	display: grid;
	grid-template-columns:
		${verticalLayout.metricColumns.timer}px
		${verticalLayout.metricColumns.ticker}px;
	gap: ${verticalLayout.metricColumns.gap}px;
	min-width: 0;

	> * {
		min-width: 0;
	}
`

export const timerPanel = css`
	display: flex;
	align-items: center;
	justify-content: center;
`

export const tickerPanel = css`
	min-width: 0;
	overflow: hidden;
`

export const joinSection = css`
	display: flex;
	align-items: center;
	justify-content: center;
`

export const taglineSection = css`
	display: flex;
	flex-direction: column;
	align-items: center;
	justify-content: center;
	box-sizing: border-box;
	min-width: 0;
	padding: 24px 64px 96px;
	color: var(--ambient-text-primary);
	text-align: center;
	text-shadow: 0 2px 14px rgba(255, 255, 255, 0.7);
`

export const taglineTitle = css`
	margin: 0 0 18px;
	font-size: 1.55rem;
	font-weight: 700;
	line-height: 1.35;
`

export const taglineText = css`
	margin: 0;
	max-width: 840px;
	color: var(--ambient-text-muted);
	font-size: 0.82rem;
	line-height: 1.7;
`
