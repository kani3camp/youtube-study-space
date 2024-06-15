import { FC, useState } from 'react'
import { useInterval } from '../lib/common'
import { getCurrentSection } from '../lib/time-table'
import * as styles from '../styles/BackgroundImage.styles'

const BACKGROUND_UPDATE_INTERVAL_SEC = 1000
const BACKGROUND_IMAGE_URL = '/images/background/7290504_3601460.jpg'

const BackgroundImage: FC = () => {
    const [lastPartName, setLastPartName] = useState<string>('')

    useInterval(() => {
        const currentSection = getCurrentSection()

        if (currentSection.partType !== lastPartName) {
            setLastPartName(currentSection.partType)
        }
    }, BACKGROUND_UPDATE_INTERVAL_SEC)

    return (
        <div>
            <img
                src={BACKGROUND_IMAGE_URL}
                css={styles.backgroundImage}
                alt='background image'
                onError={({ currentTarget }) => {
                    currentTarget.onerror = null // prevents looping
                    currentTarget.src = BACKGROUND_IMAGE_URL
                }}
            />
            <div css={styles.blurLayer}></div>
        </div>
    )
}

export default BackgroundImage
