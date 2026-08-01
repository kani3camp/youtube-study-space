import Image from 'next/image'
import type { FC } from 'react'
import { Constants } from '../lib/constants'
import * as styles from '../styles/BackgroundImage.styles'

const BACKGROUND_IMAGE_URL = '/images/background/20549579_6334500.jpg'

const BackgroundImage: FC = () => {
	return (
		<div>
			<Image
				src={BACKGROUND_IMAGE_URL}
				css={styles.backgroundImage}
				alt="background image"
				onError={({ currentTarget }) => {
					currentTarget.onerror = null // prevents looping
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
