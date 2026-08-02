import { css } from '@emotion/react'
import { fontFamily } from '../lib/common'
import { Constants } from '../lib/constants'

export const shape = css`
    height: ${Constants.messageBarHeight}px;
    width: calc(${Constants.screenWidth}px - ${Constants.sideBarWidth}px - ${Constants.tickerWidth}px);
    position: absolute;
    bottom: 0;
    left: 0;
`

export const message = css`
    height: 80%;
    width: 100%;
    padding: 0 5%;
    text-align: center;
    font-size: 1.4rem;
    display: flex;
    flex-direction: row;
    color: ${Constants.primaryTextColor};
`

export const pageInfo = css`
    width: 45%;
    height: 100%;
    display: flex;
    align-items: center;
    justify-content: center;
`

export const pageIndex = css`
    display: inline-block;
`

export const memberOnly = css`
    font-family: ${fontFamily};
    width: 2.5rem;
    margin-left: 1rem;
    padding: 0.1rem;
    display: inline-block;
    font-size: 0.6rem;
    color: white;
    background-color: #2ba640;
    border-radius: 0.3rem;
`

export const numStudyingPeople = css`
    width: 45%;
    height: 100%;
    display: inline-block;
    background-color: rgba(255, 255, 255, 0.472);
    border-radius: 0.6rem;
`
