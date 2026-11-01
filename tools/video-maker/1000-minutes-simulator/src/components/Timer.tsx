import { css } from '@emotion/react'
import { useTranslation } from 'next-i18next/pages'
import { type FC, useEffect, useState } from 'react'
import {
	CircularProgressbarWithChildren,
	buildStyles,
} from 'react-circular-progressbar'
import { AiFillFire } from 'react-icons/ai'
import { MdFreeBreakfast } from 'react-icons/md'
import { RiTimerFill } from 'react-icons/ri'
import {
	calcNumberOfPomodoroRounds,
	calcPomodoroRemaining,
} from '../lib/common'

import * as styles from '../styles/Timer.styles'
import * as common from '../styles/common.styles'
import 'react-circular-progressbar/dist/styles.css'

const AUDIO_FILES = {
	CHIME1: '/audio/chime/chime1.mp3',
	CHIME2: '/audio/chime/chime2.mp3',
} as const

const CHIME_ELEMENT_IDS = {
	CHIME1: 'chime1',
	CHIME2: 'chime2',
} as const

type Props = {
	elapsedSeconds: number
}

const Timer: FC<Props> = (props) => {
	const { t } = useTranslation()


	const [isStudyingState, setIsStudyingState] = useState<boolean>(false)

	const [remainingSeconds, percentage, isStudying] = calcPomodoroRemaining(
		props.elapsedSeconds,
	)
	if (isStudying !== isStudyingState) {
		setIsStudyingState(isStudying as boolean)
	}

	const numberOfPomodoroRounds = calcNumberOfPomodoroRounds(
		props.elapsedSeconds,
	)

	const stateHTML = isStudying ? (
		<>
			<AiFillFire size={styles.stateIconSize} css={styles.studyIcon} />
			<span
				css={css`
					vertical-align: middle;
				`}
			>
				{t('timer.focus')}
			</span>
		</>
	) : (
		<>
			<MdFreeBreakfast size={styles.stateIconSize} css={styles.breakIcon} />
			<span
				css={css`
					vertical-align: middle;
				`}
			>
				{t('timer.break')}
			</span>
		</>
	)

	useEffect(() => {
		const chimeElementId = isStudyingState
			? CHIME_ELEMENT_IDS.CHIME1
			: CHIME_ELEMENT_IDS.CHIME2
		const chime = document.getElementById(chimeElementId) as HTMLAudioElement
		chime.volume = 0.8
		void chime.play()
	}, [isStudyingState])

	useEffect(() => {
		const checkFile = async () => {
			try {
				const response1 = await fetch(AUDIO_FILES.CHIME1)
				const response2 = await fetch(AUDIO_FILES.CHIME2)

				if (!response1.ok) {
					alert(`${AUDIO_FILES.CHIME1} not found`)
				}
				if (!response2.ok) {
					alert(`${AUDIO_FILES.CHIME2} not found`)
				}
			} catch (error: unknown) {
				alert('Failed to load audio files')
				console.error('Failed to load audio files:', error)
			}
		}

		checkFile()
	}, [])


	return (
		<div css={styles.timer}>
			<div css={styles.innerCell}>
				<div css={common.heading}>
					<RiTimerFill size={common.IconSize} css={styles.icon} />
					<span>{t('timer.title')}</span>
				</div>
				<div css={styles.progressBarContainer}>
					<CircularProgressbarWithChildren
						value={Number(percentage)}
						styles={buildStyles({
							strokeLinecap: 'butt',
							pathTransitionDuration: 0,
							pathColor: isStudying ? 'orangered' : 'lime',
						})}
					>
						<div css={styles.numberOfRoundsString}>
							{t('timer.round', {
								value: numberOfPomodoroRounds,
							})}
						</div>
						<div css={styles.isStudying}>{stateHTML}</div>
						<div css={styles.remaining}>
							{String(Math.floor(Number(remainingSeconds) / 60)).padStart(
								2,
								'0',
							)}
							:
							{String(Math.floor(Number(remainingSeconds) % 60)).padStart(
								2,
								'0',
							)}
						</div>
					</CircularProgressbarWithChildren>
				</div>
				<div>
					{t('timer.next')}
					{isStudying ? (
						<span>{` ${t('timer.next_rest', { value: 5 })}`}</span>
					) : (
						<span>{` ${t('timer.next_work', { value: 5 })}`}</span>
					)}
				</div>
			</div>

			<audio id={CHIME_ELEMENT_IDS.CHIME1} src={AUDIO_FILES.CHIME1} />
			<audio id={CHIME_ELEMENT_IDS.CHIME2} src={AUDIO_FILES.CHIME2} />
		</div>
	)
}

export default Timer
