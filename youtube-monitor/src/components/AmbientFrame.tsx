import type { FC } from 'react'
import * as styles from '../styles/AmbientFrame.styles'

const AmbientFrame: FC = () => (
	<div aria-hidden="true">
		<div css={styles.rightLight} />
		<div css={styles.bottomLight} />
	</div>
)

export default AmbientFrame
