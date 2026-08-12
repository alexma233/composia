import AxeBuilder from "@axe-core/playwright";
import { expect, test, type Page } from "@playwright/test";

import { login } from "./helpers";

const routes = [
  "/",
  "/services",
  "/services/host-service",
  "/nodes",
  "/nodes/main",
  "/nodes/main/docker/containers",
  "/nodes/main/docker/networks",
  "/nodes/main/docker/volumes",
  "/nodes/main/docker/images",
  "/tasks",
  "/backups",
  "/settings",
];

const detailSources = [
  ["/tasks", "/tasks/"],
  ["/backups", "/backups/"],
  ["/nodes/main/docker/containers", "/nodes/main/docker/containers/"],
  ["/nodes/main/docker/networks", "/nodes/main/docker/networks/"],
  ["/nodes/main/docker/volumes", "/nodes/main/docker/volumes/"],
  ["/nodes/main/docker/images", "/nodes/main/docker/images/"],
] as const;

async function expectAccessible(page: Page, context: string) {
  await expect(page.locator("[data-app-ready]")).toHaveCount(1);
  const { violations } = await new AxeBuilder({ page })
    .withTags([
      "wcag2a",
      "wcag2aa",
      "wcag21a",
      "wcag21aa",
      "wcag22aa",
      "best-practice",
    ])
    .analyze();

  expect
    .soft(
      violations.flatMap((violation) =>
        violation.nodes.map((node) => ({
          rule: violation.id,
          target: node.target,
          summary: node.failureSummary,
        })),
      ),
      context,
    )
    .toEqual([]);
}

async function expectPageStatesAccessible(page: Page, context: string) {
  await expectAccessible(page, context);

  const tabs = page.getByRole("tab");
  for (let index = 0; index < (await tabs.count()); index += 1) {
    const tab = tabs.nth(index);
    const name = (await tab.textContent())?.trim() || String(index + 1);
    await tab.click();
    await expectAccessible(page, `${context} — ${name}`);
  }
}

test("public login is accessible", async ({ page }) => {
  await page.goto("/login");
  await expectAccessible(page, "/login");
});

test("authenticated route templates are accessible", async ({ page }) => {
  test.setTimeout(180_000);
  await login(page);

  for (const route of routes) {
    await page.goto(route);
    await expectPageStatesAccessible(page, route);
  }

  for (const [source, prefix] of detailSources) {
    await page.goto(source);
    const link = page.locator(`a[href^="${prefix}"]`).first();
    const href = (await link.count()) ? await link.getAttribute("href") : null;
    if (href) {
      await page.goto(href);
      await expectPageStatesAccessible(page, href);
    }
  }
});

test("navigation, preferences, and overlays expose their state", async ({
  page,
}) => {
  await login(page);

  await expect(
    page
      .getByRole("navigation", { name: "Main navigation" })
      .getByRole("link", { name: "Dashboard" }),
  ).toHaveAttribute("aria-current", "page");

  await page.keyboard.press("Tab");
  const skipLink = page.getByRole("link", { name: "Skip to content" });
  await expect(skipLink).toBeFocused();
  await skipLink.press("Enter");
  await expect(page.locator("main#main-content")).toBeFocused();

  await page.goto("/settings");
  await expect(page.getByRole("button", { name: "System" })).toHaveAttribute(
    "aria-pressed",
    "true",
  );
  await expect(page.getByRole("button", { name: "English" })).toHaveAttribute(
    "aria-pressed",
    "true",
  );

  await page.setViewportSize({ width: 390, height: 844 });
  await page.goto("/");
  await page.getByRole("button", { name: "Main navigation" }).click();
  await expectAccessible(page, "mobile navigation");

  await page.setViewportSize({ width: 1280, height: 900 });
  await page.goto("/services");
  await page.getByRole("button", { name: "Create service" }).click();
  await expectAccessible(page, "create service popover");
  await page.keyboard.press("Escape");

  await page.goto("/tasks");
  await page.getByRole("button", { name: /Filter/ }).click();
  await expectAccessible(page, "task filters");

  await page.goto("/nodes/main");
  await page.getByRole("button", { name: "docker container prune" }).click();
  await expectAccessible(page, "Docker prune dialog");
});
