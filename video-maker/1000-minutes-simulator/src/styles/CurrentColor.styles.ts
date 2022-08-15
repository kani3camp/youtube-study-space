import { css } from '@emotion/react'
import {
    basicCell,
    ColorBoxHeight,
    ColorBoxWidth,
    basicInnerCell,
    OuterMargin,
    InnerMargin,
    iconBase,
} from './common.styles'

export const currentColor = css`
    ${basicCell};
    text-align: center;
    grid-column-start: 3;
    grid-column-end: 4;
    grid-row-start: 1;
    grid-row-end: 2;
`

export const innerCell = css`
    ${basicInnerCell};
    height: calc(100% - ${OuterMargin + InnerMargin}px);
    width: calc(100% - ${2 * InnerMargin}px);
    margin: ${OuterMargin}px ${InnerMargin}px ${InnerMargin}px ${InnerMargin}px;
`

export const annotation = css`
    font-size: 0.6rem;
    color: #929292;
`

export const colorBox = css`
    height: ${ColorBoxHeight}px;
    width: ${ColorBoxWidth}px;
    border: solid black 0.1rem;
    margin: 2rem auto;

    /* 光らせるためのstyle */
    border: 2px solid transparent;
    position: relative;
    overflow: hidden;
    /* 光の疑似要素 */
    ::before {
        content: '';
        animation: shine 5s cubic-bezier(0.25, 0, 0.25, 1) infinite;
        background-color: #fff;
        width: 140%;
        height: 100%;
        transform: skewX(-45deg);
        top: 0;
        left: -200%;
        opacity: 0.5;
        position: absolute;
    }
    /* 光の動き */
    @keyframes shine {
        0% {
            left: -200%;
            opacity: 0;
        }
        70% {
            left: -200%;
            opacity: 0.5;
        }
        71% {
            left: -200%;
            opacity: 1;
        }
        100% {
            left: -20%;
            opacity: 0;
        }
    }
`
export const icon = css`
    ${iconBase};
    color: #15c827;
`
