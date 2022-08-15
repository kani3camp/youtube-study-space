import { css } from '@emotion/react'
import {
    basicCell,
    basicInnerCell,
    GaugeHeight,
    GaugeWidth,
    InnerMargin,
    OuterMargin,
    iconBase,
} from './common.styles'

export const gauge = css`
    ${basicCell};
    /* height: ${GaugeHeight}px;
    width: ${GaugeWidth}px; */
    text-align: center;
    grid-column-start: 4;
    grid-column-end: 5;
    grid-row-start: 1;
    grid-row-end: 4;
`

export const innerCell = css`
    ${basicInnerCell};
    height: calc(100% - ${2 * OuterMargin}px);
    width: calc(100% - ${OuterMargin + InnerMargin}px);
    margin: ${OuterMargin}px ${OuterMargin}px ${OuterMargin}px ${InnerMargin}px;
    position: relative;
`

export const icon = css`
    ${iconBase};
    color: #b89f14;
`

export const unit = css`
    position: absolute;
    font-size: 15px;
    bottom: 10px;
    right: 130px;
`
