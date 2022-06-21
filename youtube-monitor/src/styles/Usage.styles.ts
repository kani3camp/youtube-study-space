import { css } from '@emotion/react'

export const background = css`
    height: 390px;
    width: 400px;
    background-color: rgba(255, 241, 221, 1);
    padding: 0.5rem 1rem;
    box-sizing: border-box;
    position: absolute;
    top: 160px;
    right: 0;
    font-size: 1rem;
    text-align: center;
    color: #383838;
`

export const usage = css`
    padding: 0.4rem;
    border-radius: 1rem;
    background-color: rgba(199, 230, 233, 0.95);
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
