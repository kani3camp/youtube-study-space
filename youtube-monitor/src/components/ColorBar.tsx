import type { FC } from 'react'
import * as styles from '../styles/ColorBar.styles'
import { componentBackground, componentStyle } from '../styles/common.style'

const ColorBar: FC = () => {
	return (
		<div css={[styles.shape, componentBackground]}>
			<div css={[styles.colorBar, componentStyle]}>凡例</div>
		</div>
	)
}

export default ColorBar
