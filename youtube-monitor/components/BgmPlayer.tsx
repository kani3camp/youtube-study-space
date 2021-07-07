import React, {useEffect, useState} from "react";
import styles from "./BgmPlayer.module.sass";
import next from "next";
import { getCurrentSection } from "../lib/time_table";
import { Bgm, getCurrentRandomBgm } from "../lib/bgm";

const BgmPlayer: React.FC = () => {
    const [lastSectionId, setLastSectionId] = useState(0)
    const [audioTitle, setAudioTitle] = useState('BGMタイトル')
    const [audioArtist, setAudioArtist] = useState('BGMアーティスト')
    let [initialized, setInitialized] = useState(false)

    const updateState = () => {
        const audio = document.getElementById('music') as HTMLAudioElement
        const currentSection = getCurrentSection()

        // TODO: sectionIdが0から変わるタイミングでチャイムを再生
        if (lastSectionId === 0 && currentSection.sectionId !== 0) {
            // partTypeに応じたbgmをランダムに選択
            const bgm = getCurrentRandomBgm(currentSection?.partName)
            if (bgm !== null) {
                setAudioTitle(bgm.title)
                setAudioArtist(bgm.artist)
                // audioStart()
                setLastSectionId(currentSection.sectionId)
            }
        }
        // TODO: sectionIdが0になるタイミングでチャイムを再生
        if (lastSectionId !== 0 && currentSection.sectionId === 0) {
            // stop()
            setLastSectionId(currentSection.sectionId)
        }
    }

    const audioStart = () => {
        const audio = document.getElementById('music') as HTMLAudioElement
        audio.addEventListener('ended', function() {
            setAudioTitle('BGMタイトル')
            setAudioArtist('BGMアーティスト')
            audioNext()
        })
        audioNext()
    }

    const audioNext = () => {
        const audio = document.getElementById('music') as HTMLAudioElement
        const currentSection = getCurrentSection()
        const bgm = getCurrentRandomBgm(currentSection.partName)
        audio.src = bgm.file
        setAudioTitle(bgm.title)
        setAudioArtist(bgm.artist)
        audio.volume = 0.6
        audio.play()
    }

    const stop = () => {
        const audio = document.getElementById('music') as HTMLAudioElement
        audio.pause()
        setAudioTitle('BGMタイトル')
        setAudioArtist('BGMアーティスト')
    }

    useEffect(() => {
        // console.log('useEffect')
        if (!initialized) {
            setInitialized(true)
            audioStart()
        }
        const intervalId = setInterval(() => updateState(), 1000)
        return () => {
            // console.log('クリーンアップ')
            clearInterval(intervalId)
        }
    }, [updateState, audioStart, audioNext, stop])   // この第２引数がないといけない。。。


    return (
        <div id={styles.bgmPlayer}>
            <audio autoPlay id='music' src=""></audio>
            <h4>♪ {audioTitle}</h4>
            <h4>by {audioArtist}</h4>
        </div>
    )
}


export default BgmPlayer;
