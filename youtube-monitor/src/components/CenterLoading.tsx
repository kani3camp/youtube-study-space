import type { FC } from 'react'
import * as styles from '../styles/CenterLoading.styles'

const CenterLoading: FC = () => (
	<div css={styles.CenterLoading}>
		<div css={styles.spinner} />
		<div>Loading...</div>
	</div>
)

export default CenterLoading
