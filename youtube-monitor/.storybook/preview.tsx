import { withThemeFromJSXProvider } from '@storybook/addon-themes'
import { Global } from '@emotion/react'
import React from 'react'
import { globalStyle } from '../src/styles/global.styles'
import { I18nextProvider } from 'react-i18next'
import i18n from './i18n'
import type { Preview } from '@storybook/react'

const GlobalStyles = () => <Global styles={globalStyle} />

const preview: Preview = {
    parameters: {
        actions: { argTypesRegex: '^on[A-Z].*' },
        controls: {
            matchers: {
                color: /(background|color)$/i,
                date: /Date$/i,
            },
        },
        layout: 'centered',
    },
    decorators: [
        withThemeFromJSXProvider({
            GlobalStyles,
        }),
        (Story) => (
            <I18nextProvider i18n={i18n}>
                <Story />
            </I18nextProvider>
        ),
    ],
}

export default preview
