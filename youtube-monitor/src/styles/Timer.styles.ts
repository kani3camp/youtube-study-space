import { css } from '@emotion/react'
import { Constants } from '../lib/constants'

export const shape = css`
    height: 300px;
    width: ${Constants.sideBarWidth}px;
    position: absolute;
    bottom: 0;
    right: 0;
`

export const timer = css`
    height: 90%;
    width: 83%;
    border-radius: 0.6rem;
    position: absolute;
    bottom: 0;
    right: 0;
    top: 0;
    left: 0;
    margin: auto;
    font-size: 0.9rem;
    text-align: center;
    color: #2d2b28;
    background-color: rgba(255, 255, 255, 0.3);
`

export const timerTitle = css`
    font-size: 1.5rem;
    font-weight: bold;
`

export const sectionColor = css`
    display: inline-block;
    padding: 0 0.4rem;
    margin-top: 0.4rem;
    border-radius: 0.5rem;
`

export const spacer = css`
    margin: 0.1rem;
`

export const remaining = css`
    font-family: 'M PLUS Rounded 1c';
    font-size: 2.1rem;
    font-weight: bold;
`
export const studyMode = css`
    background-color: #f869a5e7;
`

export const breakMode = css`
    background-color: #5af87fe7;
`

export const nextDescription = css`
    color: #2d2b28;
`
