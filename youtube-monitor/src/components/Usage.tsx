import { useTranslation } from 'next-i18next'
import { FC } from 'react'
import * as styles from '../styles/Usage.styles'
import { componentStyle, componentBackground } from '../styles/common.style'

const Usage: FC = () => {
    const { t } = useTranslation()

    return (
        <div css={[styles.shape, componentBackground]}>
            <div css={[styles.usage, componentStyle]}>
                <h3 css={styles.description}>{t('usage.description')}</h3>
                <div>
                    <div css={styles.item}>
                        <span>{t('usage.in')}</span>
                        <span css={styles.commandString}>!in</span>
                    </div>
                    <div css={styles.item}>
                        <span>{t('usage.out')}</span>
                        <span css={styles.commandString}>!out</span>
                    </div>
                </div>
            </div>
        </div>
    )
}

export default Usage
