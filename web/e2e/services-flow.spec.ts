import { expect, test, type Page } from "@playwright/test";

import { expectHeading, login } from "./helpers";

test.describe.configure({ mode: "serial" });

async function createService(page: Page, folder: string) {
  await login(page, "/services");

  await expectHeading(page, "Services");
  await page.getByRole("button", { name: "Create service" }).click();
  await page.getByLabel("Folder name").fill(folder);
  await page.getByRole("button", { name: "Create", exact: true }).click();

  await expect(page).toHaveURL(`/services/${folder}`);
}

async function replaceEditorContent(page: Page, content: string) {
  const editor = page.getByRole("region", { name: "Code editor" });

  await expect(editor).toBeVisible();
  await editor.locator(".cm-content").first().click();
  await page.keyboard.press("Control+A");
  await page.keyboard.insertText(content);
}

async function openServiceFile(page: Page, fileName: string) {
  await page.getByRole("button", { name: fileName }).first().click();
}

async function expectEditorToContain(page: Page, text: string) {
  await expect(
    page
      .getByRole("region", { name: "Code editor" })
      .locator(".cm-content")
      .first(),
  ).toContainText(text);
}

async function openNewestServiceTask(page: Page, name: RegExp) {
  const taskLink = page.getByRole("link", { name }).first();

  await expect(taskLink).toBeVisible();
  await taskLink.click();
  await expect(page).toHaveURL(/\/tasks\/[0-9a-f-]+$/);
}

async function declareConfigInfraService(page: Page, folder: string) {
  await replaceEditorContent(
    page,
    `name: ${folder}\nnodes:\n  - main\ninfra:\n  config: {}\n`,
  );

  const saveButton = page.getByRole("button", { name: "Save" });

  await expect(saveButton).toBeEnabled();
  await saveButton.click();
  await expect(page.getByText("Saved composia-meta.yaml")).toBeVisible();

  await page.reload();

  await expect(page.getByRole("button", { name: "Deploy (Up)" })).toBeEnabled();
  await expect(page.getByRole("button", { name: "Stop (Down)" })).toBeEnabled();
}

async function expectSucceededTaskDetails(page: Page, heading: string) {
  await expectHeading(page, heading);
  await expect(
    page.getByText("Succeeded", { exact: true }).first(),
  ).toBeVisible({ timeout: 30_000 });
  await expect(
    page.getByText("Completed", { exact: true }).first(),
  ).toBeVisible({ timeout: 30_000 });
}

test("can create a new service from the services page", async ({ page }) => {
  const folder = `e2e-created-${Date.now()}`;

  await createService(page, folder);

  await expect(page.getByText(folder, { exact: true }).first()).toBeVisible();
  await expect(
    page.getByRole("button", { name: "composia-meta.yaml" }).first(),
  ).toBeVisible();
  await expect(
    page.getByRole("button", { name: "docker-compose.yaml" }).first(),
  ).toBeVisible();
});

test("service workspace edits persist after reload", async ({ page }) => {
  const folder = `e2e-edit-${Date.now()}`;
  const composeComment = `# saved by ${folder}`;

  await createService(page, folder);
  await openServiceFile(page, "docker-compose.yaml");
  await replaceEditorContent(page, `${composeComment}\n`);

  const saveButton = page.getByRole("button", { name: "Save" });

  await expect(saveButton).toBeEnabled();
  await saveButton.click();
  await expect(page.getByText("Saved docker-compose.yaml")).toBeVisible();

  await page.reload();
  await openServiceFile(page, "docker-compose.yaml");
  await expectEditorToContain(page, composeComment);
});

test("undeclared services cannot run service actions yet", async ({ page }) => {
  const folder = `e2e-undeclared-${Date.now()}`;

  await createService(page, folder);

  await expect(
    page.getByRole("button", { name: "Deploy (Up)" }),
  ).toBeDisabled();
  await expect(
    page.getByRole("button", { name: "Stop (Down)" }),
  ).toBeDisabled();
});

test("newly created service can be declared by editing composia-meta.yaml", async ({
  page,
}) => {
  const folder = `e2e-declare-${Date.now()}`;

  await createService(page, folder);
  await declareConfigInfraService(page, folder);
});

test("newly created declared service can run a real deploy task", async ({
  page,
}) => {
  const folder = `e2e-deploy-${Date.now()}`;

  await createService(page, folder);
  await declareConfigInfraService(page, folder);

  await page.getByRole("button", { name: "Deploy (Up)" }).click();

  await openNewestServiceTask(page, /Deploy \(Up\)/);
  await expectSucceededTaskDetails(page, "Deploy (Up)");
});

test("preseeded service can run a real deploy task", async ({ page }) => {
  await login(page, "/services/host-service");

  await expect(page).toHaveURL("/services/host-service");
  await page.getByRole("button", { name: "Deploy (Up)" }).click();

  await openNewestServiceTask(page, /Deploy \(Up\)/);
  await expectSucceededTaskDetails(page, "Deploy (Up)");
});

test("preseeded service can run a real stop task", async ({ page }) => {
  await login(page, "/services/host-service");

  await expect(page).toHaveURL("/services/host-service");
  await expect(
    page.getByText("host-service", { exact: true }).first(),
  ).toBeVisible();
  await page.getByRole("button", { name: "Stop (Down)" }).click();

  await openNewestServiceTask(page, /Stop \(Down\)/);
  await expectSucceededTaskDetails(page, "Stop (Down)");
});

test("preseeded infra.config service shows a restart error", async ({
  page,
}) => {
  await login(page, "/services/host-service");

  await expect(page).toHaveURL("/services/host-service");
  await page.getByRole("button", { name: "Restart (Down + Up)" }).click();

  await expect(page.getByRole("alert")).toContainText(
    'service "host-service" declares infra.config and cannot be restarted',
  );
});

test("service-triggered tasks are visible in the tasks list", async ({
  page,
}) => {
  await login(page, "/services/host-service");

  await page.getByRole("button", { name: "Deploy (Up)" }).click();
  await expect(
    page.getByRole("link", { name: /Deploy \(Up\)/ }).first(),
  ).toBeVisible();

  await page.goto("/tasks?serviceName=host-service");

  await expectHeading(page, "Task history");
  await expect(
    page.getByRole("link", { name: /Deploy \(Up\) for host-service/ }).first(),
  ).toBeVisible({ timeout: 30_000 });
});

test("succeeded service tasks can be rerun from task details", async ({
  page,
}) => {
  await login(page, "/services/host-service");

  await page.getByRole("button", { name: "Deploy (Up)" }).click();
  await openNewestServiceTask(page, /Deploy \(Up\)/);
  await expectSucceededTaskDetails(page, "Deploy (Up)");

  const originalUrl = page.url();

  await page.getByRole("button", { name: "Run Again" }).click();

  await expect(page).not.toHaveURL(originalUrl);
  await expect(page).toHaveURL(/\/tasks\/[0-9a-f-]+$/);
  await expectSucceededTaskDetails(page, "Deploy (Up)");
});
