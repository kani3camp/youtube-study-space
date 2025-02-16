import { css } from '@emotion/react'
import { fontFamily } from '../lib/common'
import { Constants } from '../lib/constants'

export const shape = css`
    height: ${Constants.timerHeight}px;
    width: ${Constants.sideBarWidth}px;
    position: absolute;
    bottom: 0;
    right: 0;
`

export const timer = css`
    height: 90%;
    width: 90%;
    border-radius: 0.6rem;
    font-size: 0.9rem;
    text-align: center;
    color: ${Constants.primaryTextColor};
    background-color: rgba(255, 255, 255, 0.3);
`

export const timerTitle = css`
    font-size: 1.2rem;
    font-weight: bold;
`

export const sectionColor = css`
    display: inline-block;
    padding: 0 0.4rem;
    margin-top: 0.4rem;
    border-radius: 0.5rem;
`

export const spacer = css`
    margin: 0.2rem;
`

export const remaining = css`
    font-family: ${fontFamily};
    font-size: 1.8rem;
    line-height: 2.4rem;
    font-weight: bold;
`
export const studyMode = css`
    background: rgb(253, 225, 254);
    background: linear-gradient(
        60deg,
        rgba(255, 130, 255, 0.6) 0%,
        rgba(253, 225, 254, 0.6) 40%,
        rgba(253, 225, 254, 0.6) 60%,
        rgba(255, 130, 255, 0.6) 100%
    );
`

export const breakMode = css`
    background: rgb(227, 254, 225);
    background: linear-gradient(
        60deg,
        rgba(130, 255, 165, 0.6) 0%,
        rgba(227, 255, 225, 0.6) 40%,
        rgba(227, 255, 225, 0.6) 60%,
        rgba(130, 255, 165, 0.6) 100%
    );
`
