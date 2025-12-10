import type {SidebarsConfig} from '@docusaurus/plugin-content-docs';

const sidebars: SidebarsConfig = {
  docsSidebar: [
    {
      type: 'doc',
      id: 'README',
      label: 'Overview',
    },
    {
      type: 'category',
      label: 'Getting Started',
      items: [
        'getting-started/QUICKSTART',
        'getting-started/DEVELOPMENT',
        'getting-started/BUILD_SETUP',
      ],
    },
    {
      type: 'category',
      label: 'Architecture',
      items: [
        'architecture/ARCHITECTURE',
        // 'architecture/API', // Temporarily excluded - contains {slug} patterns that break MDX
        'architecture/BLUE_OCEAN_ROADMAP',
      ],
    },
    {
      type: 'category',
      label: 'Guides',
      items: [
        'guides/DOGFOODING_GUIDE',
        'guides/RAILWAY_MIGRATION_GUIDE',
        'guides/VERCEL_MIGRATION_GUIDE',
        'guides/TESTING_GUIDE',
      ],
    },
    {
      type: 'category',
      label: 'Production',
      items: [
        'production/PRODUCTION_READINESS_AUDIT',
        'production/PRODUCTION_DEPLOYMENT_ROADMAP',
        'production/GAP_ANALYSIS',
      ],
    },
    {
      type: 'category',
      label: 'Audits',
      items: [
        {
          type: 'category',
          label: 'Security',
          items: [
            'audits/security/EXECUTIVE_SUMMARY',
            'audits/security/COMPREHENSIVE_AUDIT',
            'audits/security/QUICK_REFERENCE',
          ],
        },
        {
          type: 'category',
          label: 'Codebase',
          items: [
            'audits/codebase/GO_SUMMARY',
            'audits/codebase/COMPREHENSIVE_AUDIT',
          ],
        },
        {
          type: 'category',
          label: 'UI',
          items: [
            'audits/ui/EXECUTIVE_SUMMARY',
            'audits/ui/COMPREHENSIVE_AUDIT',
          ],
        },
      ],
    },
    {
      type: 'category',
      label: 'Implementation',
      items: [
        'implementation/MVP_IMPLEMENTATION',
        'implementation/BUILD_PIPELINE_IMPLEMENTATION',
        'implementation/CLI_IMPLEMENTATION_COMPLETE',
      ],
    },
  ],
};

export default sidebars;
