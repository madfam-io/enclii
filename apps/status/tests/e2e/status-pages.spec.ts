import { test, expect } from '@playwright/test';

test.describe('Status Page Verification', () => {
  test.describe('status.enclii.dev', () => {
    test('should load the Enclii status page', async ({ page }) => {
      const response = await page.goto('https://status.enclii.dev', {
        waitUntil: 'networkidle',
        timeout: 30000,
      });

      // Check response status
      expect(response?.status()).toBeLessThan(500);

      // Take screenshot for verification
      await page.screenshot({ path: 'test-results/status-enclii.png', fullPage: true });

      // Log page title and content
      const title = await page.title();
      console.log('Enclii Status Page Title:', title);

      // Check for key elements
      const content = await page.content();
      console.log('Page loaded. Content length:', content.length);
    });

    test('should have API health endpoint', async ({ request }) => {
      const response = await request.get('https://status.enclii.dev/api/health');
      console.log('Health endpoint status:', response.status());

      if (response.ok()) {
        const data = await response.json();
        console.log('Health response:', JSON.stringify(data, null, 2));
      }
    });

    test('should have status API endpoint', async ({ request }) => {
      const response = await request.get('https://status.enclii.dev/api/status');
      console.log('Status API status:', response.status());

      if (response.ok()) {
        const data = await response.json();
        console.log('Status response:', JSON.stringify(data, null, 2));
      }
    });
  });

  test.describe('status.madfam.io', () => {
    test('should load the MADFAM status page', async ({ page }) => {
      const response = await page.goto('https://status.madfam.io', {
        waitUntil: 'networkidle',
        timeout: 30000,
      });

      // Check response status
      expect(response?.status()).toBeLessThan(500);

      // Take screenshot for verification
      await page.screenshot({ path: 'test-results/status-madfam.png', fullPage: true });

      // Log page title and content
      const title = await page.title();
      console.log('MADFAM Status Page Title:', title);

      // Check for key elements
      const content = await page.content();
      console.log('Page loaded. Content length:', content.length);
    });

    test('should have API health endpoint', async ({ request }) => {
      const response = await request.get('https://status.madfam.io/api/health');
      console.log('Health endpoint status:', response.status());

      if (response.ok()) {
        const data = await response.json();
        console.log('Health response:', JSON.stringify(data, null, 2));
      }
    });

    test('should have status API endpoint', async ({ request }) => {
      const response = await request.get('https://status.madfam.io/api/status');
      console.log('Status API status:', response.status());

      if (response.ok()) {
        const data = await response.json();
        console.log('Status response:', JSON.stringify(data, null, 2));
      }
    });
  });
});
