import { Global } from '@emotion/react'
import { appWithTranslation } from 'next-i18next'
import type { AppProps } from 'next/app'
import type { ReactElement } from 'react'
import { globalStyle } from '../styles/global.styles'

function MyApp({ Component, pageProps }: AppProps): ReactElement {
	return (
		<>
			<Global styles={globalStyle} />
			<Component {...pageProps} />
		</>
	)
}

export default appWithTranslation(MyApp)
