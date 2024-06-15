import { useTranslation } from 'next-i18next'
import { FC } from 'react'
import * as styles from '../styles/Usage.styles'
import { componentBackground } from '../styles/common.style'

const Usage: FC = () => {
    const { t } = useTranslation()

    return (
        <div css={[styles.shape, componentBackground]}>
            <div css={styles.usage}>
                <h3 css={styles.description}>{t('usage.description')}</h3>

                <div css={styles.seat}>
                    <div css={styles.seatId}>{t('usage.seat.id')}</div>
                    <div css={styles.workName}>{t('usage.seat.work_name')}</div>
                    <div css={styles.userDisplayName}>{t('usage.seat.user_display_name')}</div>
                </div>

                <div>
                    <div>
                        <span>{t('usage.in')}</span>
                        <span css={styles.commandString}>!in</span>
                    </div>
                    <div>
                        <span>{t('usage.out')}</span>
                        <span css={styles.commandString}>!out</span>
                    </div>
                </div>
            </div>
        </div>
    )
}

export default Usage
