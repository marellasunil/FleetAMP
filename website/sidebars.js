module.exports = {
  docsSidebar: [
    { type: 'category', label: 'Getting Started', items: [
      'getting-started/overview', 'getting-started/installation', 'getting-started/connect-kubernetes-gateway',
    ]},
    { type: 'category', label: 'Fleet Management', items: [
      'fleet-management/managed-collectors', 'fleet-management/metadata-labels', 'fleet-management/groups',
    ]},
    { type: 'category', label: 'Configuration', items: [
      'fleet-management/configuration', 'configuration/deploy', 'configuration/drift', 'configuration/rollback',
    ]},
    { type: 'category', label: 'Operations', items: [
      'operations/operations-guide', 'operations/production-deployment', 'operations/os-deployment', 'operations/troubleshooting',
    ]},
    { type: 'category', label: 'Reference', items: [
      'reference/api', 'reference/architecture', 'roadmap',
    ]},
    { type: 'category', label: 'Concepts', collapsed: true, items: [
      'concepts/architecture', 'concepts/managed-agents', 'concepts/attributes-and-labels',
    ]},
    { type: 'category', label: 'Development', collapsed: true, items: ['development/build-from-source'] },
  ],
};
