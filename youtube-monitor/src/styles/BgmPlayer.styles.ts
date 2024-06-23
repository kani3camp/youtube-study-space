import { css } from '@emotion/react'
import { Constants } from '../lib/constants'

export const shape = css`
    height: ${Constants.screenHeight -
    Constants.clockHeight -
    Constants.usageHeight -
    Constants.timerHeight}px;
    width: ${Constants.sideBarWidth}px;
    position: absolute;
    right: 0;
    bottom: ${Constants.timerHeight}px;
`

export const bgmPlayer = css`
    height: 90%;
    width: 85%;
    background-color: rgba(53, 49, 49, 0.3);
    position: absolute;
    text-align: center;
    color: ${Constants.secondaryTextColor};
    word-break: break-all;
    z-index: 20;
    border-radius: 0.6rem;

    overflow: hidden;

    & h4 {
        text-align: right;
        margin-inline-end: 1rem;
        font-size: 0.6rem;
    }
`

export const audioCanvasDiv = css`
    z-index: 10;
    clip-path: inset(0, 0, 0, 0);
`

export const audioCanvas = css`
    height: 30%;
    width: 100%;
    background-color: #77777700;
    position: absolute;
    right: 0;
    bottom: 0;
    z-index: 15;
`
