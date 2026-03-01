import { Global as EmotionGlobal } from '@emotion/react'
import { withThemeFromJSXProvider } from '@storybook/addon-themes'
import type { Preview } from '@storybook/nextjs-vite'
// biome-ignore lint/correctness/noUnusedImports: JSX のランタイム／型チェックで React がスコープに必要（jsx: preserve）
import React from 'react'
import { globalStyle } from '../src/styles/global.styles'

const GlobalStyles = () => <EmotionGlobal styles={globalStyle} />

const preview: Preview = {
	parameters: {
		// Controls は addon-docs 等で表示される際のマッチャー（SB10 では addon-essentials は addon-docs に統合）
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
