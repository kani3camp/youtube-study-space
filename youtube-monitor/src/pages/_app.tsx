import { Global } from '@emotion/react'
import type { AppProps } from 'next/app'
import { appWithTranslation } from 'next-i18next/pages'
import 'react-circular-progressbar/dist/styles.css'
import { fontClassName } from '../lib/common'
import { seatAppearanceSchemaReadCapability } from '../lib/seat-appearance-schema'
import { globalStyle } from '../styles/global.styles'

function App({ Component, pageProps }: AppProps): JSX.Element {
	return (
		<div
			className={fontClassName}
			data-seat-appearance-schema-read={seatAppearanceSchemaReadCapability}
		>
			<Global styles={globalStyle} />
			<Component {...pageProps} />
		</div>
	)
}

export default appWithTranslation(App)
