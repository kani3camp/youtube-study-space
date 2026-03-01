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
		// Vite の esbuild は ESBuildOptions | false になり得るため、object の場合のみマージ
		const baseEsbuildConfig =
			typeof viteConfig.esbuild === 'object' && viteConfig.esbuild !== null
				? viteConfig.esbuild
				: {}
		return {
			...viteConfig,
			esbuild: {
				...baseEsbuildConfig,
				jsx: 'automatic',
				jsxImportSource: '@emotion/react',
			},
		}
	},
}
export default config
