import { css } from '@emotion/react'
import { Constants } from '../lib/constants'

export const shape = css`
    height: ${Constants.clockHeight}px;
    width: ${Constants.sideBarWidth}px;
    position: absolute;
    top: 0;
    right: 0;
`

export const clockStyle = css`
    height: 85%;
    width: 85%;
    border-radius: 0.6rem;
    background-color: rgba(255, 255, 255, 0.4);
    color: ${Constants.primaryTextColor};
`

export const dateStringStyle = css`
    font-size: 1rem;
    text-align: center;
`

export const timeStringStyle = css`
    font-size: 2rem;
    text-align: center;
    font-weight: bold;
    line-height: 2rem;
`
