/** @jsxImportSource @emotion/react */
import { useTranslation } from 'next-i18next'
import { type FC, memo, useCallback, useEffect, useMemo, useState } from 'react'
import {
	buildStyles,
	CircularProgressbarWithChildren,
} from 'react-circular-progressbar'
import { AiFillFire } from 'react-icons/ai'
import { MdFreeBreakfast } from 'react-icons/md'
import { useInterval } from '../lib/common'
import { Constants } from '../lib/constants'
import {
	getCurrentSection,
	getNextSection,
	getSectionDateRange,
	SectionType,
} from '../lib/time-table'
import { componentBackground, componentStyle } from '../styles/common.style'
import * as styles from '../styles/Timer.styles'
import 'react-circular-progressbar/dist/styles.css'

const UPDATE_INTERVAL_MS = 100

const FALLBACK_REMAINING = {
	remainingSec: 0,
	percentage: 0,
	isStudy: true,
	nextLabel: '',
	nextDurationMin: 0,
}

function computeRemaining(now: Date): {
	remainingSec: number
	percentage: number
	isStudy: boolean
	nextLabel: string
	nextDurationMin: number
} {
	const section = getCurrentSection()
	const { startsAt, endsAt } = getSectionDateRange(section, now)
	const sectionDurationSec = Math.max(
		1,
		Math.floor((endsAt.getTime() - startsAt.getTime()) / 1000),
	)
	const remainingSec = Math.max(
		0,
		Math.floor((endsAt.getTime() - now.getTime()) / 1000),
	)
	const percentage = (remainingSec / sectionDurationSec) * 100
	const isStudy = section.sectionType === SectionType.Study
	const next = getNextSection()
	let nextLabel = ''
	let nextDurationMin = 0
	if (next) {
		const nextRange = getSectionDateRange(next, endsAt)
		nextDurationMin = Math.floor(
			(nextRange.endsAt.getTime() - nextRange.startsAt.getTime()) / 60000,
		)
		nextLabel = next.sectionType === SectionType.Study ? 'study' : 'break'
	}
	return {
		remainingSec,
		percentage,
		isStudy,
		nextLabel,
		nextDurationMin,
	}
}

const Timer: FC = memo(function Timer() {
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

	const mm = String(Math.floor(remainingSec / 60))
	const ss = String(remainingSec % 60).padStart(2, '0')

	return (
		<div css={[styles.shape, componentBackground]} suppressHydrationWarning>
			<div css={[styles.timer, componentStyle]}>
				<div css={styles.progressBarContainer}>
					<CircularProgressbarWithChildren
						value={percentage}
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
								{isStudy ? (
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
								)}
							</div>
							<div css={styles.remaining}>
								<span css={styles.remainingMinutes}>{mm}</span>
								<span css={styles.remainingDivider}>:</span>
								<span css={styles.remainingSeconds}>{ss}</span>
							</div>
						</div>
					</CircularProgressbarWithChildren>
				</div>
				{nextLabel && (
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
