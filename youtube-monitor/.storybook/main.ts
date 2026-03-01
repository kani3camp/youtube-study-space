import type { StorybookConfig } from '@storybook/nextjs-vite'

const config: StorybookConfig = {
	stories: ['../src/**/*.mdx', '../src/**/*.stories.@(js|jsx|mjs|ts|tsx)'],
	addons: [
		'@storybook/addon-onboarding',
		'@chromatic-com/storybook',
		'@storybook/addon-themes',
		'@storybook/addon-docs',
	],
	framework: {
		name: '@storybook/nextjs-vite',
		options: {},
	},
	staticDirs: ['../public'],
	async viteFinal(viteConfig) {
		return {
			...viteConfig,
			esbuild: {
				...(viteConfig.esbuild ?? {}),
				jsx: 'automatic',
				jsxImportSource: '@emotion/react',
			},
		}
	},
}
export default config
