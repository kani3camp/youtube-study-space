import Image from 'next/image'
import type { CSSProperties, FC } from 'react'
import { Constants } from '../lib/constants'
import * as styles from '../styles/BackgroundImage.styles'

const BACKGROUND_IMAGE_URL = '/images/background/4167307_214.jpg'

type BackgroundImageProps = {
	width?: number
	height?: number
}

const BackgroundImage: FC<BackgroundImageProps> = ({
	width = Constants.screenWidth,
	height = Constants.screenHeight,
}) => {
	const isDefaultSize =
		width === Constants.screenWidth && height === Constants.screenHeight

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
				style={
					isDefaultSize
						? undefined
						: ({
								width,
								height,
								objectFit: 'cover',
							} satisfies CSSProperties)
				}
				priority={true}
			/>
			<div css={styles.blurLayer} style={{ width, height }} />
		</div>
	)
}

export default BackgroundImage
