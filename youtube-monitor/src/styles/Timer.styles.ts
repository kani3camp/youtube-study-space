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
    display: flex;
    flex-direction: column;
    align-items: center;
    justify-content: center;
    gap: 0.2rem;
`

export const progressBarContainer = css`
    width: 152px;
    height: 152px;
    margin: 0 auto;
`

export const progressInner = css`
    height: 100%;
    display: flex;
    flex-direction: column;
    align-items: center;
    justify-content: center;
    gap: 0.2rem;
`

export const stateRow = css`
    display: flex;
    align-items: center;
    justify-content: center;
    gap: 0.3rem;
    font-size: 0.85rem;
    line-height: 1.1;
`

export const stateLabel = css`
    vertical-align: middle;
    font-weight: bold;
`

export const studyIcon = css`
    color: ${Constants.timerProgressStudyColor};
`

export const breakIcon = css`
    color: ${Constants.timerProgressBreakColor};
`

export const stateLabelStudy = css`
    color: ${Constants.timerProgressStudyColor};
`

export const stateLabelBreak = css`
    color: ${Constants.timerProgressBreakColor};
`

export const remaining = css`
    font-family: ${fontFamily};
    font-size: 1.55rem;
    line-height: 1;
    font-weight: bold;
    letter-spacing: 0.03em;
`

export const nextRow = css`
    font-size: 0.82rem;
    text-align: center;
    line-height: 1.2;
`
