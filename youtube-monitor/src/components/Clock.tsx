/** @jsxImportSource @emotion/react */
import { useTranslation } from 'next-i18next'
import { type FC, memo, useEffect, useState } from 'react'
import { useInterval } from '../lib/common'
import * as styles from '../styles/Clock.styles'
import { componentBackground, componentStyle } from '../styles/common.style'

const Clock: FC = () => {
	const { t } = useTranslation()
	const [now, setNow] = useState<Date | null>(null)

	useEffect(() => {
		setNow(new Date())
	}, [])

	useInterval(() => {
		setNow((prev) => (prev ? new Date() : null))
	}, 1000)

	return (
		<div css={[styles.shape, componentBackground]}>
			<div css={[styles.clockStyle, componentStyle]}>
				<div css={styles.dateStringStyle}>
					{now
						? `${now.getFullYear()}${t('year')}${now.getMonth() + 1}${t(
								'month',
							)}${now.getDate()}${t('day')}`
						: '--'}
				</div>
				<div css={styles.timeStringStyle}>
					{now
						? `${now.getHours()}：${
								now.getMinutes() < 10
									? `0${now.getMinutes()}`
									: now.getMinutes()
							}`
						: '--:--'}
				</div>
			</div>
		</div>
	)
}

export default memo(Clock)
