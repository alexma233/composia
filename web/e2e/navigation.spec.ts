import { expect, test } from "@playwright/test";

import { expectHeading, login } from "./helpers";

test.beforeEach(async ({ page }) => {
  await login(page);
});

test("top navigation opens the main console pages", async ({ page }) => {
  const nav = page.getByRole("navigation", { name: "Main navigation" });

  await expectHeading(page, "Dashboard");

  const pages = [
    { link: "Services", path: "/services", heading: "Services" },
    { link: "Nodes", path: "/nodes", heading: "Nodes" },
    { link: "Tasks", path: "/tasks", heading: "Task history" },
    { link: "Settings", path: "/settings", heading: "Settings" },
    { link: "Dashboard", path: "/", heading: "Dashboard" },
  ];

  for (const item of pages) {
    await nav.getByRole("link", { name: item.link }).click();
    await expect(page).toHaveURL(item.path);
    await expectHeading(page, item.heading);
  }

  await expect(page.getByRole("link", { name: "Dashboard" })).toBeVisible();
});

test("node details link to the docker containers page", async ({ page }) => {
  await page.goto("/nodes");

  await expectHeading(page, "Nodes");
  await page.getByRole("link", { name: "Main", exact: true }).click();

  await expect(page).toHaveURL("/nodes/main");
  await expectHeading(page, "Main");

  await page.getByRole("link", { name: "Containers" }).click();

  await expect(page).toHaveURL("/nodes/main/docker/containers");
  await expectHeading(page, "Containers");
  await expect(page.getByLabel("Search containers...")).toBeVisible();
});
