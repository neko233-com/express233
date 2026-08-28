import { expect, test } from "@playwright/test";
import path from "path";
import { loginAsRoot } from "./helpers";

async function createVersionWorkspace(page, prefix: string) {
  const projectName = `${prefix}-${Date.now()}`;
  await page.getByTestId("new-project-input").fill(projectName);
  await page.getByTestId("add-project").click();
  await page.getByTestId("project-list").getByText(projectName, { exact: true }).click();
  await page.getByTestId("new-version-input").fill("0.0.1");
  await page.getByTestId("add-version").click();
  await expect(page.getByTestId("version-detail")).toBeVisible();
}

test("2048 桌面版形成版本、详情、策略三栏控制台", async ({ page }) => {
  await page.setViewportSize({ width: 2048, height: 1152 });
  await loginAsRoot(page);
  await createVersionWorkspace(page, "layout-desktop");

  await expect(page.getByTestId("version-list")).toBeVisible();
  await expect(page.getByTestId("version-detail")).toBeVisible();
  await expect(page.getByTestId("version-retention-card")).toBeVisible();
  const areas = await page.locator(".split-versions").evaluate((element) => getComputedStyle(element).gridTemplateAreas);
  expect(areas).toContain("versions detail inspector");
  expect(await page.evaluate(() => document.documentElement.scrollWidth <= window.innerWidth)).toBeTruthy();

  if (process.env.EXPRESS233_CAPTURE_LAYOUT) {
    await page.locator(".main").evaluate((element) => { element.scrollTop = 0; });
    await page.screenshot({ path: path.join(__dirname, "../test-results/express233-desktop-2048.png") });
  }
});

test("390 移动端关键操作纵向可达且无横向溢出", async ({ page }) => {
  await page.setViewportSize({ width: 390, height: 844 });
  await loginAsRoot(page);
  await createVersionWorkspace(page, "layout-mobile");

  await expect(page.getByTestId("version-list")).toBeVisible();
  await expect(page.getByTestId("version-detail")).toBeVisible();
  await expect(page.getByTestId("version-retention-card")).toBeVisible();
  expect(await page.evaluate(() => document.documentElement.scrollWidth <= window.innerWidth)).toBeTruthy();

  if (process.env.EXPRESS233_CAPTURE_LAYOUT) {
    await page.getByTestId("version-retention-card").scrollIntoViewIfNeeded();
    await page.screenshot({ path: path.join(__dirname, "../test-results/express233-mobile-390.png") });
  }
});
