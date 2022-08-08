import { css } from '@emotion/react'
import {
    basicCell,
    basicInnerCell,
    BGMPlayerHeight,
    BGMPlayerWidth,
} from './common.styles'

export const bgmPlayer = css`
    ${basicCell};
    height: ${BGMPlayerHeight}px;
    width: ${BGMPlayerWidth}px;
    position: absolute;
    bottom: 0;
    left: 0;
`

export const innerCell = css`
    ${basicInnerCell};
`
