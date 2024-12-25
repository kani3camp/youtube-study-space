import { css as globalStyles } from '@emotion/react'
import { Constants } from '../lib/constants'

export const globalStyle = globalStyles`
  html {
    font-family: ${Constants.fontFamily};
    font-size: xx-large;
  }

  body {
    margin: 0;
  }
`
