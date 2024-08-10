import { css } from '@emotion/react'
import { Constants } from '../lib/constants'

export const shape = css`
    height: ${Constants.shoutHeight}px;
    width: ${Constants.sideBarWidth}px;
    position: absolute;
    top: ${Constants.clockHeight + Constants.usageHeight}px;
    right: 0;
`

export const shout = css`
    font-size: 1rem;
    text-align: center;
    color: ${Constants.primaryTextColor};
    box-sizing: border-box;
    height: 95%;
    width: 85%;
    padding: 0.3rem;
    border-radius: 0.6rem;
    background-color: rgba(255, 255, 255, 0.3);
    backdrop-filter: blur(0.5rem);
    display: flex;
    flex-direction: column;
`

export const description = css`
    margin: 0.2rem;
    line-height: 1.2rem;
`

export const shoutContent = css`
    background-color: #e1d7d74e;
    border-radius: 0.3rem;
    flex-grow: 1;
`

export const messageText = css`
    font-size: 0.8rem;
    text-align: left;
`

export const userName = css`
    font-size: 0.7rem;
    text-align: right;
    overflow: hidden;
`
