import fs from 'node:fs'
import path from 'node:path'
import { fileURLToPath } from 'node:url'
import type { StorybookConfig } from '@storybook/nextjs-vite'

const configDir = path.dirname(fileURLToPath(import.meta.url))

// public/audio には大容量の配信用 BGM が入るため、Storybook の静的配信・コピー対象から除外する。
// public/chime など gitignore 対象の任意ディレクトリはローカルに存在する場合のみマウントする。
const staticDirCandidates = [
	{ from: '../public/images', to: '/images' },
	{ from: '../public/locales', to: '/locales' },
	{ from: '../public/chime', to: '/chime' },
]

const staticDirs = staticDirCandidates.filter(({ from }) =>
	fs.existsSync(path.resolve(configDir, from)),
)

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
	staticDirs,
	async viteFinal(viteConfig) {
		// Vite の esbuild は ESBuildOptions | false になり得るため、object の場合のみマージする。
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
