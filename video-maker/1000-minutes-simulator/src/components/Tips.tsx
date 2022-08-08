import { FC } from 'react'
import * as styles from '../styles/Tips.styles'
import * as common from '../styles/common.styles'

const Tips: FC = () => {
    return (
        <div css={styles.tips}>
            <div css={styles.innerCell}>
                <div css={common.heading}>Tips</div>
                <div css={styles.tipsMain}>名言・Tips</div>
                <div>さん</div>
                <div>（補足）</div>
            </div>
        </div>
    )
}

export default Tips
