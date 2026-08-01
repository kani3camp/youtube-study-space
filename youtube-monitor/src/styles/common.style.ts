import { css } from '@emotion/react'

export const componentBackground = css`
	background-color: var(--ambient-shell-bg, rgba(255, 255, 255, 0.2));
	transition: background-color 30s ease, border-color 30s ease, color 10s ease;
`

export const componentStyle = css`
	position: absolute;
	margin: auto;
	top: 0;
	bottom: 0;
	left: 0;
	right: 0;
	transition: background-color 30s ease, border-color 30s ease, color 10s ease;
`
