import { Global } from '@emotion/react'
import { appWithTranslation } from 'next-i18next'
import { AppProps } from 'next/app'
import { globalStyle } from '../styles/global.styles'

function App({ Component, pageProps }: AppProps): JSX.Element {
    if (process.env.NEXT_PUBLIC_API_KEY === undefined) {
        console.error('Environment variable NEXT_PUBLIC_API_KEY is not defined')
    }
    return (
        <>
            <Global styles={globalStyle} />
            <Component {...pageProps} />
        </>
    )
}

export default appWithTranslation(App)
