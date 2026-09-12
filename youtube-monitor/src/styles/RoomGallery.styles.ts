import { css } from '@emotion/react'
import { fontFamily } from '../lib/common'

export const page = css`
	min-height: 100vh;
	box-sizing: border-box;
	padding: clamp(0.9rem, 1.75vw, 1.6rem);
	color: #302a25;
	background: #f4f0e9;
	font-family: ${fontFamily};
`

export const header = css`
	max-width: 1800px;
	margin: 0 auto 1rem;
`

export const eyebrow = css`
	margin: 0 0 0.2rem;
	color: #8c6c4a;
	font-size: 0.62rem;
	font-weight: 800;
	letter-spacing: 0.14em;
	text-transform: uppercase;
`

export const title = css`
	margin: 0;
	color: #302a25;
	font-size: clamp(1.3rem, 2.2vw, 1.85rem);
	line-height: 1.1;
`

export const description = css`
	max-width: 55rem;
	margin: 0.4rem 0 0;
	color: #695e54;
	font-size: 0.78rem;
	line-height: 1.45;
`

export const toolbar = css`
	display: flex;
	flex-wrap: wrap;
	align-items: end;
	gap: 0.7rem;
	max-width: 1800px;
	margin: 0 auto 1.1rem;
	padding: 0.7rem 0.85rem;
	border: 1px solid rgba(117, 91, 62, 0.18);
	border-radius: 0.8rem;
	background: rgba(255, 252, 247, 0.82);
	box-shadow: 0 0.75rem 2rem rgba(102, 79, 54, 0.08);
`

export const controlGroup = css`
	display: flex;
	flex-direction: column;
	gap: 0.25rem;
`

export const controlLabel = css`
	color: #806e5d;
	font-size: 0.58rem;
	font-weight: 800;
	letter-spacing: 0.08em;
	text-transform: uppercase;
`

export const segmentedControl = css`
	display: inline-flex;
	flex-wrap: wrap;
	gap: 0.2rem;
	padding: 0.2rem;
	border-radius: 0.7rem;
	background: #ebe3d8;
`

export const segment = css`
	padding: 0.32rem 0.58rem;
	border: 0;
	border-radius: 0.5rem;
	color: #65594e;
	background: transparent;
	font: inherit;
	font-size: 0.7rem;
	cursor: pointer;

	&:hover,
	&:focus-visible {
		background: rgba(255, 255, 255, 0.65);
	}
`

export const activeSegment = css`
	color: #fffaf3;
	background: #79583c;
	font-weight: 800;
	box-shadow: 0 0.2rem 0.5rem rgba(91, 63, 38, 0.22);
`

export const workspace = css`
	display: grid;
	grid-template-columns: minmax(0, 2fr) minmax(18rem, 1fr);
	align-items: start;
	gap: 1rem;
	max-width: 1800px;
	margin: 0 auto;

	@media (max-width: 980px) {
		grid-template-columns: minmax(0, 1fr);
	}
`

export const list = css`
	min-width: 0;
`

export const section = css`
	margin-bottom: 1.2rem;
`

export const sectionHeader = css`
	display: flex;
	align-items: baseline;
	justify-content: space-between;
	gap: 1rem;
	margin-bottom: 0.55rem;
`

export const sectionTitle = css`
	margin: 0;
	color: #3c3026;
	font-size: 0.9rem;
`

export const sectionCount = css`
	color: #8d7965;
	font-size: 0.64rem;
`

export const cards = css`
	display: grid;
	grid-template-columns: repeat(auto-fill, minmax(min(100%, 10rem), 1fr));
	gap: 0.6rem;
`

export const roomCard = css`
	min-width: 0;
	padding: 0.35rem;
	border: 1px solid rgba(117, 91, 62, 0.18);
	border-radius: 0.7rem;
	color: inherit;
	background: rgba(255, 252, 247, 0.88);
	font: inherit;
	text-align: left;
	box-shadow: 0 0.45rem 1.25rem rgba(102, 79, 54, 0.06);
	cursor: pointer;
	transition:
		transform 160ms ease,
		box-shadow 160ms ease,
		border-color 160ms ease;

	&:hover,
	&:focus-visible {
		border-color: rgba(121, 88, 60, 0.55);
		box-shadow: 0 0.7rem 1.5rem rgba(102, 79, 54, 0.14);
		transform: translateY(-2px);
	}
`

