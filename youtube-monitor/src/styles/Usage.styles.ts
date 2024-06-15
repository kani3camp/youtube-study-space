import { css } from '@emotion/react'
import { Constants } from '../lib/constants'

export const shape = css`
    height: 390px;
    width: ${Constants.sideBarWidth}px;
    position: absolute;
    top: 160px;
    right: 0;
`

export const usage = css`
    font-size: 1rem;
    text-align: center;
    color: #383838;
    box-sizing: border-box;
    height: 95%;
    width: 85%;
    padding: 0.4rem;
    border-radius: 0.6rem;
    background-color: rgba(255, 255, 255, 0.3);
    backdrop-filter: blur(0.5rem);
    position: absolute;
    top: 0;
    right: 0;
    left: 0;
    bottom: 0;
    margin: auto;
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
    border: solid #3a3a3a 0.09rem;
    width: 7.1rem;
    height: 3.5rem;
    margin: 0.2rem auto;
    margin-bottom: 1rem;
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

export const commandString = css`
    font-weight: bold;
    display: inline-block;
    background-color: #fff;
    border-radius: 0.15rem;
    padding: 0 0.4rem;
    margin: 0.1rem;
`
