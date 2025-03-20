import { css } from '@emotion/react'
import { sourceCodeProFontFamily } from '../lib/common'
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
    width: 90%;
    padding: 0.4rem;
    border-radius: 0.6rem;
    background-color: rgba(255, 255, 255, 0.3);
    backdrop-filter: blur(0.5rem);
`

export const menuTitle = css`
    margin: 0.1rem;
`

export const list = css`
    display: flex;
    justify-content: space-around;
    align-items: center;
`

export const listItem = css`
    flex: 1;
`

export const itemImage = css``

export const image = css`
    position: relative;
`

export const name = css`
    font-size: 0.8em;
    /* color: #414141; */
    margin: 0.1rem;
    line-height: 0.9rem;

    text-overflow: ellipsis;
    overflow: hidden;
    word-wrap: break-word;
    display: -webkit-box;
    -webkit-line-clamp: 2;
    -webkit-box-orient: vertical;
`

export const commandCode = css`
    padding: 0.15rem;
    background-color: #eee;
    border-radius: 0.2rem;
    font-size: 0.75em;
    font-weight: bold;
    text-overflow: ellipsis;
    overflow: hidden;
    white-space: nowrap;
`

export const notice = css`
    font-size: 0.36rem;
    position: absolute;
    width: 94%;
    bottom: 0.12rem;
    color: #4763d7;
    white-space: nowrap;
    text-overflow: ellipsis;
    overflow: hidden;
`
