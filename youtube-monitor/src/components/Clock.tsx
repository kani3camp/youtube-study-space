import { useTranslation } from 'next-i18next'
import { FC, useState } from 'react'
import { useInterval } from '../lib/common'
import * as styles from '../styles/Clock.styles'
import { componentBackground, componentStyle } from '../styles/common.style'

const Clock: FC = () => {
    const { t } = useTranslation()

    const updateIntervalMilliSec = 1000

    const [now, setNow] = useState<Date>(new Date())

    useInterval(() => {
        setNow(new Date())
    }, updateIntervalMilliSec)

    return (
        <div css={[styles.shape, componentBackground]}>
            <div css={[styles.clockStyle, componentStyle]}>
                <div css={styles.dateStringStyle}>
                    {`${now.getFullYear()}${t('year')}${now.getMonth() + 1}${t(
                        'month'
                    )}${now.getDate()}${t('day')}`}
                </div>
                <div css={styles.timeStringStyle}>
                    {now.getHours()}：
                    {now.getMinutes() < 10 ? `0${now.getMinutes()}` : now.getMinutes()}
                </div>
            </div>
        </div>
    )
}

export default Clock
