import { css } from '@emotion/react'
import {
    basicCell,
    basicInnerCell,
    GaugeHeight,
    GaugeWidth,
} from './common.styles'

export const gauge = css`
    ${basicCell};
    height: ${GaugeHeight}px;
    width: ${GaugeWidth}px;
    position: absolute;
    top: 0;
    right: 0;
    text-align: center;
`

export const innerCell = css`
    ${basicInnerCell};
`
