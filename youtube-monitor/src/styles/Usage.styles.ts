import { css } from '@emotion/react'
import { Constants } from '../lib/constants'
import { sourceCodeProFontFamily } from '../lib/common'

export const shape = css`
    height: ${Constants.usageHeight}px;
    width: ${Constants.sideBarWidth}px;
    position: absolute;
    top: ${Constants.clockHeight}px;
    right: 0;
`

export const usage = css`
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

export const seatId = css`
    margin: 0 auto;
    font-size: 0.7em;
    color: #414141;
`

export const description = css`
    margin: 0.2rem;
`

export const seat = css`
    background-color: rgba(243, 236, 236, 0.523);
    width: 6rem;
    height: 3.5rem;
    margin: 0.2rem auto;
    margin-bottom: 0.3rem;
    text-overflow: ellipsis;
    overflow: hidden;
    display: flex;
    flex-direction: column;
    justify-content: center;
    align-items: center;
    border-radius: 0.15rem;
`

export const workName = css`
    margin: 0;
    font-size: 0.8em;
    color: #28292d;
    max-width: 100%;
    text-overflow: ellipsis;
    overflow: hidden;
    white-space: nowrap;
`

export const userDisplayName = css`
    margin: 0;
    font-size: 1em;
    max-width: 100%;
    text-overflow: ellipsis;
    overflow: hidden;
    white-space: nowrap;
    color: #202020;
`

export const command = css`
    margin: 0.2rem;
    font-size: 0.85rem;
`

export const commandCode = css`
    font-weight: bold;
    display: inline-block;
    background-color: #eee;
    border-radius: 0.15rem;
    padding: 0 0.3rem;
    margin: 0.07rem;
    font-family: ${sourceCodeProFontFamily};
`
