const { i18n } = require('./next-i18next.config')
const {
	PHASE_PRODUCTION_BUILD,
	PHASE_PRODUCTION_SERVER,
} = require('next/constants')
const { validatePublicEnv } = require('./next-public-env')

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

// 本番ビルド・起動時にだけ公開環境変数を検証する。
// 開発サーバーや Storybook(STORYBOOK=true) は設定なしでも読み込めるようにし、src 側は利用時に検証する。
module.exports = (phase) => {
	const isStorybook = process.env.STORYBOOK === 'true'
	if (
		!isStorybook &&
		(phase === PHASE_PRODUCTION_BUILD || phase === PHASE_PRODUCTION_SERVER)
	) {
		validatePublicEnv(process.env)
	}
	return nextConfig
}
