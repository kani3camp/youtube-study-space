/** @jsxImportSource @emotion/react */
import { useTranslation } from 'next-i18next'
import { memo, useCallback, useEffect, useMemo, useState } from 'react'
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

const UPDATE_INTERVAL_MS = 100

const Timer = memo(function Timer() {
	const { t } = useTranslation()
	const [now, setNow] = useState<Date | null>(null)

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
		<div css={[styles.shape, componentBackground]}>
			<div css={[styles.timer, componentStyle]}>
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
				{isReady && (
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
