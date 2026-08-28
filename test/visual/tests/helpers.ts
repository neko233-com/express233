import { expect, type Page } from "@playwright/test";

export async function loginAsRoot(page: Page) {
  await page.goto("/");
  await expect(page.getByTestId("login-panel")).toBeVisible();
  await page.locator("#user").fill("root");
  await page.locator("#pass").fill("root");
  await page.getByTestId("login-submit").click();
  await expect(page.getByTestId("app-shell")).toBeVisible();
  await expect(page.getByTestId("whoami")).toContainText("root");
}
