/** @jsxImportSource @emotion/react */
import { useTranslation } from 'next-i18next/pages'
import {
	type CSSProperties,
	memo,
	useCallback,
	useEffect,
	useMemo,
	useState,
} from 'react'
import {
	buildStyles,
	CircularProgressbarWithChildren,
} from 'react-circular-progressbar'
import { AiFillFire } from 'react-icons/ai'
import { MdFreeBreakfast } from 'react-icons/md'
import { useInterval } from '../lib/common'
import { Constants } from '../lib/constants'
import {
	computeRemaining,
	FALLBACK_REMAINING,
	formatRemainingTime,
} from '../lib/timer'
import { componentBackground, componentStyle } from '../styles/common.style'
import * as styles from '../styles/Timer.styles'
import type { MonitorVariant } from './monitor/types'

const UPDATE_INTERVAL_MS = 100

type TimerProps = {
	variant?: MonitorVariant
	style?: CSSProperties
}

const Timer = memo(function Timer({
	variant = 'horizontal',
	style,
}: TimerProps) {
	const { t } = useTranslation()
	const [now, setNow] = useState<Date | null>(null)
	const isVertical = variant === 'vertical'

	useEffect(() => {
		setNow(new Date())
	}, [])

	useInterval(
		useCallback(() => {
			setNow((prev) => (prev ? new Date() : null))
		}, []),
		UPDATE_INTERVAL_MS,
	)

	const { remainingSec, percentage, isStudy, nextLabel, nextDurationMin } =
		useMemo(() => (now ? computeRemaining(now) : FALLBACK_REMAINING), [now])

	const { minutes, seconds } = formatRemainingTime(remainingSec)
	const isReady = now !== null

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
					styles.timer,
					isVertical && styles.verticalTimer,
					!isVertical && componentStyle,
				]}
			>
				{isVertical ? (
					<div css={styles.verticalTimerBar}>
						<div css={styles.verticalTimerSummary}>
							<div css={styles.verticalTimerIcon}>
								{isReady ? (
									isStudy ? (
										<AiFillFire size={30} css={styles.studyIcon} />
									) : (
										<MdFreeBreakfast size={30} css={styles.breakIcon} />
									)
								) : (
									<span css={styles.verticalTimerPlaceholder}>--</span>
								)}
							</div>
							<div css={styles.verticalTimerDetails}>
								<div
									css={[
										styles.verticalStateLabel,
										isStudy ? styles.stateLabelStudy : styles.stateLabelBreak,
									]}
								>
									{isReady ? t(isStudy ? 'study' : 'break') : '--'}
								</div>
								<div css={styles.verticalRemaining}>
									{isReady ? `${minutes}:${seconds}` : '--:--'}
								</div>
							</div>
						</div>
						<div css={styles.verticalProgressTrack}>
							<div
								css={[
									styles.verticalProgressValue,
									isStudy
										? styles.verticalProgressValueStudy
										: styles.verticalProgressValueBreak,
								]}
								style={{ width: `${isReady ? percentage : 0}%` }}
							/>
						</div>
					</div>
				) : (
					<div css={styles.progressBarContainer}>
						<CircularProgressbarWithChildren
							value={isReady ? percentage : 0}
							strokeWidth={10}
							styles={buildStyles({
								strokeLinecap: 'round',
								pathTransitionDuration: 0,
								pathColor: isStudy
									? Constants.timerProgressStudyColor
									: Constants.timerProgressBreakColor,
								trailColor: 'rgba(255,255,255,0.35)',
								backgroundColor: 'transparent',
							})}
						>
							<div css={styles.progressInner}>
								<div css={styles.stateRow}>
									{isReady ? (
										isStudy ? (
											<>
												<AiFillFire size={22} css={styles.studyIcon} />
												<span css={[styles.stateLabel, styles.stateLabelStudy]}>
													{t('study')}
												</span>
											</>
										) : (
											<>
												<MdFreeBreakfast size={22} css={styles.breakIcon} />
												<span css={[styles.stateLabel, styles.stateLabelBreak]}>
													{t('break')}
												</span>
											</>
										)
									) : (
										<span css={styles.statePlaceholder}>--</span>
									)}
								</div>
								<div css={styles.remaining}>
									{isReady ? (
										<>
											<span css={styles.remainingMinutes}>{minutes}</span>
											<span css={styles.remainingDivider}>:</span>
											<span css={styles.remainingSeconds}>{seconds}</span>
										</>
									) : (
										<span css={styles.remainingPlaceholder}>--:--</span>
									)}
								</div>
							</div>
						</CircularProgressbarWithChildren>
					</div>
				)}
				{!isVertical && isReady && (
					<div css={styles.nextRow}>
						{t('next')} {nextDurationMin}
						{t('minutes')} {t(nextLabel)}
					</div>
				)}
			</div>
		</div>
	)
})

export default Timer
