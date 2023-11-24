import type { StorybookConfig } from '@storybook/nextjs'

const config: StorybookConfig = {
    stories: ['../src/**/*.mdx', '../src/**/*.stories.@(js|jsx|mjs|ts|tsx)'],
    addons: [
        '@storybook/addon-links',
        '@storybook/addon-essentials',
        '@storybook/addon-onboarding',
        '@storybook/addon-interactions',
        '@storybook/addon-themes',
    ],
    framework: {
        name: '@storybook/nextjs',
        options: {},
    },
    docs: {
        autodocs: 'tag',
    },
    webpackFinal: async (config, { configType }) => {
        config.module?.rules?.push({
            test: /\.(js|jsx|ts|tsx)$/,
            use: [
                {
                    loader: 'babel-loader',
                    options: {
                        plugins: ['@emotion/babel-plugin'],
                    },
                },
            ],
        })
        if (config.resolve && config.resolve.alias) {
            config.resolve.alias = {
                ...config.resolve.alias,
                'next-i18next': 'react-i18next',
            }
        }
        return config
    },
    staticDirs: ['../public'],
}
export default config
