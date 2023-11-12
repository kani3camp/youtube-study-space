import { withThemeFromJSXProvider } from '@storybook/addon-themes'
import { Global } from '@emotion/react'
import React from 'react'
import { globalStyle } from '../src/styles/global.styles'

const GlobalStyles = () => <Global styles={globalStyle} />

export const decorators = [
    withThemeFromJSXProvider({
        GlobalStyles,
    }),
]
