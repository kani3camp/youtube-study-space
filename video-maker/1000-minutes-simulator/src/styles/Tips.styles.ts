import { css } from '@emotion/react'
import {
    TipsHeight,
    TipsWidth,
    TipsTop,
    basicCell,
    basicInnerCell,
} from './common.styles'

export const tips = css`
    ${basicCell};
    height: ${TipsHeight}px;
    width: ${TipsWidth}px;
    position: absolute;
    top: ${TipsTop}px;
    left: 0;
`

export const innerCell = css`
    ${basicInnerCell};
`

export const tipsMain = css`
    font-weight: bold;
`
