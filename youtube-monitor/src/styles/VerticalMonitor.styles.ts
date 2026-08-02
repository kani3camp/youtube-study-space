import { css } from '@emotion/react'

export const footer = css`
	position: absolute;
	top: 1580px;
	left: 24px;
	width: 912px;
	padding: 28px 48px;
	box-sizing: border-box;
	border-radius: 0.8rem;
	background-color: var(--ambient-panel-bg);
	border: 1px solid var(--ambient-border);
	color: var(--ambient-text-primary);
	text-align: center;
	font-size: 1.1rem;
	line-height: 1.8;
	transition:
		background-color var(--ambient-background-transition-duration, 30s) linear,
		border-color var(--ambient-background-transition-duration, 30s) linear;
`

export const footerTitle = css`
	margin: 0 0 0.4rem;
	font-size: 1.4rem;
	font-weight: 700;
`

export const footerText = css`
	margin: 0;
	color: var(--ambient-text-muted);
`
