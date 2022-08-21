import { css } from '@emotion/react'
import {
    TimerHeight,
    TimerWidth,
    basicCell,
    basicInnerCell,
    OuterMargin,
    InnerMargin,
    iconBase,
} from './common.styles'

const stateFontSize = 40
export const stateIconSize = stateFontSize

export const timer = css`
    ${basicCell};
    /* height: ${TimerHeight}px;
    width: ${TimerWidth}px; */
    text-align: center;
    grid-column-start: 1;
    grid-column-end: 2;
    grid-row-start: 1;
    grid-row-end: 2;
`

export const innerCell = css`
    ${basicInnerCell};
    height: calc(100% - ${OuterMargin + InnerMargin}px);
    width: calc(100% - ${InnerMargin + OuterMargin}px);
    margin: ${OuterMargin}px ${InnerMargin}px ${InnerMargin}px ${OuterMargin}px;
`

export const progressBarContainer = css`
    width: 55%;
    margin: 1rem auto;
`

export const numberOfRoundsString = css`
    font-size: 1rem;
    color: #c5afd3;
`

export const isStudying = css`
    font-size: ${stateFontSize}px;
`

export const remaining = css`
    font-size: 1.2rem;
`

export const icon = css`
    ${iconBase};
    color: magenta;
`

const stateIcon = css`
    ${icon};
    margin-right: 0.2rem;
`

export const studyIcon = css`
    ${stateIcon};
    color: orangered;
`
export const breakIcon = css`
    ${stateIcon};
    color: lime;
    font-size: ${stateFontSize}px;
`
