import React from 'react'
import type { Preview } from '@storybook/react'

import { css, Global as EmotionGlobal } from '@emotion/react'
import { withThemeFromJSXProvider } from '@storybook/addon-themes'
import { globalStyle } from '../src/styles/global.styles'

const GlobalStyles = () => <EmotionGlobal styles={globalStyle} />

const preview: Preview = {
    parameters: {
        controls: {
            matchers: {
                color: /(background|color)$/i,
                date: /Date$/i,
            },
        },
    },

    decorators: [
        withThemeFromJSXProvider({
            GlobalStyles,
        }),
    ],
}

export default preview
