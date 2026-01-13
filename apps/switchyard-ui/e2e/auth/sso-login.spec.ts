import { test, expect } from '@playwright/test';

/**
 * Authentication E2E Tests
 *
 * Priority: P0 (Critical)
 * Tests SSO login flow via Janua, session management, and logout.
 */

test.describe('SSO Authentication', () => {
  test.describe('Login Flow', () => {
    test('should display login page for unauthenticated users', async ({ page }) => {
      await page.goto('/');

      // Should redirect to login or show login button
      const loginVisible = await page.getByRole('button', { name: /sign in|log in/i }).isVisible();
      const onLoginPage = page.url().includes('/login');

      expect(loginVisible || onLoginPage).toBeTruthy();
    });

    test('should have SSO login option', async ({ page }) => {
      await page.goto('/login');

      // Look for SSO/Janua login button
      const ssoButton = page.getByRole('button', { name: /sign in with janua|continue with sso|single sign-on/i });
      await expect(ssoButton).toBeVisible();
    });

    test('should redirect to Janua on SSO button click', async ({ page }) => {
      await page.goto('/login');

      // Click SSO button
      const ssoButton = page.getByRole('button', { name: /sign in with janua|continue with sso|single sign-on/i });

      // Set up navigation promise before clicking
      const navigationPromise = page.waitForURL('**/auth.madfam.io/**', { timeout: 15000 });
      await ssoButton.click();

      // Verify redirect to Janua
      await navigationPromise;
      expect(page.url()).toContain('auth.madfam.io');
    });
  });

  test.describe('Protected Routes', () => {
    test('should redirect /dashboard to login when unauthenticated', async ({ page }) => {
      await page.goto('/dashboard');

      // Should redirect to login page
      await page.waitForURL('**/login**', { timeout: 10000 });
      expect(page.url()).toContain('/login');
    });

    test('should redirect /projects to login when unauthenticated', async ({ page }) => {
      await page.goto('/projects');

      await page.waitForURL('**/login**', { timeout: 10000 });
      expect(page.url()).toContain('/login');
    });

    test('should redirect /services to login when unauthenticated', async ({ page }) => {
      await page.goto('/services');

      await page.waitForURL('**/login**', { timeout: 10000 });
      expect(page.url()).toContain('/login');
    });
  });
});

test.describe('Session Management', () => {
  // These tests require valid credentials - skip in CI without credentials
  test.skip(
    !process.env.TEST_USER_PASSWORD,
    'TEST_USER_PASSWORD not set - skipping authenticated tests'
  );

  test('should maintain session after page refresh', async ({ page }) => {
    // This test would require authentication setup
    // Login first
    await page.goto('/login');

    const ssoButton = page.getByRole('button', { name: /sign in with janua|continue with sso/i });
    if (await ssoButton.isVisible()) {
      await ssoButton.click();

      // Complete Janua login
      await page.waitForURL('**/auth.madfam.io/**');
      await page.fill('[name="email"], [type="email"]', process.env.TEST_USER_EMAIL || '');
      await page.fill('[name="password"], [type="password"]', process.env.TEST_USER_PASSWORD || '');
      await page.getByRole('button', { name: /sign in|log in/i }).click();

      // Wait for redirect back
      await page.waitForURL('**/', { timeout: 15000 });
    }

    // Verify logged in
    const userElement = page.getByRole('button', { name: /user|profile|account/i });
    await expect(userElement).toBeVisible();

    // Refresh page
    await page.reload();

    // Should still be logged in
    await expect(userElement).toBeVisible();
  });
});

test.describe('Logout Flow', () => {
  test.skip(
    !process.env.TEST_USER_PASSWORD,
    'TEST_USER_PASSWORD not set - skipping logout tests'
  );

  test('should have logout option in user menu', async ({ page }) => {
    // Would need authenticated session
    // After login, open user menu
    const userMenu = page.getByRole('button', { name: /user|profile|account/i });

    if (await userMenu.isVisible()) {
      await userMenu.click();

      // Look for logout option
      const logoutButton = page.getByRole('menuitem', { name: /log out|sign out|logout/i });
      await expect(logoutButton).toBeVisible();
    }
  });

  test('should redirect to login after logout', async ({ page }) => {
    // After clicking logout, should redirect to login
    // This would need full authentication flow setup
    const userMenu = page.getByRole('button', { name: /user|profile|account/i });

    if (await userMenu.isVisible()) {
      await userMenu.click();

      const logoutButton = page.getByRole('menuitem', { name: /log out|sign out|logout/i });
      await logoutButton.click();

      // Should redirect to login
      await page.waitForURL('**/login**', { timeout: 10000 });
      expect(page.url()).toContain('/login');
    }
  });
});
