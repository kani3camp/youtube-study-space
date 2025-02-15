import { css } from '@emotion/react'
import {
	InnerMargin,
	OuterMargin,
	basicCell,
	basicInnerCell,
	iconBase,
} from './common.styles'

export const elapsed = css`
    ${basicCell};
    text-align: center;
    grid-column-start: 2;
    grid-column-end: 3;
    grid-row-start: 1;
    grid-row-end: 2;
`

export const innerCell = css`
    ${basicInnerCell};
    height: calc(100% - ${OuterMargin + InnerMargin}px);
    width: calc(100% - ${2 * InnerMargin}px);
    margin: ${OuterMargin}px ${InnerMargin}px ${InnerMargin}px ${InnerMargin}px;
`

export const elapsedTime = css`
    align-items: center;
    margin-top: 2rem;
    font-size: 2rem;
    font-weight: bold;
`

export const elapsedTimeSubscript = css`
    font-size: 1rem;
    margin-left: 0.2rem;
    margin-right: 0.5rem;
    font-weight: normal;
`

export const icon = css`
    ${iconBase};
    color: #2e7fef;
`

export const elapsedTime2 = css`
    ${elapsedTime};
    font-size: 1.5rem;
`
