import { css } from '@emotion/react'
import {
    TimerHeight,
    TimerWidth,
    basicCell,
    basicInnerCell,
} from './common.styles'

export const timer = css`
    ${basicCell};
    height: ${TimerHeight}px;
    width: ${TimerWidth}px;
    position: absolute;
    top: 0;
    left: 0;
    text-align: center;
`

export const innerCell = css`
    ${basicInnerCell};
`

export const progressBarContainer = css`
    width: 50%;
    margin: 0 auto;
`
