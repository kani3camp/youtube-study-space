import type { FC } from 'react'
import { BallTriangle } from 'react-loader-spinner'
import { Constants } from '../lib/constants'
import * as styles from '../styles/CenterLoading.styles'

const CenterLoading: FC = () => (
	<div css={styles.CenterLoading}>
		<BallTriangle color={Constants.primaryTextColor} height={130} width={130} />
		<div>Loading...</div>
	</div>
)

export default CenterLoading
