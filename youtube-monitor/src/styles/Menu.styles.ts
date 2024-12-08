import { css } from '@emotion/react'
import { Constants } from '../lib/constants'

export const shape = css`
    height: ${Constants.menuHeight}px;
    width: ${Constants.sideBarWidth}px;
    position: absolute;
    top: ${Constants.clockHeight + Constants.usageHeight}px;
    right: 0;
`

export const menu = css`
    font-size: 1rem;
    text-align: center;
    color: ${Constants.primaryTextColor};
    box-sizing: border-box;
    height: 95%;
    width: 85%;
    padding: 0.4rem;
    border-radius: 0.6rem;
    background-color: rgba(255, 255, 255, 0.3);
    backdrop-filter: blur(0.5rem);
`

export const menuTitle = css`
    margin: 0.2rem;
`

export const menuBody = css`
    display: flex;
    justify-content: space-around;
    align-items: center;
`

export const itemBody = css`
    margin: 0 auto;
    font-size: 0.7em;
    color: #414141;
    height: 4rem;
`

export const notice = css`
    font-size: 0.35rem;
`
