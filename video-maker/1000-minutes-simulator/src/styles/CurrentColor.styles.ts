import { css } from '@emotion/react'
import {
    CurrentColorHeight,
    CurrentColorWidth,
    CurrentColorLeft,
    basicCell,
    ColorBoxHeight,
    ColorBoxWidth,
    basicInnerCell,
} from './common.styles'

export const currentColor = css`
    ${basicCell};
    height: ${CurrentColorHeight}px;
    width: ${CurrentColorWidth}px;
    position: absolute;
    top: 0;
    left: ${CurrentColorLeft}px;
    text-align: center;
`

export const innerCell = css`
    ${basicInnerCell};
`

export const annotation = css`
    font-size: 0.5rem;
    color: #747474;
`

export const colorBox = css`
    height: ${ColorBoxHeight}px;
    width: ${ColorBoxWidth}px;
    border: solid black 0.1rem;
    margin: 0.5rem auto;
`
