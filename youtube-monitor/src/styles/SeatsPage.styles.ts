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
