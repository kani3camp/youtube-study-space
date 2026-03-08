import { css } from '@emotion/react'
import {
	sidebarBgmHeight,
	sidebarCardHorizontalInsetPx,
	sidebarCardVerticalInsetPx,
} from './../lib/constants'
import { Constants } from '../lib/constants'

export const shape = css`
    height: ${Constants.colorBarHeight}px;
    width: ${Constants.sideBarWidth}px;
    position: absolute;
    bottom: ${sidebarBgmHeight}px;
    right: 0;
`

export const colorBar = css`
    height: calc(100% - ${sidebarCardVerticalInsetPx}px);
    width: calc(100% - ${sidebarCardHorizontalInsetPx}px);
    background-color: rgba(255, 255, 255, 0.3);
    backdrop-filter: blur(0.5rem);
    display: flex;
    align-items: center;
    justify-content: center;
    font-size: 0.8rem;
    font-weight: bold;
    color: ${Constants.primaryTextColor};
    border-radius: 0.6rem;
    padding: 0.2rem;
    box-sizing: border-box;
`
