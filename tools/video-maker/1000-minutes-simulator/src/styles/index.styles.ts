import { css } from '@emotion/react'
import * as common from './common.styles'

export const indexStyle = css`
    position: relative;
    height: ${common.ScreenHeight}px;
    width: ${common.ScreenWidth}px;
    display: grid;
    grid-template-columns: 3fr 3fr 3fr 2fr;
    grid-template-rows: 5fr 5fr 2fr;
    background-size: cover;
`
