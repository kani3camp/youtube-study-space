import type * as Preset from '@docusaurus/preset-classic';
import type { Config } from '@docusaurus/types';
import { themes as prismThemes } from 'prism-react-renderer';

const config: Config = {
	title: 'YouTubeオンライン作業部屋 コマンド一覧',
	tagline: 'ライブチャットに書き込もう',
	favicon: 'img/favicon.ico',

	// Set the production url of your site here
	url: 'https://sorarideblog.github.io',
	// Set the /<baseUrl>/ pathname under which your site is served
	// For GitHub pages deployment, it is often '/<projectName>/'
	baseUrl: '/youtube-study-space',
	// Don't use trailing slash for consistent SEO
	trailingSlash: false,

	// GitHub pages deployment config.
	organizationName: 'kani3camp', // Usually your GitHub org/user name.
	projectName: 'youtube-study-space', // Usually your repo name.
	deploymentBranch: 'docusaurus',

	onBrokenLinks: 'throw',
	onBrokenMarkdownLinks: 'warn',

	i18n: {
		defaultLocale: 'ja',
		locales: ['ja', 'en', 'zh-Hant', 'ko'],
		path: 'i18n',
		localeConfigs: {
			ja: {
				label: '日本語',
				direction: 'ltr',
				htmlLang: 'ja',
			},
			en: {
				label: 'English',
				direction: 'ltr',
				htmlLang: 'en',
			},
			'zh-Hant': {
				label: '繁體中文',
				direction: 'ltr',
				htmlLang: 'zh-Hant',
			},
			ko: {
				label: '한국어',
				direction: 'ltr',
				htmlLang: 'ko',
			},
		},
	},

	themes: ['@docusaurus/theme-mermaid'],

	markdown: {
		mermaid: true,
	},

	presets: [
		[
			'classic',
			{
				docs: {
					sidebarPath: './sidebars.ts',
					// Please change this to your repo.
					// Remove this to remove the "edit this page" links.
					editUrl: 'https://github.com/kani3camp/youtube-study-space/tree/dev/docs-site/',
				},
				theme: {
					customCss: './src/css/custom.css',
				},
				gtag: {
					trackingID: 'G-7CHFZXDRCT',
				},
			} satisfies Preset.Options,
		],
	],

	themeConfig: {
		docs: {
			sidebar: {
				hideable: true,
			},
		},
		navbar: {
			title: 'YouTubeオンライン作業部屋',
			logo: {
				alt: 'Site Logo',
				src: 'img/logo.svg',
			},
			items: [
				{
					type: 'docSidebar',
					sidebarId: 'tutorialSidebar',
					position: 'left',
					label: 'ドキュメント',
				},
				{
					type: 'localeDropdown',
					position: 'right',
				},
				{
					href: 'https://github.com/kani3camp/youtube-study-space',
					label: 'GitHub',
					position: 'right',
				},
			],
		},
		footer: {
			style: 'dark',
			links: [
				{
					title: 'ドキュメント',
					items: [
						{
							label: 'コマンド一覧',
							to: '/docs/essential',
						},
						{
							label: '公開資料',
							href: 'https://youtube-study-space.notion.site/5021213988a34747a7513f1067deb76d',
						},
					],
				},
				{
					title: 'コミュニティ',
					items: [
						{
							label: 'YouTubeコミュニティ',
							href: 'https://www.youtube.com/channel/UCXuD2XmPTdpVy7zmwbFVZWg/community',
						},
						{
							label: 'YouTubeメンバーシップ',
							href: 'https://www.youtube.com/channel/UCXuD2XmPTdpVy7zmwbFVZWg/join',
						},
						{
							label: 'Discord',
							href: 'https://discord.gg/h9SenAvawT',
						},
						{
							label: 'X',
							href: 'https://twitter.com/osr_soraride',
						},
					],
				},
				{
					title: 'More',
					items: [
						{
							label: 'GitHub',
							href: 'https://github.com/kani3camp/youtube-study-space',
						},
					],
				},
			],
			copyright: `Copyright © ${new Date().getFullYear()} らいど開発`,
		},
		prism: {
			theme: prismThemes.github,
			darkTheme: prismThemes.dracula,
		},
		colorMode: {
			defaultMode: 'dark',
			respectPrefersColorScheme: true,
		},
	} satisfies Preset.ThemeConfig,
};

export default config;
