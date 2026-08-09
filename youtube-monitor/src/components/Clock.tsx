/** @jsxImportSource @emotion/react */
import { useTranslation } from 'next-i18next/pages'
import { type CSSProperties, type FC, memo, useEffect, useState } from 'react'
import { useInterval } from '../lib/common'
import * as styles from '../styles/Clock.styles'
import { componentBackground, componentStyle } from '../styles/common.style'
import type { MonitorVariant } from './monitor/types'

type ClockProps = {
	variant?: MonitorVariant
	style?: CSSProperties
	time?: Date
}

const Clock: FC<ClockProps> = ({ variant = 'horizontal', style, time }) => {
	const { t } = useTranslation()
	const [now, setNow] = useState<Date | null>(null)
	const isVertical = variant === 'vertical'

	useEffect(() => {
		setNow(time ?? new Date())
	}, [time])

	useInterval(() => {
		setNow((prev) => (prev ? new Date() : null))
	}, 1000)

	return (
		<div
			css={[
				styles.shape,
				isVertical && styles.verticalShape,
				!isVertical && componentBackground,
			]}
			style={style}
		>
			<div
				css={[
					styles.clockStyle,
					isVertical && styles.verticalClockStyle,
					!isVertical && componentStyle,
				]}
			>
				{!isVertical && (
					<div css={styles.dateStringStyle}>
						{now
							? `${now.getFullYear()}${t('year')}${now.getMonth() + 1}${t(
									'month',
								)}${now.getDate()}${t('day')}`
							: '--'}
					</div>
				)}
				<div
					css={[
						styles.timeStringStyle,
						isVertical && styles.verticalTimeStringStyle,
					]}
				>
					{now
						? `${now.getHours()}${isVertical ? ':' : '：'}${
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
