import { expect, test } from "@playwright/test";

import { expectHeading, login } from "./helpers";

test("task filters update and reset the URL", async ({ page }) => {
  await login(page, "/tasks");

  await expectHeading(page, "Task history");

  await page.getByRole("button", { name: /Filter/ }).click();
  await page.getByRole("button", { name: "All statuses" }).click();

  const searchInput = page.getByPlaceholder("Search statuses...");
  await searchInput.fill("Running");
  await searchInput.press("ArrowDown");
  await searchInput.press("Enter");

  await page.getByRole("button", { name: "Apply" }).click();

  await expect(page).toHaveURL("/tasks?status=running");

  await page.getByRole("button", { name: /Filter/ }).click();
  await page.getByRole("button", { name: "Reset" }).click();

  await expect(page).toHaveURL("/tasks");
});
