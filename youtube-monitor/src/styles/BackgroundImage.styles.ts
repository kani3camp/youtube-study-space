import { css } from '@emotion/react'
import { Constants } from '../lib/constants'

export const blurLayer = css`
    backdrop-filter: blur(0.2rem);
    height: ${Constants.screenHeight}px;
    width: ${Constants.screenWidth}px;
`

export const backgroundImage = css`
	position: absolute;
	top: 0;
	left: 0;
	z-index: -1;
`

export const verticalBackground = css`
	position: absolute;
	inset: 0;
	z-index: -1;
	height: 100%;
	width: 100%;
`

export const verticalBackgroundImage = css`
	filter: saturate(0.72) brightness(0.97);
`

export const verticalBlurLayer = css`
	position: absolute;
	inset: 0;
`
