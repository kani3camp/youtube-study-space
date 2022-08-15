import { css } from '@emotion/react'
import {
    iconBase,
    basicCell,
    basicInnerCell,
    BGMPlayerHeight,
    BGMPlayerWidth,
    InnerMargin,
    OuterMargin,
} from './common.styles'

export const bgmPlayer = css`
    ${basicCell};
    /* height: ${BGMPlayerHeight}px;
    width: ${BGMPlayerWidth}px; */
    grid-column-start: 1;
    grid-column-end: 4;
    grid-row-start: 3;
    grid-row-end: 4;
`

export const innerCell = css`
    ${basicInnerCell};
    height: calc(100% - ${OuterMargin + InnerMargin}px);
    width: calc(100% - ${OuterMargin + InnerMargin}px);
    margin: ${InnerMargin}px ${InnerMargin}px ${OuterMargin}px ${OuterMargin}px;
`

export const icon = css`
    ${iconBase};
    color: #41e6e6;
`
