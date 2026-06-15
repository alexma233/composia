import { expect, type Page } from "@playwright/test";

const defaultUsername = process.env.WEB_LOGIN_USERNAME ?? "admin";
const defaultPassword = process.env.WEB_LOGIN_PASSWORD ?? "admin";

type LoginOptions = {
  username?: string;
  password?: string;
};

export async function submitLoginForm(page: Page, options: LoginOptions = {}) {
  await page.getByLabel("Username").fill(options.username ?? defaultUsername);
  await page.getByLabel("Password").fill(options.password ?? defaultPassword);
  await page.getByRole("button", { name: "Sign in" }).click();
}

export async function login(page: Page, destination = "/") {
  await page.goto(destination);

  if (new URL(page.url()).pathname === "/login") {
    await submitLoginForm(page);
  }

  await expect(page).not.toHaveURL(/\/login(?:\?|$)/);
}

export async function expectHeading(page: Page, name: string) {
  await expect(page.getByRole("heading", { level: 1, name })).toBeVisible();
}
