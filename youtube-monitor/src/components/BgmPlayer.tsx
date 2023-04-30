import Wave from '@foobar404/wave'
import { FC, useEffect, useState } from 'react'
import { getCurrentRandomBgm } from '../lib/bgm'
import { Constants } from '../lib/constants'
import { getCurrentSection, SectionType } from '../lib/time_table'
import * as styles from '../styles/BgmPlayer.styles'

const BgmPlayer: FC = () => {
    const BGM_VOLUME = Constants.bgmVolume
    const CHIME_VOLUME = Constants.chimeVolume

    const [lastSectionType, setLastSectionType] = useState('')
    const [audioTitle, setAudioTitle] = useState('BGMタイトル')
    const [audioArtist, setAudioArtist] = useState('BGMアーティスト')
    const [initialized, setInitialized] = useState(false)

    const audioDivId = 'music'
    const audioCanvasId = 'audioCanvas'
    const chimeSingleDivId = 'chimeSingle'
    const chimeDoubleDivId = 'chimeDouble'

    useEffect(() => {
        checkChimeCanPlay()
    }, [])

    const checkChimeCanPlay = async () => {
        console.log('checking chime audio files.')
        const chimeDivIdList = [chimeSingleDivId, chimeDoubleDivId]
        chimeDivIdList.forEach((divId) => {
            const chime = document.getElementById(divId) as HTMLAudioElement
            chime.addEventListener('error', () => {
                alert(`error loading: ${chime.src}`)
            })
            if (!chime.src) {
                alert(`invalid chime src: ${chime.src}`)
            }
            chime.load()
        })
    }

    const updateState = () => {
        const currentSection = getCurrentSection()

        // 休憩時間から作業時間に変わるタイミングでチャイムを再生
        if (
            lastSectionType === SectionType.Break &&
            currentSection.sectionType === SectionType.Study
        ) {
            chime1Play()
        }
        // 作業時間から休憩時間に変わるタイミングでチャイムを再生
        if (
            lastSectionType === SectionType.Study &&
            currentSection.sectionType === SectionType.Break
        ) {
            chime2Play()
        }
        setLastSectionType(currentSection.sectionType)
    }

    const audioStart = () => {
        const audio = document.getElementById(audioDivId) as HTMLAudioElement
        audio.addEventListener('ended', () => {
            console.log('ended.')
            setAudioTitle('BGMタイトル')
            setAudioArtist('BGMアーティスト')
            audioNext()
        })
        audio.addEventListener('error', () => {
            console.log('error loading audio file.')
            audioNext()
        })
        audioNext()
    }

    const audioNext = () => {
        const audio = document.getElementById(audioDivId) as HTMLAudioElement
        const currentSection = getCurrentSection()
        const bgm = getCurrentRandomBgm(currentSection.partType)
        audio.src = bgm.file
        setAudioTitle(bgm.title)
        setAudioArtist(bgm.artist)
        audio.volume = BGM_VOLUME
        audio.play()
    }

    const stop = () => {
        const audio = document.getElementById(audioDivId) as HTMLAudioElement
        audio.pause()
        setAudioTitle('BGMタイトル')
        setAudioArtist('BGMアーティスト')
    }

    const chime1Play = () => {
        const chimeSingle = document.getElementById(chimeSingleDivId) as HTMLAudioElement
        chimeSingle.volume = CHIME_VOLUME
        chimeSingle.play()
    }

    const chime2Play = () => {
        const chimeDouble = document.getElementById(chimeDoubleDivId) as HTMLAudioElement
        chimeDouble.volume = CHIME_VOLUME
        chimeDouble.play()
    }

    useEffect(() => {
        // console.log('useEffect')
        if (!initialized) {
            setInitialized(true)

            const wave = new Wave()
            const waveOptions = {
                type: 'wave',
                colors: ['#000', '#111'],
                stroke: 0,
            }
            wave.fromElement(audioDivId, audioCanvasId, waveOptions)

            audioStart()
        }
        const intervalId = setInterval(() => updateState(), 1000)
        return () => {
            // console.log('クリーンアップ')
            clearInterval(intervalId)
        }
    }, [updateState, audioStart, audioNext, stop]) // この第２引数がないといけない。。。

    return (
        <>
            <div css={styles.bgmPlayer}>
                <audio autoPlay id={audioDivId}></audio>

                <audio id={chimeSingleDivId} src={Constants.chimeSingleFilePath}></audio>
                <audio id={chimeDoubleDivId} src={Constants.chimeDoubleFilePath}></audio>
                <h4>♪ {audioTitle}</h4>
                <h4>by {audioArtist}</h4>
            </div>
            <div css={styles.audioCanvasDiv}>
                <canvas id={audioCanvasId} css={styles.audioCanvas}></canvas>
            </div>
        </>
    )
}

export default BgmPlayer
