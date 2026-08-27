const config = {
  title: 'FleetAMP',
  tagline: 'Open fleet management for telemetry collectors',
  favicon: 'img/favicon.svg',
  url: 'https://fleetamp.marellasunil.com',
  baseUrl: '/',
  organizationName: 'marellasunil',
  projectName: 'FleetAMP',
  onBrokenLinks: 'throw',
  onBrokenMarkdownLinks: 'warn',
  trailingSlash: false,
  presets: [
    [
      'classic',
      {
        docs: {
          routeBasePath: 'docs',
          sidebarPath: require.resolve('./sidebars.js'),
          editUrl: 'https://github.com/marellasunil/FleetAMP/edit/main/website/',
        },
        blog: false,
        theme: { customCss: require.resolve('./src/css/custom.css') },
      },
    ],
  ],
  themeConfig: {
    navbar: {
      title: 'FleetAMP',
      items: [
        { to: '/docs/getting-started/overview', label: 'Docs', position: 'left' },
        { to: '/docs/concepts/architecture', label: 'Architecture', position: 'left' },
        { to: '/docs/roadmap', label: 'Roadmap', position: 'left' },
        { href: 'https://github.com/marellasunil/FleetAMP', label: 'GitHub', position: 'right' },
      ],
    },
    footer: {
      style: 'dark',
      links: [
        { title: 'Docs', items: [{ label: 'Getting Started', to: '/docs/getting-started/overview' }, { label: 'Architecture', to: '/docs/concepts/architecture' }] },
        { title: 'Community', items: [{ label: 'GitHub', href: 'https://github.com/marellasunil/FleetAMP' }, { label: 'Contributing', href: 'https://github.com/marellasunil/FleetAMP/blob/main/CONTRIBUTING.md' }] },
      ],
      copyright: `Copyright © ${new Date().getFullYear()} FleetAMP contributors. Apache-2.0 licensed.`,
    },
  },
};

module.exports = config;
