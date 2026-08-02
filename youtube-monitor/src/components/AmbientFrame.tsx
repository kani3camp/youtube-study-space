import type { FC } from 'react'
import { Constants } from '../lib/constants'
import * as styles from '../styles/AmbientFrame.styles'

type AmbientFrameProps = {
	vertical?: boolean
}

const AmbientFrame: FC<AmbientFrameProps> = ({ vertical = false }) => (
	<div aria-hidden="true">
		<div
			css={styles.rightLight}
			style={
				vertical
					? { width: 180, height: Constants.verticalScreenHeight }
					: undefined
			}
		/>
		<div
			css={styles.bottomLight}
			style={
				vertical
					? {
							width: Constants.verticalScreenWidth,
							height: 180,
						}
					: undefined
			}
		/>
	</div>
)

export default AmbientFrame
