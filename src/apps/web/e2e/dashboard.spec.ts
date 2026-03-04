import { test, expect } from '@playwright/test';

test.describe('Dashboard Functionality', () => {
    test('should load the dashboard page and display KPIs', async ({ page }) => {
        // Navigate to the app root, which should redirect to /dashboard
        await page.goto('http://localhost:3000/');

        // Verify redirection to /dashboard
        await expect(page).toHaveURL(/.*\/dashboard/);

        // Verify the Dashboard title is visible
        const title = page.locator('h1', { hasText: 'Dashboard' });
        await expect(title).toBeVisible();

        // Verify that KPI cards are loaded
        await expect(page.locator('text=Total GMV')).toBeVisible();
        await expect(page.locator('text=Avg Margin')).toBeVisible();
        await expect(page.locator('text=Units Sold')).toBeVisible();
        await expect(page.locator('text=Active SKUs')).toBeVisible();
    });

    test('should open and close the chat panel', async ({ page }) => {
        await page.goto('http://localhost:3000/dashboard');

        // the chat toggle button
        const toggleButton = page.locator('button', { hasText: '💬' });
        await expect(toggleButton).toBeVisible();

        // Click to open chat
        await toggleButton.click();

        // Verify chat header is visible
        await expect(page.locator('text=AI Copilot')).toBeVisible();

        // Close the chat
        await toggleButton.click();

        // Ensure chat input is not visible (or the panel is hidden)
        const chatInput = page.locator('input[placeholder*="Ask about"]');
        await expect(chatInput).toBeHidden();
    });

    test('sidebar navigation renders correctly', async ({ page }) => {
        await page.goto('http://localhost:3000/');

        const sidebar = page.locator('aside');
        await expect(sidebar).toBeVisible();

        // Verify links exist
        await expect(page.locator('text=Reports')).toBeVisible();
        await expect(page.locator('text=News')).toBeVisible();
        await expect(page.locator('text=Config')).toBeVisible();
    });
});
