import { Global } from '@emotion/react'
import type { AppProps } from 'next/app'
import { appWithTranslation } from 'next-i18next'
import 'react-circular-progressbar/dist/styles.css'
import { fontClassName } from '../lib/common'
import { globalStyle } from '../styles/global.styles'

function App({ Component, pageProps }: AppProps): JSX.Element {
	return (
		<div className={fontClassName}>
			<Global styles={globalStyle} />
			<Component {...pageProps} />
		</div>
	)
}

export default appWithTranslation(App)
