import { Wave } from '@foobar404/wave'
import React, { useEffect, useRef, useState } from 'react'
import { Constants } from '../lib/constants'
import { getCurrentSection, SectionType } from '../lib/time-table'
import * as styles from '../styles/BgmPlayer.styles'
import { componentBackground, componentStyle } from '../styles/common.style'
import { getCurrentRandomBgm } from '../lib/bgm'
import jsmediatags from 'jsmediatags'

const BgmPlayer: React.FC = () => {
    const [lastSectionType, setLastSectionType] = useState('')
    const [audioTitle, setAudioTitle] = useState('BGM TITLE')
    const [audioArtist, setAudioArtist] = useState('BGM ARTIST')

    const audioDivId = 'music'
    const audioCanvasId = 'audioCanvas'
    const chimeSingleDivId = 'chimeSingle'
    const chimeDoubleDivId = 'chimeDouble'

    const waveRef = useRef<Wave | null>(null)

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
            playChimeSingle()
        }
        // 作業時間から休憩時間に変わるタイミングでチャイムを再生
        if (
            lastSectionType === SectionType.Study &&
            currentSection.sectionType === SectionType.Break
        ) {
            playChimeDouble()
        }
        setLastSectionType(currentSection.sectionType)
    }

    const audioStart = () => {
        const audio = document.getElementById(audioDivId) as HTMLAudioElement
        audio.addEventListener('ended', () => {
            setAudioTitle('BGM TITLE')
            setAudioArtist('BGM ARTIST')
            audioNext()
        })
        audio.addEventListener('error', () => {
            console.error(`Error loading audio file: ${audio.src}`)
            audioNext()
        })
        audioNext()
    }

    type ID3Tag = {
        tags: {
            title: string | null
            artist: string | null
        }
    }

    const audioNext = async () => {
        const audio = document.getElementById(audioDivId) as HTMLAudioElement

        const bgm = await getCurrentRandomBgm()
        audio.src = bgm
        jsmediatags.read(audio.src, {
            onSuccess(tag: ID3Tag) {
                const title = tag.tags.title
                const artist = tag.tags.artist
                setAudioTitle(title !== null && title !== undefined ? title : 'BGM TITLE')
                setAudioArtist(artist !== null && artist !== undefined ? artist : 'BGM ARTIST')
            },
            onError(error: Error) {
                console.error(error)
            },
        })
        audio.volume = Constants.bgmVolume

        audio.play()
    }

    const stop = () => {
        const audio = document.getElementById(audioDivId) as HTMLAudioElement
        audio.pause()
        setAudioTitle('BGM TITLE')
        setAudioArtist('BGM ARTIST')
    }

    const playChimeSingle = () => {
        const chimeSingle = document.getElementById(chimeSingleDivId) as HTMLAudioElement
        chimeSingle.volume = Constants.chimeVolume
        chimeSingle.play()
    }

    const playChimeDouble = () => {
        const chimeDouble = document.getElementById(chimeDoubleDivId) as HTMLAudioElement
        chimeDouble.volume = Constants.chimeVolume
        chimeDouble.play()
    }

    useEffect(() => {
        if (!waveRef.current) {
            const audioElement = document.getElementById(audioDivId) as HTMLAudioElement
            const canvasElement = document.getElementById(audioCanvasId) as HTMLCanvasElement

            const wave = new Wave(audioElement, canvasElement)
            wave.addAnimation(
                new wave.animations.Wave({
                    fillColor: '#27272787',

                    lineColor: '#0000', // NOTE: alpha=0 が目的。lineWidth: 0が意味なさそうだったので
                    rounded: true,
                    bottom: true,
                    count: 30,
                })
            )
            audioStart()

            waveRef.current = wave
        }
    }, [])

    useEffect(() => {
        const intervalId = setInterval(() => updateState(), 1000)
        return () => {
            clearInterval(intervalId)
        }
    }, [updateState, audioStart, audioNext, stop])

    return (
        <div css={[styles.shape, componentBackground]}>
            <div css={[styles.bgmPlayer, componentStyle]}>
                <audio autoPlay id={audioDivId}></audio>

                <audio id={chimeSingleDivId} src={Constants.chimeSingleFilePath}></audio>
                <audio id={chimeDoubleDivId} src={Constants.chimeDoubleFilePath}></audio>
                <h4>♪ {audioTitle}</h4>
                <h4>by {audioArtist}</h4>

                <div css={styles.audioCanvasDiv}>
                    <canvas id={audioCanvasId} css={styles.audioCanvas}></canvas>
                </div>
            </div>
        </div>
    )
}

export default BgmPlayer
