import { css } from '@emotion/react'

export const componentBackground = css`
	background-color: var(--ambient-shell-bg, rgba(255, 255, 255, 0.2));
	transition:
		background-color var(--ambient-background-transition-duration, 30s) linear,
		border-color var(--ambient-background-transition-duration, 30s) linear;
`

export const componentStyle = css`
	position: absolute;
	margin: auto;
	top: 0;
	bottom: 0;
	left: 0;
	right: 0;
	transition:
		background-color var(--ambient-background-transition-duration, 30s) linear,
		border-color var(--ambient-background-transition-duration, 30s) linear;
`
