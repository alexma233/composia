import { expect, test } from "@playwright/test";

import { expectHeading, login } from "./helpers";

test("dashboard loads through the real controller", async ({ page }) => {
  await login(page);

  await expect(page).toHaveURL("/");
  await expectHeading(page, "Dashboard");
  await expect(page.locator('[role="alert"]')).toHaveCount(0);

  const nav = page.getByRole("navigation", { name: "Main navigation" });
  const servicesCard = page.getByRole("link", { name: /Services/i }).first();
  const nodesCard = page.getByRole("link", { name: /Nodes/i }).first();

  await expect(nav.getByRole("link", { name: "Services" })).toBeVisible();
  await expect(nav.getByRole("link", { name: "Nodes" })).toBeVisible();
  await expect(servicesCard).toHaveAttribute("href", "/services");
  await expect(nodesCard).toHaveAttribute("href", "/nodes");
});