export const selectedRoomCard = css`
	border-color: #8d6139;
	box-shadow: 0 0 0 2px rgba(141, 97, 57, 0.18);
`

export const previewShell = css`
	position: relative;
	width: 100%;
	overflow: hidden;
	border-radius: 0.55rem;
	background:
		radial-gradient(
			circle at 20% 15%,
			rgba(255, 255, 255, 0.85),
			transparent 40%
		),
		#d9d0c2;
`

export const previewCanvas = css`
	position: absolute;
	top: 0;
	left: 0;
	transform-origin: top left;
	pointer-events: none;
`

export const noImageLabel = css`
	position: absolute;
	top: 0.4rem;
	right: 0.4rem;
	z-index: 2;
	padding: 0.2rem 0.35rem;
	border: 1px solid rgba(66, 53, 42, 0.2);
	border-radius: 999px;
	color: #5d4b3b;
	background: rgba(255, 250, 239, 0.86);
	font-size: 0.52rem;
	font-weight: 800;
	letter-spacing: 0.04em;
`

export const cardBody = css`
	display: flex;
	flex-direction: column;
	gap: 0.25rem;
	padding: 0.4rem 0.15rem 0.15rem;
`

export const cardTitleRow = css`
	display: flex;
	align-items: center;
	justify-content: space-between;
	gap: 0.35rem;
`

export const cardTitle = css`
	min-width: 0;
	overflow: hidden;
	color: #382c23;
	font-size: 0.75rem;
	font-weight: 800;
	text-overflow: ellipsis;
	white-space: nowrap;
`

export const roomId = css`
	color: #987d62;
	font-family: monospace;
	font-size: 0.58rem;
`

export const badgeRow = css`
	display: flex;
	flex-wrap: wrap;
	gap: 0.2rem;
`

export const badge = css`
	display: inline-flex;
	align-items: center;
	padding: 0.16rem 0.3rem;
	border-radius: 999px;
	color: #685a4c;
	background: #eee6db;
	font-size: 0.52rem;
	font-weight: 800;
	line-height: 1;
`

export const enabledBadge = css`
	color: #376d51;
	background: #dceee1;
`

export const disabledBadge = css`
	color: #766b61;
	background: #e9e5df;
`

export const cardMeta = css`
	display: flex;
	justify-content: space-between;
	gap: 0.5rem;
	color: #887767;
	font-size: 0.58rem;
`

export const detailPanel = css`
	position: sticky;
	top: 1rem;
	min-width: 0;
	padding: 0.85rem;
	border: 1px solid rgba(117, 91, 62, 0.2);
	border-radius: 1rem;
	background: rgba(255, 252, 247, 0.92);
	box-shadow: 0 0.9rem 2.5rem rgba(102, 79, 54, 0.1);

	@media (max-width: 980px) {
		position: static;
	}
`

export const detailHeading = css`
	display: flex;
	align-items: start;
	justify-content: space-between;
	gap: 1rem;
	margin-bottom: 0.8rem;
`

export const detailTitle = css`
	margin: 0;
	color: #302a25;
	font-size: 1.15rem;
`

export const detailDescription = css`
	margin: 0.25rem 0 0;
	color: #806f60;
	font-size: 0.72rem;
`

export const detailPreview = css`
	margin-bottom: 0.9rem;
	border: 1px solid rgba(117, 91, 62, 0.2);
	box-shadow: inset 0 0 0 1px rgba(255, 255, 255, 0.25);
`

export const detailMeta = css`
	display: grid;
	grid-template-columns: repeat(2, minmax(0, 1fr));
	gap: 0.55rem;
	margin: 0;
`

export const detailMetaItem = css`
	margin: 0;
	padding: 0.55rem 0.65rem;
	border-radius: 0.65rem;
	background: #f2ece3;
`

export const detailMetaLabel = css`
	display: block;
	margin-bottom: 0.18rem;
	color: #927b63;
	font-size: 0.62rem;
	font-weight: 800;
	letter-spacing: 0.06em;
	text-transform: uppercase;
`

export const detailMetaValue = css`
	color: #40342a;
	font-size: 0.78rem;
	font-weight: 700;
`
