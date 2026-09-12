import { css } from '@emotion/react'
import { Constants } from '../lib/constants'

export const roomLayout = css`
    position: relative;
    top: 0;
    left: 0;
    width: 100%;
    height: ${Constants.screenHeight - Constants.messageBarHeight}px;
    box-sizing: border-box;
    margin: auto;
    background-size: contain;
`

export const partition = css`
	position: absolute;
	background-color: #2d2b41;
`

export const floorImageFallback = css`
	position: absolute;
	inset: 0;
	background:
		radial-gradient(
			circle at 18% 15%,
			rgba(255, 255, 255, 0.7),
			transparent 42%
		),
		#d9d0c2;
`
