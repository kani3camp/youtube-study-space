import { FC, useState } from 'react'
import { useInterval } from '../lib/common'
import { getCurrentSection } from '../lib/time-table'
import * as styles from '../styles/BackgroundImage.styles'

const BACKGROUND_UPDATE_INTERVAL_SEC = 1000
const UNSPLASH_BASE_URL = 'https://source.unsplash.com/featured/1920x1080'
const UNSPLASH_QUERY_PARAM: string =
    '/?' +
    'work,cafe,study,nature,chill,coffee,tea,sea,lake,outdoor,land,spring,summer,fall,winter,hotel' +
    ',green,purple,pink,blue,dark,azure,yellow,orange,gray,brown,red,black,pastel' +
    ',blossom,flower,corridor,door,background,wood,resort,travel,vacation,beach,grass' +
    ',pencil,pen,eraser,stationary,classic,jazz,lo-fi,fruit,vegetable,sky,instrument,cup' +
    ',star,moon,night,cloud,rain,mountain,river,calm,sun,sunny,water,building,drink,keyboard' +
    ',morning,evening'
const UNSPLASH_URL = UNSPLASH_BASE_URL + UNSPLASH_QUERY_PARAM

const BackgroundImage: FC = () => {
    const [srcUrl, setSrcUrl] = useState<string>(UNSPLASH_URL)
    const [lastPartName, setLastPartName] = useState<string>('')

    useInterval(() => {
        const now = new Date()
        const currentSection = getCurrentSection()

        if (currentSection.partType !== lastPartName) {
            setSrcUrl(`${UNSPLASH_URL},${now.getTime()}`)
            setLastPartName(currentSection.partType)
        }
    }, BACKGROUND_UPDATE_INTERVAL_SEC)

    return (
        <div css={styles.backgroundImage}>
            <img
                src={srcUrl}
                alt='background image'
                onError={({ currentTarget }) => {
                    currentTarget.onerror = null // prevents looping
                    currentTarget.src = `${UNSPLASH_URL},${new Date().getTime()}`
                }}
            />
        </div>
    )
}

export default BackgroundImage
