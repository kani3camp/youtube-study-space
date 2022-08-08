import { css } from '@emotion/react'
import {
    ElapsedHeight,
    ElapsedWidth,
    ElapsedLeft,
    basicCell,
    basicInnerCell,
} from './common.styles'

export const elapsed = css`
    ${basicCell};
    height: ${ElapsedHeight}px;
    width: ${ElapsedWidth}px;
    position: absolute;
    top: 0;
    left: ${ElapsedLeft}px;
    text-align: center;
`

export const innerCell = css`
    ${basicInnerCell};
`

export const elapsedTime = css`
    span {
        margin: 0 0.2rem;
    }
`
