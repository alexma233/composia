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

test("mobile navigation keeps every destination reachable", async ({ page }) => {
  await page.setViewportSize({ width: 390, height: 844 });
  await page.reload();
  await expect(page.locator("[data-app-ready]")).toHaveCount(1);

  await page.getByRole("button", { name: "Main navigation" }).click();
  const settingsLink = page
    .locator("nav:visible")
    .getByRole("link", { name: "Settings" });

  await expect(settingsLink).toBeVisible();
  await settingsLink.click();

  await expect(page).toHaveURL("/settings");
  await expectHeading(page, "Settings");
  await expect
    .poll(() =>
      page.evaluate(
        () => document.documentElement.scrollWidth <= document.documentElement.clientWidth,
      ),
    )
    .toBe(true);
});

test("interactive controls hydrate without nested targets", async ({ page }) => {
  const hydrationWarnings: string[] = [];
  page.on("console", (message) => {
    if (message.text().includes("hydration_mismatch")) {
      hydrationWarnings.push(message.text());
    }
  });

  await page.goto("/nodes/main");
  await expect(
    page.getByRole("button", {
      name: "docker image prune -a — Choose prune command",
    }),
  ).toBeVisible();
  await expect(
    page.getByRole("button", {
      name: "docker system prune -a --volumes — Choose prune command",
    }),
  ).toBeVisible();
  await expect(
    page.locator(
      "button button, a button, button a, [role='button'] button, [role='button'] a",
    ),
  ).toHaveCount(0);

  await page.goto("/nodes/main/docker/containers");
  await expect(
    page.locator(
      "button button, a button, button a, [role='button'] button, [role='button'] a",
    ),
  ).toHaveCount(0);
  expect(hydrationWarnings).toEqual([]);
});

test("service editor has an accessible name", async ({ page }) => {
  await page.goto("/services/host-service");
  await expect(page.getByRole("textbox", { name: "Code editor" })).toBeVisible();
});
