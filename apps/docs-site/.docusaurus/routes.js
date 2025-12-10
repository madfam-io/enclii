import React from 'react';
import ComponentCreator from '@docusaurus/ComponentCreator';

export default [
  {
    path: '/',
    component: ComponentCreator('/', '4a2'),
    routes: [
      {
        path: '/',
        component: ComponentCreator('/', '863'),
        routes: [
          {
            path: '/',
            component: ComponentCreator('/', 'e58'),
            routes: [
              {
                path: '/architecture/ARCHITECTURE',
                component: ComponentCreator('/architecture/ARCHITECTURE', 'a5c'),
                exact: true,
                sidebar: "docsSidebar"
              },
              {
                path: '/architecture/BLUE_OCEAN_ROADMAP',
                component: ComponentCreator('/architecture/BLUE_OCEAN_ROADMAP', '928'),
                exact: true,
                sidebar: "docsSidebar"
              },
              {
                path: '/architecture/DOGFOODING_MAP',
                component: ComponentCreator('/architecture/DOGFOODING_MAP', '8bd'),
                exact: true
              },
              {
                path: '/architecture/ENCLII_CAPABILITY_MATRIX',
                component: ComponentCreator('/architecture/ENCLII_CAPABILITY_MATRIX', 'c76'),
                exact: true
              },
              {
                path: '/architecture/ENCLII_EXECUTIVE_SUMMARY',
                component: ComponentCreator('/architecture/ENCLII_EXECUTIVE_SUMMARY', 'd27'),
                exact: true
              },
              {
                path: '/architecture/ENCLII_QUICK_REFERENCE',
                component: ComponentCreator('/architecture/ENCLII_QUICK_REFERENCE', '0b3'),
                exact: true
              },
              {
                path: '/architecture/SOFTWARE_SPEC',
                component: ComponentCreator('/architecture/SOFTWARE_SPEC', '388'),
                exact: true
              },
              {
                path: '/audits/',
                component: ComponentCreator('/audits/', '7f3'),
                exact: true
              },
              {
                path: '/audits/codebase/COMPREHENSIVE_AUDIT',
                component: ComponentCreator('/audits/codebase/COMPREHENSIVE_AUDIT', 'be6'),
                exact: true,
                sidebar: "docsSidebar"
              },
              {
                path: '/audits/codebase/ENCLII_COMPREHENSIVE_AUDIT',
                component: ComponentCreator('/audits/codebase/ENCLII_COMPREHENSIVE_AUDIT', '821'),
                exact: true
              },
              {
                path: '/audits/codebase/GO_AUDIT_REPORT',
                component: ComponentCreator('/audits/codebase/GO_AUDIT_REPORT', 'aae'),
                exact: true
              },
              {
                path: '/audits/codebase/GO_SUMMARY',
                component: ComponentCreator('/audits/codebase/GO_SUMMARY', '877'),
                exact: true,
                sidebar: "docsSidebar"
              },
              {
                path: '/audits/codebase/QUICK_REFERENCE',
                component: ComponentCreator('/audits/codebase/QUICK_REFERENCE', '7e0'),
                exact: true
              },
              {
                path: '/audits/codebase/SWITCHYARD_AUDIT',
                component: ComponentCreator('/audits/codebase/SWITCHYARD_AUDIT', '27f'),
                exact: true
              },
              {
                path: '/audits/dependencies/',
                component: ComponentCreator('/audits/dependencies/', '184'),
                exact: true
              },
              {
                path: '/audits/dependencies/AUDIT_CHECKLIST',
                component: ComponentCreator('/audits/dependencies/AUDIT_CHECKLIST', 'de6'),
                exact: true
              },
              {
                path: '/audits/dependencies/COMPREHENSIVE_ANALYSIS',
                component: ComponentCreator('/audits/dependencies/COMPREHENSIVE_ANALYSIS', 'd46'),
                exact: true
              },
              {
                path: '/audits/dependencies/QUICK_REFERENCE',
                component: ComponentCreator('/audits/dependencies/QUICK_REFERENCE', '6e8'),
                exact: true
              },
              {
                path: '/audits/MASTER_REPORT',
                component: ComponentCreator('/audits/MASTER_REPORT', '227'),
                exact: true
              },
              {
                path: '/audits/security/AUTH_REPORT',
                component: ComponentCreator('/audits/security/AUTH_REPORT', 'bee'),
                exact: true
              },
              {
                path: '/audits/security/COMPREHENSIVE_AUDIT',
                component: ComponentCreator('/audits/security/COMPREHENSIVE_AUDIT', '68a'),
                exact: true,
                sidebar: "docsSidebar"
              },
              {
                path: '/audits/security/EXECUTIVE_SUMMARY',
                component: ComponentCreator('/audits/security/EXECUTIVE_SUMMARY', '383'),
                exact: true,
                sidebar: "docsSidebar"
              },
              {
                path: '/audits/security/QUICK_REFERENCE',
                component: ComponentCreator('/audits/security/QUICK_REFERENCE', 'ffe'),
                exact: true,
                sidebar: "docsSidebar"
              },
              {
                path: '/audits/security/SECRET_MANAGEMENT',
                component: ComponentCreator('/audits/security/SECRET_MANAGEMENT', 'b37'),
                exact: true
              },
              {
                path: '/audits/technical-debt/',
                component: ComponentCreator('/audits/technical-debt/', '8cc'),
                exact: true
              },
              {
                path: '/audits/technical-debt/ACTION_CHECKLIST',
                component: ComponentCreator('/audits/technical-debt/ACTION_CHECKLIST', '57d'),
                exact: true
              },
              {
                path: '/audits/technical-debt/EXECUTIVE_SUMMARY',
                component: ComponentCreator('/audits/technical-debt/EXECUTIVE_SUMMARY', 'eb1'),
                exact: true
              },
              {
                path: '/audits/technical-debt/SYNTHESIS_REPORT',
                component: ComponentCreator('/audits/technical-debt/SYNTHESIS_REPORT', '760'),
                exact: true
              },
              {
                path: '/audits/testing/ASSESSMENT_SUMMARY',
                component: ComponentCreator('/audits/testing/ASSESSMENT_SUMMARY', '174'),
                exact: true
              },
              {
                path: '/audits/testing/COVERAGE_STATUS',
                component: ComponentCreator('/audits/testing/COVERAGE_STATUS', '091'),
                exact: true
              },
              {
                path: '/audits/testing/IMPROVEMENT_ROADMAP',
                component: ComponentCreator('/audits/testing/IMPROVEMENT_ROADMAP', '5fc'),
                exact: true
              },
              {
                path: '/audits/testing/INFRASTRUCTURE_ASSESSMENT',
                component: ComponentCreator('/audits/testing/INFRASTRUCTURE_ASSESSMENT', 'b0b'),
                exact: true
              },
              {
                path: '/audits/ui/COMPREHENSIVE_AUDIT',
                component: ComponentCreator('/audits/ui/COMPREHENSIVE_AUDIT', 'da4'),
                exact: true,
                sidebar: "docsSidebar"
              },
              {
                path: '/audits/ui/EXECUTIVE_SUMMARY',
                component: ComponentCreator('/audits/ui/EXECUTIVE_SUMMARY', 'ad3'),
                exact: true,
                sidebar: "docsSidebar"
              },
              {
                path: '/audits/ui/SWITCHYARD_UI_AUDIT',
                component: ComponentCreator('/audits/ui/SWITCHYARD_UI_AUDIT', '51c'),
                exact: true
              },
              {
                path: '/audits/ui/SWITCHYARD_UI_SUMMARY',
                component: ComponentCreator('/audits/ui/SWITCHYARD_UI_SUMMARY', '9c0'),
                exact: true
              },
              {
                path: '/DNS_SETUP_PORKBUN',
                component: ComponentCreator('/DNS_SETUP_PORKBUN', '19c'),
                exact: true
              },
              {
                path: '/getting-started/BUILD_SETUP',
                component: ComponentCreator('/getting-started/BUILD_SETUP', '539'),
                exact: true,
                sidebar: "docsSidebar"
              },
              {
                path: '/getting-started/DEVELOPMENT',
                component: ComponentCreator('/getting-started/DEVELOPMENT', '462'),
                exact: true,
                sidebar: "docsSidebar"
              },
              {
                path: '/getting-started/QUICKSTART',
                component: ComponentCreator('/getting-started/QUICKSTART', 'a1c'),
                exact: true,
                sidebar: "docsSidebar"
              },
              {
                path: '/guides/AUDIT_LOGGING_TEST_GUIDE',
                component: ComponentCreator('/guides/AUDIT_LOGGING_TEST_GUIDE', 'e22'),
                exact: true
              },
              {
                path: '/guides/DOGFOODING_GUIDE',
                component: ComponentCreator('/guides/DOGFOODING_GUIDE', '9c8'),
                exact: true,
                sidebar: "docsSidebar"
              },
              {
                path: '/guides/RAILWAY_MIGRATION_GUIDE',
                component: ComponentCreator('/guides/RAILWAY_MIGRATION_GUIDE', '21b'),
                exact: true,
                sidebar: "docsSidebar"
              },
              {
                path: '/guides/TESTING_GUIDE',
                component: ComponentCreator('/guides/TESTING_GUIDE', '50c'),
                exact: true,
                sidebar: "docsSidebar"
              },
              {
                path: '/guides/VERCEL_MIGRATION_GUIDE',
                component: ComponentCreator('/guides/VERCEL_MIGRATION_GUIDE', '143'),
                exact: true,
                sidebar: "docsSidebar"
              },
              {
                path: '/implementation/BLUE_OCEAN_IMPLEMENTATION_STATUS',
                component: ComponentCreator('/implementation/BLUE_OCEAN_IMPLEMENTATION_STATUS', '7a3'),
                exact: true
              },
              {
                path: '/implementation/BOOTSTRAP_AUTH_STRATEGY',
                component: ComponentCreator('/implementation/BOOTSTRAP_AUTH_STRATEGY', 'eb2'),
                exact: true
              },
              {
                path: '/implementation/BUILD_PIPELINE_IMPLEMENTATION',
                component: ComponentCreator('/implementation/BUILD_PIPELINE_IMPLEMENTATION', 'f03'),
                exact: true,
                sidebar: "docsSidebar"
              },
              {
                path: '/implementation/CLI_IMPLEMENTATION_COMPLETE',
                component: ComponentCreator('/implementation/CLI_IMPLEMENTATION_COMPLETE', '4b4'),
                exact: true,
                sidebar: "docsSidebar"
              },
              {
                path: '/implementation/IMMEDIATE_PRIORITIES_IMPLEMENTATION',
                component: ComponentCreator('/implementation/IMMEDIATE_PRIORITIES_IMPLEMENTATION', '7ea'),
                exact: true
              },
              {
                path: '/implementation/MAIN_INTEGRATION_COMPLETE',
                component: ComponentCreator('/implementation/MAIN_INTEGRATION_COMPLETE', '8a2'),
                exact: true
              },
              {
                path: '/implementation/MVP_IMPLEMENTATION',
                component: ComponentCreator('/implementation/MVP_IMPLEMENTATION', 'c41'),
                exact: true,
                sidebar: "docsSidebar"
              },
              {
                path: '/NPM_REGISTRY_IMPLEMENTATION',
                component: ComponentCreator('/NPM_REGISTRY_IMPLEMENTATION', '837'),
                exact: true
              },
              {
                path: '/production/GAP_ANALYSIS',
                component: ComponentCreator('/production/GAP_ANALYSIS', 'b3f'),
                exact: true,
                sidebar: "docsSidebar"
              },
              {
                path: '/production/PRODUCTION_CHECKLIST',
                component: ComponentCreator('/production/PRODUCTION_CHECKLIST', 'e0d'),
                exact: true
              },
              {
                path: '/production/PRODUCTION_DEPLOYMENT_ROADMAP',
                component: ComponentCreator('/production/PRODUCTION_DEPLOYMENT_ROADMAP', '923'),
                exact: true,
                sidebar: "docsSidebar"
              },
              {
                path: '/production/PRODUCTION_READINESS_AUDIT',
                component: ComponentCreator('/production/PRODUCTION_READINESS_AUDIT', '355'),
                exact: true,
                sidebar: "docsSidebar"
              },
              {
                path: '/SSO_DEPLOYMENT_INSTRUCTIONS',
                component: ComponentCreator('/SSO_DEPLOYMENT_INSTRUCTIONS', 'b5e'),
                exact: true
              },
              {
                path: '/',
                component: ComponentCreator('/', 'd96'),
                exact: true,
                sidebar: "docsSidebar"
              }
            ]
          }
        ]
      }
    ]
  },
  {
    path: '*',
    component: ComponentCreator('*'),
  },
];
