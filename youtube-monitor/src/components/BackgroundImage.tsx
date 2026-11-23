import Image from 'next/image'
import { type FC, useRef } from 'react'
import { Constants } from '../lib/constants'
import * as styles from '../styles/BackgroundImage.styles'

const BACKGROUND_IMAGE_URL = '/images/background/4167307_214.jpg'

const BackgroundImage: FC = () => {
	// next/image の onError は onerror = null では止まらないため、再試行済みかを自前で持つ。
	const fallbackTriedRef = useRef(false)
	return (
		<div>
			<Image
				src={BACKGROUND_IMAGE_URL}
				css={styles.backgroundImage}
				alt="background image"
				onError={({ currentTarget }) => {
					if (fallbackTriedRef.current) {
						return
					}
					fallbackTriedRef.current = true
					currentTarget.src = BACKGROUND_IMAGE_URL
				}}
				width={Constants.screenWidth}
				height={Constants.screenHeight}
				priority={true}
			/>
			<div css={styles.blurLayer} />
		</div>
	)
}

export default BackgroundImage
