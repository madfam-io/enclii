import { test, expect } from '@playwright/test';

/**
 * Enclii API Health Tests
 *
 * These tests validate that the Switchyard API is accessible and responding.
 * A failure here indicates a critical production issue (502/500 errors).
 *
 * BLOCKING: These tests block deployment if they fail.
 */

const API_BASE_URL = process.env.API_BASE_URL || 'https://api.enclii.dev';
const APP_BASE_URL = process.env.APP_BASE_URL || 'https://app.enclii.dev';

test.describe('API Health Checks', () => {
  test('api.enclii.dev/health returns 200', async ({ request }) => {
    const response = await request.get(`${API_BASE_URL}/health`);

    // This is the critical test - if this fails, production is broken
    expect(response.status()).toBe(200);

    const body = await response.json();
    expect(body).toHaveProperty('status');
  });

  test('api.enclii.dev/health/ready returns 200', async ({ request }) => {
    const response = await request.get(`${API_BASE_URL}/health/ready`);

    expect(response.status()).toBe(200);
  });

  test('api.enclii.dev/health/live returns 200', async ({ request }) => {
    const response = await request.get(`${API_BASE_URL}/health/live`);

    expect(response.status()).toBe(200);
  });

  test('api.enclii.dev returns non-502 status', async ({ request }) => {
    const response = await request.get(`${API_BASE_URL}/`);

    // The API root might return 404 or redirect, but NOT 502
    expect(response.status()).not.toBe(502);
    expect(response.status()).not.toBe(503);
  });
});

test.describe('UI Health Checks', () => {
  test('app.enclii.dev loads successfully', async ({ page }) => {
    const response = await page.goto(APP_BASE_URL);

    // Should not be a server error
    expect(response?.status()).toBeLessThan(500);
  });

  test('app.enclii.dev has proper title', async ({ page }) => {
    await page.goto(APP_BASE_URL);

    // Wait for page to be interactive
    await page.waitForLoadState('domcontentloaded');

    // Should have some content (not blank)
    const title = await page.title();
    expect(title.length).toBeGreaterThan(0);
  });
});

test.describe('API Endpoint Validation', () => {
  test('v1/auth/login endpoint is reachable (not 502)', async ({ request }) => {
    const response = await request.get(`${API_BASE_URL}/v1/auth/login`, {
      maxRedirects: 0, // Don't follow redirects
      failOnStatusCode: false,
    });

    // Should redirect or return auth-related status, NOT 502
    expect(response.status()).not.toBe(502);
    expect(response.status()).not.toBe(503);
  });

  test('OpenAPI spec is accessible', async ({ request }) => {
    const response = await request.get(`${API_BASE_URL}/swagger/doc.json`, {
      failOnStatusCode: false,
    });

    // OpenAPI spec should be available (or return proper 404, not 502)
    expect(response.status()).not.toBe(502);
    expect(response.status()).not.toBe(503);
  });
});
