module.exports = {
  docsSidebar: [
    {
      type: 'category',
      label: 'Getting Started',
      items: [
        'getting-started/overview',
        'getting-started/installation',
        'getting-started/quick-start',
        'getting-started/connect-kubernetes-gateway',
      ],
    },
    {
      type: 'category',
      label: 'Concepts',
      items: ['concepts/architecture', 'concepts/managed-agents', 'concepts/attributes-and-labels'],
    },
    {
      type: 'category',
      label: 'Fleet Management',
      items: ['fleet-management/inventory', 'fleet-management/configuration'],
    },
    {
      type: 'category',
      label: 'Integrations',
      items: ['integrations/overview'],
    },
    {
      type: 'category',
      label: 'Development',
      items: ['development/build-from-source'],
    },
    'roadmap',
  ],
};
