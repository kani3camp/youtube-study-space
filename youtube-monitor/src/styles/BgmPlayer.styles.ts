import { css } from '@emotion/react'
import { Constants } from '../lib/constants'

export const shape = css`
    height: calc(1080px - 200px - 350px - 300px);
    width: ${Constants.sideBarWidth}px;
    position: absolute;
    right: 0;
    bottom: 300px;
`

export const bgmPlayer = css`
    height: 90%;
    width: 85%;
    background-color: rgba(53, 49, 49, 0.3);
    position: absolute;
    text-align: center;
    color: white;
    word-break: break-all;
    z-index: 20;
    border-radius: 0.6rem;

    position: absolute;
    top: 0;
    bottom: 0;
    left: 0;
    right: 0;
    margin: auto;
    overflow: hidden;

    & h4 {
        text-align: right;
        margin-inline-end: 1rem;
        font-size: 0.9rem;
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
