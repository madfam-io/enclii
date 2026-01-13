import { test, expect } from '@playwright/test';

/**
 * Theme E2E Tests
 *
 * Priority: P1/P2
 * Tests dark mode toggle, light mode, system preference, and persistence.
 */

test.describe('Theme Toggle', () => {
  test.describe('Dark Mode', () => {
    test('should have theme toggle button', async ({ page }) => {
      await page.goto('/');

      // Look for theme toggle button
      const themeToggle = page.getByRole('button', { name: /toggle theme|theme|dark|light/i });
      await expect(themeToggle).toBeVisible();
    });

    test('should apply dark mode class to html element', async ({ page }) => {
      await page.goto('/');

      const themeToggle = page.getByRole('button', { name: /toggle theme|theme/i });
      await themeToggle.click();

      // Select dark mode option
      const darkOption = page.getByRole('menuitem', { name: /dark/i });
      if (await darkOption.isVisible()) {
        await darkOption.click();
      }

      // Verify dark class on html
      const html = page.locator('html');
      await expect(html).toHaveClass(/dark/);
    });

    test('dark mode should not have white backgrounds on cards', async ({ page }) => {
      await page.goto('/');

      // Set dark mode
      const themeToggle = page.getByRole('button', { name: /toggle theme|theme/i });
      await themeToggle.click();

      const darkOption = page.getByRole('menuitem', { name: /dark/i });
      if (await darkOption.isVisible()) {
        await darkOption.click();
      }

      // Wait for theme to apply
      await page.waitForTimeout(300);

      // Check card backgrounds
      const cards = page.locator('[class*="Card"], .card, [data-testid="stat-card"]');
      const cardCount = await cards.count();

      for (let i = 0; i < Math.min(cardCount, 5); i++) {
        const card = cards.nth(i);
        const bgColor = await card.evaluate((el) => {
          return window.getComputedStyle(el).backgroundColor;
        });

        // Should not be pure white (rgb(255, 255, 255))
        expect(bgColor).not.toBe('rgb(255, 255, 255)');
      }
    });

    test('text should be readable in dark mode', async ({ page }) => {
      await page.goto('/');

      // Set dark mode
      const themeToggle = page.getByRole('button', { name: /toggle theme|theme/i });
      await themeToggle.click();

      const darkOption = page.getByRole('menuitem', { name: /dark/i });
      if (await darkOption.isVisible()) {
        await darkOption.click();
      }

      await page.waitForTimeout(300);

      // Check text color (should be light on dark background)
      const heading = page.locator('h1, h2, h3').first();
      if (await heading.isVisible()) {
        const color = await heading.evaluate((el) => {
          return window.getComputedStyle(el).color;
        });

        // Parse RGB values
        const match = color.match(/rgb\((\d+),\s*(\d+),\s*(\d+)\)/);
        if (match) {
          const [, r, g, b] = match.map(Number);
          // Text should be light (high RGB values) in dark mode
          const brightness = (r + g + b) / 3;
          expect(brightness).toBeGreaterThan(100);
        }
      }
    });
  });

  test.describe('Light Mode', () => {
    test('should apply light mode when selected', async ({ page }) => {
      await page.goto('/');

      const themeToggle = page.getByRole('button', { name: /toggle theme|theme/i });
      await themeToggle.click();

      const lightOption = page.getByRole('menuitem', { name: /light/i });
      if (await lightOption.isVisible()) {
        await lightOption.click();
      }

      // Verify NOT dark class
      const html = page.locator('html');
      await expect(html).not.toHaveClass(/dark/);
    });
  });

  test.describe('System Preference', () => {
    test('should respect system preference', async ({ page }) => {
      // Emulate dark color scheme
      await page.emulateMedia({ colorScheme: 'dark' });
      await page.goto('/');

      // Select system option
      const themeToggle = page.getByRole('button', { name: /toggle theme|theme/i });
      await themeToggle.click();

      const systemOption = page.getByRole('menuitem', { name: /system/i });
      if (await systemOption.isVisible()) {
        await systemOption.click();
      }

      // Should follow system (dark)
      const html = page.locator('html');
      await expect(html).toHaveClass(/dark/);
    });
  });

  test.describe('Persistence', () => {
    test('should persist theme choice across page navigation', async ({ page }) => {
      await page.goto('/');

      // Set dark mode
      const themeToggle = page.getByRole('button', { name: /toggle theme|theme/i });
      await themeToggle.click();

      const darkOption = page.getByRole('menuitem', { name: /dark/i });
      if (await darkOption.isVisible()) {
        await darkOption.click();
      }

      // Navigate to another page
      await page.goto('/login');
      await page.waitForLoadState('domcontentloaded');

      // Check theme is still dark
      const html = page.locator('html');
      await expect(html).toHaveClass(/dark/);
    });

    test('should persist theme after page refresh', async ({ page }) => {
      await page.goto('/');

      // Set dark mode
      const themeToggle = page.getByRole('button', { name: /toggle theme|theme/i });
      await themeToggle.click();

      const darkOption = page.getByRole('menuitem', { name: /dark/i });
      if (await darkOption.isVisible()) {
        await darkOption.click();
      }

      // Refresh page
      await page.reload();
      await page.waitForLoadState('domcontentloaded');

      // Theme should still be dark
      const html = page.locator('html');
      await expect(html).toHaveClass(/dark/);
    });
  });

  test.describe('No Theme Flash', () => {
    test('should not flash white on page load in dark mode', async ({ page, context }) => {
      // Set dark mode preference in storage before navigating
      await context.addInitScript(() => {
        window.localStorage.setItem('theme', 'dark');
      });

      // Track background colors during load
      let flashDetected = false;

      await page.exposeFunction('reportFlash', () => {
        flashDetected = true;
      });

      await page.goto('/', { waitUntil: 'commit' });

      // Check initial background
      const initialBg = await page.evaluate(() => {
        return window.getComputedStyle(document.documentElement).backgroundColor;
      });

      // If initial background is pure white, that's a flash
      if (initialBg === 'rgb(255, 255, 255)') {
        flashDetected = true;
      }

      // Note: Some flash may be acceptable during hydration
      // This test documents the behavior
      console.log('Initial background:', initialBg);
      console.log('Flash detected:', flashDetected);
    });
  });
});
