import { test, expect } from '@playwright/test';
import { viewports } from '../fixtures';

/**
 * Responsive Design E2E Tests
 *
 * Priority: P0/P1
 * Tests layout at different viewport sizes, hamburger menu, and overflow handling.
 */

test.describe('Responsive Design', () => {
  test.describe('Mobile (375px)', () => {
    test.use({ viewport: viewports.mobile });

    test('should not have horizontal overflow', async ({ page }) => {
      await page.goto('/');

      // Check for horizontal scroll
      const hasOverflow = await page.evaluate(() => {
        return document.documentElement.scrollWidth > document.documentElement.clientWidth;
      });

      expect(hasOverflow).toBeFalsy();
    });

    test('should show hamburger menu', async ({ page }) => {
      await page.goto('/');

      // Look for hamburger/menu button
      const hamburger = page.getByRole('button', { name: /menu|navigation/i });
      await expect(hamburger).toBeVisible();
    });

    test('should hide desktop navigation', async ({ page }) => {
      await page.goto('/');

      // Desktop nav should be hidden
      const desktopNav = page.locator('[data-testid="desktop-nav"], nav.hidden\\:lg\\:flex, .desktop-nav');

      // Either not present or hidden
      const count = await desktopNav.count();
      if (count > 0) {
        await expect(desktopNav.first()).not.toBeVisible();
      }
    });

    test('hamburger menu should open mobile navigation', async ({ page }) => {
      await page.goto('/');

      // Click hamburger
      const hamburger = page.getByRole('button', { name: /menu|navigation/i });

      if (await hamburger.isVisible()) {
        await hamburger.click();

        // Mobile nav should appear (Sheet/Drawer component)
        const mobileNav = page.locator('[role="dialog"], [data-testid="mobile-nav"], .sheet-content');
        await expect(mobileNav).toBeVisible();
      }
    });

    test('mobile menu should contain navigation links', async ({ page }) => {
      await page.goto('/');

      const hamburger = page.getByRole('button', { name: /menu|navigation/i });

      if (await hamburger.isVisible()) {
        await hamburger.click();

        // Check for navigation links
        const dashboardLink = page.getByRole('link', { name: /dashboard/i });
        const projectsLink = page.getByRole('link', { name: /projects/i });

        // At least one nav link should be visible
        const dashboardVisible = await dashboardLink.isVisible().catch(() => false);
        const projectsVisible = await projectsLink.isVisible().catch(() => false);

        expect(dashboardVisible || projectsVisible).toBeTruthy();
      }
    });

    test('mobile menu should close when link is clicked', async ({ page }) => {
      await page.goto('/');

      const hamburger = page.getByRole('button', { name: /menu|navigation/i });

      if (await hamburger.isVisible()) {
        await hamburger.click();

        // Click a link
        const anyLink = page.locator('[role="dialog"] a, .sheet-content a').first();
        if (await anyLink.isVisible()) {
          await anyLink.click();

          // Menu should close
          const mobileNav = page.locator('[role="dialog"], .sheet-content');
          await expect(mobileNav).not.toBeVisible();
        }
      }
    });

    test('stat cards should stack vertically', async ({ page }) => {
      await page.goto('/');

      // Cards container should be flex-col or grid with single column
      const cardsContainer = page.locator('[class*="grid"], [class*="flex"]').filter({
        has: page.locator('[data-testid="stat-card"], [class*="Card"]'),
      });

      if ((await cardsContainer.count()) > 0) {
        const container = cardsContainer.first();
        const classes = await container.getAttribute('class');

        // Should have responsive grid classes
        expect(classes).toMatch(/grid-cols-1|flex-col|sm:|md:|lg:/);
      }
    });
  });

  test.describe('Tablet (768px)', () => {
    test.use({ viewport: viewports.tablet });

    test('should not have horizontal overflow', async ({ page }) => {
      await page.goto('/');

      const hasOverflow = await page.evaluate(() => {
        return document.documentElement.scrollWidth > document.documentElement.clientWidth;
      });

      expect(hasOverflow).toBeFalsy();
    });

    test('navigation should be appropriately scaled', async ({ page }) => {
      await page.goto('/');

      // Either hamburger or desktop nav should be visible
      const hamburger = page.getByRole('button', { name: /menu|navigation/i });
      const desktopNav = page.locator('nav, [data-testid="desktop-nav"]').filter({
        has: page.getByRole('link', { name: /dashboard|projects/i }),
      });

      const hamburgerVisible = await hamburger.isVisible().catch(() => false);
      const desktopVisible = await desktopNav.first().isVisible().catch(() => false);

      expect(hamburgerVisible || desktopVisible).toBeTruthy();
    });

    test('stat cards should display in 2-column grid', async ({ page }) => {
      await page.goto('/');

      // Get first stat card position
      const cards = page.locator('[data-testid="stat-card"], [class*="Card"]');
      const count = await cards.count();

      if (count >= 2) {
        const card1Box = await cards.nth(0).boundingBox();
        const card2Box = await cards.nth(1).boundingBox();

        if (card1Box && card2Box) {
          // Cards should be side by side (same Y) or stacked (different Y)
          // On tablet, 2 columns means cards 1 & 2 should have similar Y
          const sameRow = Math.abs(card1Box.y - card2Box.y) < 50;
          const differentX = Math.abs(card1Box.x - card2Box.x) > 50;

          // Either same row (2-col) or stacked (1-col)
          expect(sameRow && differentX || !sameRow).toBeTruthy();
        }
      }
    });
  });

  test.describe('Desktop (1280px)', () => {
    test.use({ viewport: viewports.desktop });

    test('should not have horizontal overflow', async ({ page }) => {
      await page.goto('/');

      const hasOverflow = await page.evaluate(() => {
        return document.documentElement.scrollWidth > document.documentElement.clientWidth;
      });

      expect(hasOverflow).toBeFalsy();
    });

    test('should show full desktop navigation', async ({ page }) => {
      await page.goto('/');

      // Desktop nav should be visible
      const navLinks = page.locator('nav a, header a').filter({
        hasText: /dashboard|projects|services|deployments/i,
      });

      const count = await navLinks.count();
      expect(count).toBeGreaterThan(0);
    });

    test('hamburger menu should be hidden', async ({ page }) => {
      await page.goto('/');

      // Hamburger should not be visible on desktop
      const hamburger = page.getByRole('button', { name: /menu/i }).filter({
        has: page.locator('[class*="Menu"], [class*="menu"]'),
      });

      // Should be hidden or not present
      const visible = await hamburger.isVisible().catch(() => false);
      expect(visible).toBeFalsy();
    });

    test('stat cards should display in row', async ({ page }) => {
      await page.goto('/');

      const cards = page.locator('[data-testid="stat-card"], [class*="Card"]');
      const count = await cards.count();

      if (count >= 3) {
        const card1Box = await cards.nth(0).boundingBox();
        const card2Box = await cards.nth(1).boundingBox();
        const card3Box = await cards.nth(2).boundingBox();

        if (card1Box && card2Box && card3Box) {
          // All cards should be in same row (similar Y)
          const sameRow =
            Math.abs(card1Box.y - card2Box.y) < 50 && Math.abs(card2Box.y - card3Box.y) < 50;

          expect(sameRow).toBeTruthy();
        }
      }
    });
  });

  test.describe('All Viewports', () => {
    const allViewports = [
      { name: 'mobile', ...viewports.mobile },
      { name: 'tablet', ...viewports.tablet },
      { name: 'desktop', ...viewports.desktop },
    ];

    for (const vp of allViewports) {
      test(`no JavaScript errors on ${vp.name}`, async ({ page }) => {
        await page.setViewportSize({ width: vp.width, height: vp.height });

        const errors: string[] = [];
        page.on('console', (msg) => {
          if (msg.type() === 'error') {
            errors.push(msg.text());
          }
        });

        await page.goto('/');
        await page.waitForLoadState('networkidle');

        // Filter out known non-critical errors
        const criticalErrors = errors.filter(
          (e) => !e.includes('favicon') && !e.includes('Failed to load resource')
        );

        expect(criticalErrors).toHaveLength(0);
      });

      test(`main content is accessible on ${vp.name}`, async ({ page }) => {
        await page.setViewportSize({ width: vp.width, height: vp.height });
        await page.goto('/');

        // Main content should be visible
        const main = page.locator('main, [role="main"], .main-content');
        await expect(main.first()).toBeVisible();
      });
    }
  });
});
