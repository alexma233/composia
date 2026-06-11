import { expect, test } from "@playwright/test";

test("dashboard loads through the real controller", async ({ page }) => {
  await page.goto("/login");

  await page.locator('input[name="username"]').fill("admin");
  await page.locator('input[name="password"]').fill("admin");
  await page.locator('button[type="submit"]').click();

  await expect(page).toHaveURL("/");
  await expect(page.locator('[role="alert"]')).toHaveCount(0);
  await expect(page.locator('a[href="/services"]')).toBeVisible();
  await expect(page.locator('a[href="/nodes"]')).toBeVisible();
});
