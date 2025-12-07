import { css } from '@emotion/react'
import {
	BGMPlayerHeight,
	BGMPlayerWidth,
	InnerMargin,
	OuterMargin,
	basicCell,
	basicInnerCell,
	iconBase,
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

export const item = css`
    margin: 0.1rem 0.3rem;
    svg {
        color: #8fd2d2;
        vertical-align: middle;
    }

    span {
        vertical-align: middle;
        margin-left: 0.5rem;
    }
`
