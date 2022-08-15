import { css } from '@emotion/react'
import {
    basicCell,
    basicInnerCell,
    InnerMargin,
    OuterMargin,
    iconBase,
} from './common.styles'

export const tips = css`
    ${basicCell};
    grid-column-start: 1;
    grid-column-end: 4;
    grid-row-start: 2;
    grid-row-end: 3;
`

export const innerCell = css`
    ${basicInnerCell};
    height: calc(100% - ${2 * InnerMargin}px);
    width: calc(100% - ${InnerMargin + OuterMargin}px);
    margin: ${InnerMargin}px ${InnerMargin}px ${InnerMargin}px ${OuterMargin}px;
`

export const tipsMain = css`
    font-weight: bold;
    font-family: serif;
`

export const icon = css`
    ${iconBase};
    color: #f9f954;
`
