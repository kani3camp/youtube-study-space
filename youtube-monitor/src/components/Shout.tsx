import { useTranslation } from 'next-i18next'
import { FC } from 'react'
import * as styles from '../styles/Shout.styles'
import { componentStyle, componentBackground } from '../styles/common.style'

const Shout: FC = () => {
    const { t } = useTranslation()

    const messageText = 'やっほうこれが目標だ！ !shoutで目標を宣言しよう！'
    const username = 'ユーザー'

    return (
        <div css={[styles.shape, componentBackground]}>
            <div css={[styles.shout, componentStyle]}>
                <h3 css={styles.description}>{t('shout.description')}</h3>

                <div css={styles.shoutContent}>
                    <div css={styles.messageText}>{messageText}</div>
                    <div css={styles.userName}>{t('shout.username', { name: username })}</div>
                </div>
            </div>
        </div>
    )
}

export default Shout
