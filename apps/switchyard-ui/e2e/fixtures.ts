import { test as base, expect } from '@playwright/test';

/**
 * Enclii E2E Test Fixtures
 *
 * Extended test fixtures with authentication helpers and common utilities.
 */

// Test user credentials from environment
const TEST_USER = {
  email: process.env.TEST_USER_EMAIL || 'test-dev@madfam.io',
  password: process.env.TEST_USER_PASSWORD || '',
};

// Extend base test with custom fixtures
export const test = base.extend<{
  // Authenticated page (logged in)
  authenticatedPage: typeof base;
}>({
  // Fixture that provides an authenticated session
  authenticatedPage: async ({ page }, use) => {
    // Check if we have test credentials
    if (!TEST_USER.password) {
      console.warn('TEST_USER_PASSWORD not set - skipping authentication');
      await use(base);
      return;
    }

    // Navigate to login
    await page.goto('/login');

    // Click SSO login button
    const ssoButton = page.getByRole('button', { name: /sign in with janua|continue with sso/i });
    if (await ssoButton.isVisible()) {
      await ssoButton.click();

      // Wait for redirect to Janua
      await page.waitForURL('**/auth.madfam.io/**', { timeout: 10000 });

      // Fill Janua credentials
      await page.fill('[name="email"], [type="email"]', TEST_USER.email);
      await page.fill('[name="password"], [type="password"]', TEST_USER.password);
      await page.getByRole('button', { name: /sign in|log in/i }).click();

      // Wait for redirect back to app
      await page.waitForURL('**/', { timeout: 15000 });
    }

    await use(base);
  },
});

// Re-export expect for convenience
export { expect };

/**
 * Test data-testid selectors for consistent element targeting
 */
export const testIds = {
  // Navigation
  desktopNav: 'desktop-nav',
  mobileNav: 'mobile-nav',
  hamburgerMenu: 'hamburger-menu',
  themeToggle: 'theme-toggle',
  userMenu: 'user-menu',

  // Dashboard
  statCard: 'stat-card',
  recentActivity: 'recent-activity',
  servicesTable: 'services-table',
  loadingSkeleton: 'loading-skeleton',

  // Auth
  loginButton: 'login-button',
  logoutButton: 'logout-button',
  userAvatar: 'user-avatar',
};

/**
 * Common viewport sizes for responsive testing
 */
export const viewports = {
  mobile: { width: 375, height: 812 },
  tablet: { width: 768, height: 1024 },
  desktop: { width: 1280, height: 800 },
  desktopLarge: { width: 1920, height: 1080 },
};

/**
 * Wait for page to be fully loaded (no network activity)
 */
export async function waitForPageLoad(page: typeof base extends { page: infer P } ? P : never) {
  await page.waitForLoadState('networkidle');
}

/**
 * Check for console errors during test
 */
export function setupConsoleErrorCapture(page: typeof base extends { page: infer P } ? P : never) {
  const errors: string[] = [];

  page.on('console', (msg) => {
    if (msg.type() === 'error') {
      errors.push(msg.text());
    }
  });

  return errors;
}
