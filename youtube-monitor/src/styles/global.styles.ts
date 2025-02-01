import { css as globalStyles } from '@emotion/react'
import { fontFamily } from '../lib/common'

export const globalStyle = globalStyles`
  html {
    font-family: ${fontFamily};
    font-size: xx-large;
  }

  body {
    margin: 0;
  }
`
