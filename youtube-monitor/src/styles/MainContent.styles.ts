import { css } from '@emotion/react'
import { Constants } from '../lib/constants'

export const mainContent = css`
    height: ${Constants.screenHeight}px;
    width: calc(${Constants.screenWidth}px - ${Constants.sideBarWidth}px);
    position: absolute;
    top: 0;
    left: 0;
`
