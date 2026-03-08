import { css } from '@emotion/react'
import {
	Constants,
	sidebarCardHorizontalInsetPx,
	sidebarCardVerticalInsetPx,
} from '../lib/constants'

export const shape = css`
    height: ${Constants.clockHeight}px;
    width: ${Constants.sideBarWidth}px;
    position: absolute;
    top: 0;
    right: 0;
`

export const clockStyle = css`
    height: calc(100% - ${sidebarCardVerticalInsetPx}px);
    width: calc(100% - ${sidebarCardHorizontalInsetPx}px);
    border-radius: 0.6rem;
    background-color: rgba(255, 255, 255, 0.4);
    color: ${Constants.primaryTextColor};
    padding: 0.2rem;
    box-sizing: border-box;
`

export const dateStringStyle = css`
    font-size: 0.6rem;
    text-align: center;
`

export const timeStringStyle = css`
    font-size: 1.2rem;
    text-align: center;
    font-weight: bold;
    line-height: 1.6rem;
`
