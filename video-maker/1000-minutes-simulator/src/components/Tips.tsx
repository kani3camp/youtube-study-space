import { FC } from 'react'
import { MdTipsAndUpdates } from 'react-icons/md'
import * as styles from '../styles/Tips.styles'
import * as common from '../styles/common.styles'

const Tips: FC = () => {
    return (
        <div css={styles.tips}>
            <div css={styles.innerCell}>
                <div css={common.heading}>
                    <MdTipsAndUpdates
                        size={common.IconSize}
                        css={styles.icon}
                    ></MdTipsAndUpdates>
                    Tips
                </div>
                <div css={styles.tipsMain}>名言・Tips (No. x)</div>
                <div>さん</div>
                <div>（補足）</div>
            </div>
        </div>
    )
}

export default Tips
