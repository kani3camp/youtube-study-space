import { useTranslation } from 'next-i18next'
import { FC } from 'react'
import * as styles from '../styles/Usage.styles'
import { componentStyle, componentBackground } from '../styles/common.style'

const Usage: FC = () => {
    const { t } = useTranslation()

    return (
        <div css={[styles.shape, componentBackground]}>
            <div css={[styles.usage, componentStyle]}>
                <h4 css={styles.description}>{t('usage.description')}</h4>

                <div css={styles.seat}>
                    <div css={styles.seatId}>{t('usage.seat.id')}</div>
                    <div css={styles.workName}>{t('usage.seat.work_name')}</div>
                    <div css={styles.userDisplayName}>{t('usage.seat.user_display_name')}</div>
                </div>

                <div>
                    <div css={styles.command}>
                        <span css={styles.commandCode}>!in</span>
                        <span>{t('usage.in')}</span>
                    </div>
                    <div css={styles.command}>
                        <span css={styles.commandCode}>!out</span>
                        <span>{t('usage.out')}</span>
                    </div>
                </div>
            </div>
        </div>
    )
}

export default Usage
