import { Global as EmotionGlobal } from '@emotion/react'
import { withThemeFromJSXProvider } from '@storybook/addon-themes'
import type { Preview } from '@storybook/nextjs-vite'
import { globalStyle } from '../src/styles/global.styles'

const GlobalStyles = () => <EmotionGlobal styles={globalStyle} />

const preview: Preview = {
	parameters: {
		// addon-docs 等の Controls 表示で使われるマッチャー設定（SB10 では addon-essentials が addon-docs に統合）
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
