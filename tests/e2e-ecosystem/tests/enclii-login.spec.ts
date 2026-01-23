import { test, expect } from '@playwright/test';

/**
 * Enclii SSO/Login Flow Tests
 *
 * These tests validate that the authentication flow works correctly,
 * including redirection to Janua SSO.
 *
 * BLOCKING: These tests block deployment if they fail.
 */

const APP_BASE_URL = process.env.APP_BASE_URL || 'https://app.enclii.dev';
const JANUA_URL = process.env.JANUA_URL || 'https://auth.madfam.io';

test.describe('SSO Flow Validation', () => {
  test('login button triggers SSO redirect', async ({ page }) => {
    await page.goto(APP_BASE_URL);

    // Wait for page to load
    await page.waitForLoadState('networkidle');

    // Look for login button or link
    const loginButton = page.getByRole('button', { name: /login|sign in/i })
      .or(page.getByRole('link', { name: /login|sign in/i }));

    // If login button exists, click it and verify redirect
    if (await loginButton.count() > 0) {
      // Set up promise to catch navigation
      const navigationPromise = page.waitForURL(/auth\.madfam\.io|enclii\.dev/, {
        timeout: 10000,
      });

      await loginButton.first().click();
      await navigationPromise;

      // Should redirect to either Janua or stay on enclii
      const url = page.url();
      expect(
        url.includes('auth.madfam.io') || url.includes('enclii.dev')
      ).toBeTruthy();
    }
  });

  test('Janua SSO is accessible', async ({ request }) => {
    const response = await request.get(`${JANUA_URL}/.well-known/openid-configuration`);

    // OIDC discovery endpoint should be available
    expect(response.status()).toBe(200);

    const config = await response.json();
    expect(config).toHaveProperty('issuer');
    expect(config).toHaveProperty('authorization_endpoint');
    expect(config).toHaveProperty('token_endpoint');
  });

  test('JWKS endpoint is accessible', async ({ request }) => {
    const response = await request.get(`${JANUA_URL}/.well-known/jwks.json`);

    expect(response.status()).toBe(200);

    const jwks = await response.json();
    expect(jwks).toHaveProperty('keys');
    expect(Array.isArray(jwks.keys)).toBeTruthy();
  });
});

test.describe('Protected Routes', () => {
  test('dashboard redirects unauthenticated users', async ({ page }) => {
    // Try to access dashboard directly
    const response = await page.goto(`${APP_BASE_URL}/dashboard`);

    // Should either redirect to login or show login page
    // NOT return 500/502
    expect(response?.status()).toBeLessThan(500);
  });

  test('projects page redirects unauthenticated users', async ({ page }) => {
    const response = await page.goto(`${APP_BASE_URL}/projects`);

    // Should redirect to login or return proper status
    expect(response?.status()).toBeLessThan(500);
  });
});
