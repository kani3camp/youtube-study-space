import { css } from '@emotion/react'
import { Constants } from '../lib/constants'

export const backgroundImage = css`
    position: absolute;
    top: 0;
    left: 0;
    width: ${Constants.screenWidth};
    height: ${Constants.screenHeight};
    z-index: -1;
`
