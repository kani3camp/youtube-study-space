import React from 'react'
import type { Preview } from '@storybook/react'

import { Global as EmotionGlobal, css } from '@emotion/react'
import { withThemeFromJSXProvider } from '@storybook/addon-themes'
import * as globalStyles from '../src/styles/global.styles'

const GlobalStyles = () => <EmotionGlobal styles={globalStyles} />

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
