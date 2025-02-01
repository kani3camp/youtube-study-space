import jsmediatags from 'jsmediatags'
import { useTranslation } from 'next-i18next'
import { FC, useEffect, useState } from 'react'
import { BsFillPersonFill } from 'react-icons/bs'
import { IoMdMusicalNotes } from 'react-icons/io'
import { MdQueueMusic } from 'react-icons/md'
import { getCurrentRandomBgm } from '../lib/bgm'
import { OffsetSec } from '../pages'
import * as styles from '../styles/BGMPlayer.styles'
import * as common from '../styles/common.styles'

type Props = {
    elapsedMinutes: number
}

const BGMPlayer: FC<Props> = (props) => {
    const { t } = useTranslation()

    const [audioTitle, setAudioTitle] = useState<string>(t('bgm.title'))
    const [audioArtist, setAudioArtist] = useState<string>(t('bgm.artist'))

    const audioDivId = 'music'

    useEffect(() => {
        audioStart()
    }, [])

    const audioStart = () => {
        const audio = document.getElementById(audioDivId) as HTMLAudioElement
        audio.addEventListener('ended', () => {
            setAudioTitle(t('bgm.title'))
            setAudioArtist(t('bgm.artist'))
            audioNext()
        })
        audio.addEventListener('error', (event) => {
            console.error('failed loading audio: ', event)
            audioNext()
        })

        // offsetのぶんだけ待ってから再生開始
        setTimeout(() => {
            audioNext()
        }, OffsetSec * 1000)
    }

    const audioNext = async () => {
        const audio = document.getElementById(audioDivId) as HTMLAudioElement
        const bgm = await getCurrentRandomBgm()

        audio.src = bgm
        jsmediatags.read(audio.src, {
            onSuccess(tag) {
                const title = tag.tags.title
                const artist = tag.tags.artist
                setAudioTitle(
                    title !== null && title !== undefined
                        ? title
                        : t('bgm.title')
                )
                setAudioArtist(
                    artist !== null && artist !== undefined
                        ? artist
                        : t('bgm.artist')
                )
            },
            onError(error) {
                console.error(error)
            },
        })
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
