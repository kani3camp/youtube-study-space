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

export const roundString = css`
    font-size: 0.7rem;
    color: #a57ec1;
`

export const icon = css`
    ${iconBase};
    color: magenta;
`
