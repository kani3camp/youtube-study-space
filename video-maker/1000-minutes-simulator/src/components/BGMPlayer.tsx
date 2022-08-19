import { FC, useEffect, useState } from 'react'
import { BsFillPersonFill } from 'react-icons/bs'
import { IoMdMusicalNotes } from 'react-icons/io'
import { MdQueueMusic } from 'react-icons/md'
import { getRandomBgm } from '../lib/common'
import { OffsetSec } from '../pages'
import * as styles from '../styles/BGMPlayer.styles'
import * as common from '../styles/common.styles'

type Props = {
    elapsedMinutes: number
}

const BGMPlayer: FC<Props> = (props) => {
    const [audioTitle, setAudioTitle] = useState('BGMタイトル')
    const [audioArtist, setAudioArtist] = useState('BGMアーティスト')

    const audioDivId = 'music'

    useEffect(() => {
        audioStart()
    }, [])

    const audioStart = () => {
        const audio = document.getElementById(audioDivId) as HTMLAudioElement
        audio.addEventListener('ended', () => {
            setAudioTitle('BGMタイトル')
            setAudioArtist('BGMアーティスト')
            audioNext()
        })
        audio.addEventListener('error', (event) => {
            console.error('failed loading audio.', event)
            audioNext()
        })

        // offsetのぶんだけ待ってから再生開始
        setTimeout(() => {
            audioNext()
        }, OffsetSec * 1000)
    }

    const audioNext = () => {
        const audio = document.getElementById(audioDivId) as HTMLAudioElement
        const bgm = getRandomBgm()
        audio.src = bgm.file
        setAudioTitle(bgm.title)
        setAudioArtist(bgm.artist)
        audio.volume = 0.3
        audio.addEventListener('loadeddata', () => {
            audio.play()
        })
    }

    return (
        <div css={styles.bgmPlayer}>
            <div css={styles.innerCell}>
                <div css={common.heading}>
                    <MdQueueMusic
                        size={common.IconSize}
                        css={styles.icon}
                    ></MdQueueMusic>
                    <span>BGM</span>
                </div>
                <div css={styles.item}>
                    <IoMdMusicalNotes></IoMdMusicalNotes>
                    <span>{audioTitle}</span>
                </div>
                <div css={styles.item}>
                    <BsFillPersonFill></BsFillPersonFill>
                    <span>{audioArtist}</span>
                </div>

                <audio autoPlay id={audioDivId}></audio>
            </div>
        </div>
    )
}

export default BGMPlayer
