import React, { FC, useState } from 'react'
import { useInterval } from '../lib/common'
import {
    getCurrentSection,
    getNextSection,
    remainingTime,
    SectionType,
} from '../lib/time_table'
import * as styles from '../styles/Timer.styles'

const TimeUpdateIntervalMilliSec = (1 / 30) * 1000 // 30fps

const Timer: FC = () => {
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
                setCurrentPartName(currentSection.partType)
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
                    currentSection.sectionType === SectionType.Study
                        ? '休憩'
                        : '作業'
                )
                setSectionType(currentSection.sectionType)
                setSectionMessage(
                    currentSection.sectionType === SectionType.Study
                        ? '✏️ 作業 ✏️'
                        : '☕️ 休憩 ☕️'
                )
            }
        }
    }, TimeUpdateIntervalMilliSec)

    return (
        <div css={styles.timer}>
            <div css={styles.timerTitle}>
                <div
                    css={[
                        styles.sectionColor,
                        sectionType === SectionType.Study
                            ? styles.studyMode
                            : styles.breakMode,
                    ]}
                >
                    {sectionMessage}
                </div>
            </div>
            <div css={styles.remaining}>
                {remainingMin}：
                {String(Math.floor(Number(remainingSec) % 60)).padStart(2, '0')}
            </div>
            <span>{`${currentPartName}` + ' '}</span>
            <span>
                {currentSectionId !== 0 ? `セクション${currentSectionId}` : ''}
            </span>
            <div css={styles.spacer} />
            <div css={styles.nextDescription}>
                <span>次は </span>
                <span>{nextSectionDuration}</span>
                <span>分 </span>
                <span>{nextSection}</span>
            </div>
        </div>
    )
}

export default Timer
