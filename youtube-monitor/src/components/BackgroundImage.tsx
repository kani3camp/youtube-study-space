import React, { FC, useState } from 'react'
import { useInterval } from '../lib/common'
import { getCurrentSection } from '../lib/time_table'
import * as styles from '../styles/BackgroundImage.styles'

const BackgroundImage: FC = () => {
    const stateUpdateIntervalMilliSec = 1000
    const baseUrl = 'https://source.unsplash.com/featured/1920x1080'
    const args: string =
        '/?' +
        'work,cafe,study,nature,chill,coffee,tea,sea,lake,outdoor,land,spring,summer,fall,winter,hotel' +
        ',green,purple,pink,blue,dark,azure,yellow,orange,gray,brown,red,black,pastel' +
        ',blossom,flower,corridor,door,background,wood,resort,travel,vacation,beach,grass' +
        ',pencil,pen,eraser,stationary,classic,jazz,lo-fi,fruit,vegetable,sky,instrument,cup' +
        ',star,moon,night,cloud,rain,mountain,river,calm,sun,sunny,water,building,drink,keyboard' +
        ',morning,evening'
    const unsplashUrl = baseUrl + args

    const [srcUrl, setSrcUrl] = useState<string>(unsplashUrl)
    const [lastPartName, setLastPartName] = useState<string>('')

    useInterval(() => {
        const now = new Date()
        const currentSection = getCurrentSection()

        if (currentSection.partType !== lastPartName) {
            setSrcUrl(`${unsplashUrl},${now.getTime()}`)
            setLastPartName(currentSection.partType)
        }
    }, stateUpdateIntervalMilliSec)

    return (
        <div css={styles.backgroundImage}>
            <img src={srcUrl} alt='背景画像' />
        </div>
    )
}

export default BackgroundImage
