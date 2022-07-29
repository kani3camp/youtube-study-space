import React, { FC } from 'react'
import * as styles from '../styles/CenterLoading.styles'
import { BallTriangle } from 'react-loader-spinner'

const CenterLoading: FC = () => (
  <div css={styles.CenterLoading}>
    <BallTriangle color='#36479f' height={130} width={130} />
  </div>
)

export default CenterLoading
