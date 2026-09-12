import { css } from '@emotion/react'

export const preview = css`
	display: flex;
	flex-direction: column;
	gap: 1rem;
	padding: 1rem;
	min-width: min-content;
	background: #eef1f4;
	color: #18202a;
	font-family: ui-sans-serif, system-ui, sans-serif;
`

export const controls = css`
	display: flex;
	flex-wrap: wrap;
	align-items: center;
	gap: 0.75rem 1rem;
	max-width: 1200px;
	padding: 0.8rem 1rem;
	border: 1px solid #c7ced6;
	border-radius: 0.5rem;
	background: white;
`

export const field = css`
	display: flex;
	align-items: center;
	gap: 0.4rem;
	font-size: 0.85rem;
`

export const profileButtons = css`
	display: flex;
	overflow: hidden;
	border: 1px solid #83909d;
	border-radius: 0.35rem;
`

export const profileButton = css`
	padding: 0.42rem 0.7rem;
	border: 0;
	border-right: 1px solid #83909d;
	background: #f8fafc;
	cursor: pointer;

	&:last-of-type {
		border-right: 0;
	}
`

export const selectedProfileButton = css`
	background: #244d73;
	color: white;
`

export const fixtureDescription = css`
	flex-basis: 100%;
	margin: 0;
	color: #4b5866;
	font-size: 0.8rem;
`

export const scaledFrame = css`
	position: relative;
	overflow: hidden;
	border: 1px solid #8d99a5;
	background: #202731;
`

export const fullFrame = css`
	position: absolute;
	top: 0;
	left: 0;
	transform-origin: top left;
	background: #17202a;
`

export const roomRegion = css`
	position: absolute;
	top: 0;
	left: 0;
	overflow: hidden;
	background: linear-gradient(145deg, #dbe5e5 0%, #cadad3 48%, #b9c9d0 100%);
`

export const cleanImagePlaceholder = css`
	position: absolute;
	inset: 0;
	background-image:
		linear-gradient(rgba(255, 255, 255, 0.18) 1px, transparent 1px),
		linear-gradient(90deg, rgba(255, 255, 255, 0.18) 1px, transparent 1px);
	background-size: 100px 100px;
`

export const frameRegion = css`
	display: flex;
	align-items: center;
	justify-content: center;
	box-sizing: border-box;
	border: 3px dashed rgba(255, 255, 255, 0.42);
	background: #26333e;
	color: rgba(255, 255, 255, 0.82);
	font-size: 32px;
	font-weight: 700;
	letter-spacing: 0.04em;
`

export const sidebarRegion = css`
	position: absolute;
	top: 0;
	right: 0;
`

export const bottomRegion = css`
	position: absolute;
	bottom: 0;
`

export const overlay = css`
	position: absolute;
	inset: 0;
	z-index: 100;
	pointer-events: none;
`

export const statusPanel = css`
	display: grid;
	grid-template-columns: repeat(5, minmax(135px, auto));
	gap: 0.45rem;
	max-width: 1200px;
	padding: 0.8rem 1rem;
	border: 1px solid #c7ced6;
	border-radius: 0.5rem;
	background: white;
	font-size: 0.78rem;
`

export const statusSummary = css`
	grid-column: 1 / -1;
	margin: 0;
	font-weight: 700;
`

export const valid = css`
	color: #116636;
`

export const invalid = css`
	color: #a11a1a;
`

export const statusItem = css`
	margin: 0;
`
