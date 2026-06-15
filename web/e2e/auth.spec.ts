import { expect, test } from "@playwright/test";

import { expectHeading, login, submitLoginForm } from "./helpers";

test("protected routes redirect through login and return to the original page", async ({
  page,
}) => {
  await page.goto("/tasks?status=running");

  await expect(page).toHaveURL(/\/login\?next=%2Ftasks%3Fstatus%3Drunning$/);

  await submitLoginForm(page);

  await expect(page).toHaveURL("/tasks?status=running");
  await expectHeading(page, "Task history");
});

test("invalid credentials show a login error", async ({ page }) => {
  await page.goto("/login");

  await submitLoginForm(page, { password: "not-the-right-password" });

  await expect(page).toHaveURL("/login");
  await expect(page.getByRole("alert")).toContainText(
    "Invalid username or password.",
  );
});

test("logged-in users are redirected away from the login page", async ({
  page,
}) => {
  await login(page);

  await page.goto("/login?next=%2Ftasks");

  await expect(page).toHaveURL("/tasks");
  await expectHeading(page, "Task history");
});

test("logging out clears the session", async ({ page }) => {
  await login(page);

  await page.getByRole("button", { name: "Log out" }).click();

  await expect(page).toHaveURL("/login");

  await page.goto("/settings");

  await expect(page).toHaveURL(/\/login\?next=%2Fsettings$/);
});
