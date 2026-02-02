/** @type {import('@docusaurus/plugin-content-docs').SidebarsConfig} */
const sidebars = {
  tutorialSidebar: [
    'intro',
    'getting-started',
    {
      type: 'category',
      label: 'Usage',
      items: [
        'usage/cli-reference',
        'usage/nix-builder',
        'usage/private-repos',
      ],
    },
    {
      type: 'category',
      label: 'Reference',
      items: [
        'reference/lockfile-format',
        'reference/architecture',
      ],
    },
  ],
};

export default sidebars;
