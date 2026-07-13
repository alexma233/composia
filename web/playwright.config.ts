import { defineConfig } from "@playwright/test";

const host = "127.0.0.1";
const port = 4173;
const baseURL = process.env.PLAYWRIGHT_BASE_URL ?? `http://${host}:${port}`;

export default defineConfig({
  testDir: "./e2e",
  reporter: process.env.CI ? "list" : "html",
  use: {
    baseURL,
    launchOptions: {
      executablePath: process.env.PLAYWRIGHT_CHROMIUM_EXECUTABLE_PATH,
    },
    trace: "on-first-retry",
  },
  webServer: {
    command: `deno task build && deno task preview -- --host ${host} --port ${port}`,
    reuseExistingServer: !process.env.CI,
    timeout: 600_000,
    url: baseURL,
  },
});
