import { FullConfig } from '@playwright/test';

/**
 * Global setup for Playwright E2E tests.
 *
 * This runs once before all tests to set up any required state.
 */
async function globalSetup(config: FullConfig): Promise<void> {
  console.log('E2E Global Setup - Starting');

  // Set environment variables for test mode
  process.env.E2E_TEST_MODE = 'true';

  // Log configuration
  console.log(`  Base URL: ${config.projects[0]?.use?.baseURL || 'not set'}`);
  console.log(`  Test user configured: ${!!process.env.TEST_USER_PASSWORD}`);
  console.log(`  CI mode: ${!!process.env.CI}`);

  console.log('E2E Global Setup - Complete');
}

export default globalSetup;
