import { expect, test } from "@playwright/test";

test("dashboard loads through the real controller", async ({ page }) => {
  await page.goto("/login");

  await page.locator('input[name="username"]').fill("admin");
  await page.locator('input[name="password"]').fill("admin");
  await page.locator('button[type="submit"]').click();

  await expect(page).toHaveURL("/");
  await expect(page.locator('[role="alert"]')).toHaveCount(0);

  const summaryCards = page.locator(".grid.gap-4.sm\\:grid-cols-2");

  await expect(summaryCards.locator('a[href="/services"]')).toBeVisible();
  await expect(summaryCards.locator('a[href="/nodes"]')).toBeVisible();
});
