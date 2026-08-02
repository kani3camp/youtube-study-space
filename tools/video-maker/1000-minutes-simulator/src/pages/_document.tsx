import Document, { Head, Html, Main, NextScript } from 'next/document'
import type { ReactElement } from 'react'

class MyDocument extends Document {
	render(): ReactElement {
		return (
			<Html>
				<Head>
					<link
						href="https://fonts.googleapis.com/css2?family=Inter"
						rel="stylesheet"
					/>
					<link rel="preconnect" href="https://fonts.googleapis.com" />
					<link
						rel="preconnect"
						href="https://fonts.gstatic.com"
						crossOrigin="anonymous"
					/>
					<link
						href="https://fonts.googleapis.com/css2?family=M+PLUS+Rounded+1c:wght@300&display=swap"
						rel="stylesheet"
					/>

					<link
						href="https://fonts.googleapis.com/css2?family=Zen+Maru+Gothic&display=swap"
						rel="stylesheet"
					/>
					<link
						href="https://fonts.googleapis.com/css2?family=Noto+Serif+JP:wght@500&display=swap"
						rel="stylesheet"
					/>
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
