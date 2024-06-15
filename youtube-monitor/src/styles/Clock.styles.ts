import { css } from '@emotion/react'
import { Constants } from '../lib/constants'

export const shape = css`
    height: 160px;
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
    color: #2d2b28;
    position: absolute;
    top: 0;
    bottom: 0;
    left: 0;
    right: 0;
    margin: auto;
`

export const dateStringStyle = css`
    font-size: 1.2rem;
    text-align: center;
`

export const timeStringStyle = css`
    font-size: 2rem;
    text-align: center;
    font-weight: bold;
    line-height: 2rem;
`
