import type { FC } from 'react'
import * as styles from '../styles/AmbientFrame.styles'

type AmbientFrameProps = {
	vertical?: boolean
}

const AmbientFrame: FC<AmbientFrameProps> = ({ vertical = false }) => (
	<div aria-hidden="true">
		<div css={[styles.rightLight, vertical && styles.verticalRightLight]} />
		<div css={[styles.bottomLight, vertical && styles.verticalBottomLight]} />
	</div>
)

export default AmbientFrame
