import React, {useEffect, useState} from "react";
import styles from "./BgmPlayer.module.sass";
import next from "next";
import { getCurrentSection } from "../lib/time_table";
import { Bgm, getCurrentRandomBgm } from "../lib/bgm";


const BgmPlayer: React.FC = () => {
    const [lastSectionId, setLastSectionId] = useState(0)
    const [audioTitle, setAudioTitle] = useState('BGMタイトル')
    const [audioArtist, setAudioArtist] = useState('BGMアーティスト')

    const updateState = () => {
        const audio = document.getElementById('music') as HTMLAudioElement
        const currentSection = getCurrentSection()

        if (currentSection !== null) {
            // sectionIdが0から変わるタイミングで新しいbgmを再生開始
            if (lastSectionId === 0 && currentSection.sectionId !== 0) {
                // partTypeに応じたbgmをランダムに選択
                const bgm = getCurrentRandomBgm(currentSection?.partName)
                if (bgm !== null) {
                    setAudioTitle(bgm.title)
                    setAudioArtist(bgm.artist)
                    loopPlay(bgm.file)
                    setLastSectionId(currentSection.sectionId)
                }
            }
            // sectionIdが0になるタイミングでbgmを停止
            if (lastSectionId !== 0 && currentSection.sectionId === 0) {
                setAudioTitle('BGMタイトル')
                setAudioArtist('BGMアーティスト')
                stop()
                setLastSectionId(currentSection.sectionId)
            }
        }
    }

    const loopPlay = (src: string) => {
        const audio: HTMLAudioElement = document.getElementById('music') as HTMLAudioElement
        audio.src = src
        audio.loop = true
        audio.volume = 0.6
        audio.play()
    }

    const stop = () => {
        console.log('stop()')
        const audio: HTMLAudioElement = document.getElementById('music') as HTMLAudioElement
        audio.pause()
    }

    useEffect(() => {
        console.log('useEffect')
        const intervalId = setInterval(() => updateState(), 1000)
        return () => {
            console.log('クリーンアップ')
            clearInterval(intervalId)
        }
    }, [updateState, loopPlay, stop])   // この第２引数がないといけない。。。


    return (
        <div id={styles.bgmPlayer}>
            <audio autoPlay id='music' src=""></audio>
            <h4>♪ {audioTitle}</h4>
            <h4>by {audioArtist}</h4>
        </div>
    )
}


export default BgmPlayer;
