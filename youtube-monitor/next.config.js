const { i18n } = require('./next-i18next.config')

/** @type {import('next').NextConfig} */
const nextConfig = {
	i18n,
	images: {
		remotePatterns: [
			{
				protocol: 'https',
				hostname: 'yt3.ggpht.com',
				port: '',
				pathname: '/**',
			},
		],
	},
	reactStrictMode: true,
	compiler: {
		emotion: {
			sourceMap: true,
			autoLabel: 'dev-only',
			labelFormat: '[filename]--[local]',
			cssPropOptimization: true,
		},
	},
}

module.exports = nextConfig
