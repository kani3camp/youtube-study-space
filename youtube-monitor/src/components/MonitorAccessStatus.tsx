import { css } from '@emotion/react'
import type { FC } from 'react'

const root = css`
	width: 1920px;
	height: 1080px;
	margin: 0;
	display: flex;
	align-items: center;
	justify-content: center;
	background: #10151f;
	color: #f4f7fb;
`

const panel = css`
	max-width: 980px;
	padding: 64px;
	border: 2px solid rgba(255, 255, 255, 0.28);
	border-radius: 24px;
	background: rgba(0, 0, 0, 0.32);
	text-align: center;
`

const titleStyle = css`
	margin: 0 0 24px;
	font-size: 2rem;
`

const detailStyle = css`
	margin: 0;
	font-size: 1rem;
	line-height: 1.7;
`

type MonitorAccessStatusProps = {
	title: string
	detail: string
}

const MonitorAccessStatus: FC<MonitorAccessStatusProps> = ({
	title,
	detail,
}) => (
	<main css={root} role="status">
		<div css={panel}>
			<h1 css={titleStyle}>{title}</h1>
			<p css={detailStyle}>{detail}</p>
		</div>
	</main>
)

export default MonitorAccessStatus
