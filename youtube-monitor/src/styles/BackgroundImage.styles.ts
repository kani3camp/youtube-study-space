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
