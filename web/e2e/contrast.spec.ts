import AxeBuilder from "@axe-core/playwright";
import { expect, test } from "@playwright/test";

import { login } from "./helpers";

const themes = ["light", "dark"] as const;
const accents = ["blue", "emerald", "violet", "rose", "amber"] as const;
const routes = [
  "/",
  "/services/host-service",
  "/nodes/main",
  "/nodes/main/docker/containers",
  "/tasks",
  "/backups",
  "/settings",
];

async function addSemanticContrastProbes(
  page: import("@playwright/test").Page,
) {
  await page.evaluate(() => {
    const probe = document.createElement("div");
    probe.dataset.contrastProbe = "";
    probe.className = "fixed top-0 left-0 z-50 flex gap-2 bg-background p-2";
    probe.innerHTML = `
      <span class="bg-success text-success-foreground">Success</span>
      <span class="bg-warning text-warning-foreground">Warning</span>
      <span class="bg-info text-info-foreground">Info</span>
      <span class="bg-destructive-subtle text-destructive-subtle-foreground">Destructive</span>
      <a href="#" class="bg-primary text-primary-foreground hover:bg-primary-hover">Primary</a>
    `;
    document.body.append(probe);
  });
  await page.locator("[data-contrast-probe] a").hover();
}

async function expectOpaqueFocusRing(page: import("@playwright/test").Page) {
  const button = page.getByRole("button", { name: "Refresh", exact: true });
  await button.focus();
  const colors = await button.evaluate((element) => ({
    ring: getComputedStyle(element).getPropertyValue("--tw-ring-color").trim(),
    token: getComputedStyle(document.documentElement)
      .getPropertyValue("--ring")
      .trim(),
  }));
  expect.soft(colors.ring).toBe(colors.token);
}

async function waitForHydratedPage(page: import("@playwright/test").Page) {
  await expect(page.locator("[data-app-ready]")).toHaveCount(1);
  await page.evaluate(
    () =>
      new Promise<void>((resolve) => {
        requestAnimationFrame(() => requestAnimationFrame(() => resolve()));
      }),
  );
}

for (const theme of themes) {
  for (const accent of accents) {
    test(`${theme}/${accent} meets color contrast requirements`, async ({
      page,
    }) => {
      await page.addInitScript(
        ([nextTheme, nextAccent]) => {
          localStorage.setItem("composia.theme-mode", nextTheme);
          localStorage.setItem("composia.accent-color", nextAccent);
        },
        [theme, accent],
      );
      await login(page);

      for (const route of routes) {
        await page.goto(route);
        await waitForHydratedPage(page);
        if (route === "/") {
          await addSemanticContrastProbes(page);
        }
        if (route.endsWith("/docker/containers")) {
          await expectOpaqueFocusRing(page);
        }
        const { violations } = await new AxeBuilder({ page })
          .withRules(["color-contrast"])
          .analyze();

        expect
          .soft(
            violations.flatMap((violation) =>
              violation.nodes.map((node) => ({
                rule: violation.id,
                target: node.target,
              })),
            ),
            `${theme}/${accent} ${route}`,
          )
          .toEqual([]);
      }
    });
  }
}
