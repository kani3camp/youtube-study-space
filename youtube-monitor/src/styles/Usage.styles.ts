import { css } from '@emotion/react'
import { Constants } from '../lib/constants'

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
    display: flex;
    flex-direction: column;
    justify-content: center;
    gap: 0.35rem;
`

export const description = css`
    margin: 0.2rem;
    font-weight: bold;
`

export const command = css`
    margin: 0.2rem;
    font-size: 0.95rem;
`

export const commandCode = css`
    font-weight: bold;
    display: inline-block;
    background-color: rgba(0, 0, 0, 0.05);
    border-radius: 0.15rem;
    padding: 0 0.3rem;
    margin: 0.07rem;
    font-family: monospace;
`

export const commandDesc = css`
    font-size: 0.9rem;
`
