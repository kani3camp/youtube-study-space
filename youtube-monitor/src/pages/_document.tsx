import Document, { Head, Html, Main, NextScript } from 'next/document'

class MyDocument extends Document {
    render(): JSX.Element {
        return (
            <Html>
                <Head>
                    <link href='https://fonts.googleapis.com/css2?family=Inter' rel='stylesheet' />
                    <link rel='preconnect' href='https://fonts.googleapis.com' />
                    <link
                        rel='preconnect'
                        href='https://fonts.gstatic.com'
                        crossOrigin='anonymous'
                    />
                    <link
                        href='https://fonts.googleapis.com/css2?family=M+PLUS+Rounded+1c&display=swap'
                        rel='stylesheet'
                    ></link>

                    <link
                        href='https://fonts.googleapis.com/css2?family=Zen+Maru+Gothic&display=swap'
                        rel='stylesheet'
                    ></link>
                </Head>
                <body>
                    <Main />
                    <NextScript />
                </body>
            </Html>
        )
    }
}

export default MyDocument
