import { css } from '@emotion/react'
import { FontFamily } from './common.styles'

export const globalStyle = css`
    html,
    body {
        padding: 0;
        margin: 0;
        font-family: ${FontFamily};
        font-size: 30px;
        color: #ffffff;
        background-color: #1d152c;
    }

    a {
        color: inherit;
        text-decoration: none;
    }

    * {
        box-sizing: border-box;
    }
`
