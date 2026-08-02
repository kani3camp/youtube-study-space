import { css } from '@emotion/react'
import {
	Constants,
	sidebarBgmHeight,
	sidebarCardHorizontalInsetPx,
	sidebarCardVerticalInsetPx,
} from '../lib/constants'

export const shape = css`
    height: ${sidebarBgmHeight}px;
    width: ${Constants.sideBarWidth}px;
    position: absolute;
    right: 0;
    bottom: 0;
`

export const bgmPlayer = css`
    height: calc(100% - ${sidebarCardVerticalInsetPx}px);
    width: calc(100% - ${sidebarCardHorizontalInsetPx}px);
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
        margin-inline-end: 0.5rem;
        margin-block-start: 0.2rem;
        margin-block-end: 0.2rem;
        font-size: 0.5rem;
        font-weight: normal;
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
