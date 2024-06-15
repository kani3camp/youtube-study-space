import { useTranslation } from 'next-i18next'
import { FC, useState } from 'react'
import { useInterval } from '../lib/common'
import { getCurrentSection, getNextSection, remainingTime, SectionType } from '../lib/time-table'
import * as styles from '../styles/Timer.styles'
import { componentBackground } from '../styles/common.style'

const TIME_UPDATE_INTERVAL_MILLI_SEC = (1 / 30) * 1000 // 30fps

const Timer: FC = () => {
    const { t } = useTranslation()

    const [sectionType, setSectionType] = useState<string>(SectionType.Break)
    const [sectionMessage, setSectionMessage] = useState<string>('')
    const [remainingMin, setRemainingMin] = useState<number>(0)
    const [remainingSec, setRemainingSec] = useState<number>(0)
    const [currentPartName, setCurrentPartName] = useState<string>('')
    const [currentSectionId, setCurrentSectionId] = useState<number>(0)
    const [nextSectionDuration, setNextSectionDuration] = useState<number>(0)
    const [nextSection, setNextSection] = useState<string>('')

    useInterval(() => {
        // フレームごとの更新

        const now: Date = new Date()
        const currentSection = getCurrentSection()
        if (currentSection !== null) {
            let remaining_min: number = remainingTime(
                now.getHours(),
                now.getMinutes(),
                currentSection.ends.h,
                currentSection.ends.m
            )
            const remaining_sec: number = (60 - now.getSeconds()) % 60
            if (remaining_sec !== 0) remaining_min -= 1

            const nextSection = getNextSection()
            if (nextSection !== null) {
                setRemainingMin(remaining_min)
                setRemainingSec(remaining_sec)
                setCurrentPartName(t(currentSection.partType))
                setCurrentSectionId(currentSection.sectionId)
                setNextSectionDuration(
                    remainingTime(
                        nextSection.starts.h,
                        nextSection.starts.m,
                        nextSection.ends.h,
                        nextSection.ends.m
                    )
                )
                setNextSection(
                    currentSection.sectionType === SectionType.Study ? t('break') : t('study')
                )
                setSectionType(currentSection.sectionType)
                setSectionMessage(
                    currentSection.sectionType === SectionType.Study
                        ? `✏️ ${t('study')} ✏️`
                        : `☕️ ${t('break')} ☕️`
                )
            }
        }
    }, TIME_UPDATE_INTERVAL_MILLI_SEC)

    return (
        <div css={[styles.shape, componentBackground]}>
            <div css={styles.timer}>
                <div css={styles.timerTitle}>
                    <div
                        css={[
                            styles.sectionColor,
                            sectionType === SectionType.Study ? styles.studyMode : styles.breakMode,
                        ]}
                    >
                        {sectionMessage}
                    </div>
                </div>
                <div css={styles.remaining}>
                    {remainingMin}：{String(Math.floor(Number(remainingSec) % 60)).padStart(2, '0')}
                </div>
                <span>{`${currentPartName}` + ' '}</span>
                <span>
                    {currentSectionId !== 0 ? t('section', { value: currentSectionId }) : ''}
                </span>
                <div css={styles.spacer} />
                <div css={styles.nextDescription}>
                    <span>{`${t('next')} `}</span>
                    <span>{nextSectionDuration}</span>
                    <span>{`${t('minutes')} `}</span>
                    <span>{nextSection}</span>
                </div>
            </div>
        </div>
    )
}

export default Timer
