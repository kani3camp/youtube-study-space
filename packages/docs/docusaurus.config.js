// @ts-check
// Note: type annotations allow type checking and IDEs autocompletion

const lightCodeTheme = require("prism-react-renderer/themes/github");
const darkCodeTheme = require("prism-react-renderer/themes/dracula");

/** @type {import('@docusaurus/types').Config} */
const config = {
  title: "Study w/ 王攻子 開発ドキュメント",
  tagline: "",
  url: "https://docs.studywith.oceme.co",
  baseUrl: "/",
  onBrokenLinks: "throw",
  onBrokenMarkdownLinks: "warn",
  favicon: "img/favicon.ico",
  organizationName: "doanryo", // Usually your GitHub org/user name.
  projectName: "studywithocemeco", // Usually your repo name.
  i18n: {
    defaultLocale: "ja",
    locales: ["ja"],
  },

  presets: [
    [
      "classic",
      /** @type {import('@docusaurus/preset-classic').Options} */
      ({
        docs: {
          sidebarPath: require.resolve("./sidebars.js"),
          editUrl: "https://github.com/doanryo/studywithocemeco/tree/dev/packages/docs",
          routeBasePath: "/",
        },
        theme: {
          customCss: require.resolve("./src/css/custom.css"),
        },
      }),
    ],
  ],

  themeConfig:
    /** @type {import('@docusaurus/preset-classic').ThemeConfig} */
    ({
      navbar: {
        title: "Study w/ 王攻子 開発ドキュメント",
        items: [
          {
            href: "https://github.com/doanryo/studywithocemeco",
            label: "GitHub",
            position: "right",
          },
        ],
      },
      footer: {
        style: "dark",
        links: [
          {
            title: "SNS",
            items: [
              {
                label: "Twitter",
                href: "https://twitter.com/ocemeco",
              },
            ],
          },
          {
            title: "リソース",
            items: [
              {
                label: "GitHub",
                href: "https://github.com/doanryo/studywithocemeco",
              },
            ],
          },
        ],
        copyright: `Copyright © ${new Date().getFullYear()} doanryo. Built with Docusaurus.`,
      },
      prism: {
        theme: lightCodeTheme,
        darkTheme: darkCodeTheme,
      },
    }),
};

module.exports = config;
