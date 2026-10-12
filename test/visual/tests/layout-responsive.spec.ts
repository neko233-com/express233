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

test("2048 桌面以版本与文件两栏呈现，高级工具默认折叠", async ({ page }) => {
  await page.setViewportSize({ width: 2048, height: 1152 });
  await loginAsRoot(page);
  await createVersionWorkspace(page, "layout-desktop");

  await expect(page.getByTestId("version-list")).toBeVisible();
  await expect(page.getByTestId("version-detail")).toBeVisible();
  await expect(page.getByTestId("version-retention-card")).toBeVisible();
  await expect(page.getByTestId("workspace-next")).toBeInViewport();
  await expect(page.getByTestId("tag-batch-paths")).not.toBeVisible();
  await expect(page.locator("#versionRetentionLimited")).not.toBeVisible();
  const list = await page.locator(".version-sidebar").boundingBox();
  const detail = await page.getByTestId("version-detail").boundingBox();
  expect(list).not.toBeNull();
  expect(detail).not.toBeNull();
  expect(detail!.x).toBeGreaterThanOrEqual(list!.x + list!.width);
  expect(Math.abs(detail!.y - list!.y)).toBeLessThanOrEqual(1);
  const columns = await page.locator(".split-versions").evaluate((element) => getComputedStyle(element).gridTemplateColumns);
  expect(columns.split(" ")).toHaveLength(2);
  expect(await page.evaluate(() => document.documentElement.scrollWidth <= window.innerWidth)).toBeTruthy();

  if (process.env.EXPRESS233_CAPTURE_LAYOUT) {
    await expect(page.locator(".toast")).toHaveCount(0);
    await page.locator(".main").evaluate((element) => { element.scrollTop = 0; });
    await page.screenshot({ path: path.join(__dirname, "../test-results/express233-desktop-2048.png") });
  }
});

test("390 移动端下一步可完成上传并进入发布，无横向溢出", async ({ page }) => {
  await page.setViewportSize({ width: 390, height: 844 });
  await loginAsRoot(page);
  await createVersionWorkspace(page, "layout-mobile");

  await expect(page.getByTestId("version-list")).toBeVisible();
  await expect(page.getByTestId("version-detail")).toBeVisible();
  await expect(page.getByTestId("version-retention-card")).toBeVisible();
  await expect(page.locator("#versionRetentionLimited")).not.toBeVisible();
  expect(await page.evaluate(() => document.documentElement.scrollWidth <= window.innerWidth)).toBeTruthy();
  await page.getByTestId("workspace-next").scrollIntoViewIfNeeded();
  await expect(page.getByTestId("workspace-next")).toBeInViewport();

  if (process.env.EXPRESS233_CAPTURE_LAYOUT) {
    await expect(page.locator(".toast")).toHaveCount(0);
    await page.screenshot({ path: path.join(__dirname, "../test-results/express233-mobile-390.png") });
  }
  const chooseFile = page.waitForEvent("filechooser");
  await page.getByTestId("workspace-next").click();
  await (await chooseFile).setFiles(path.join(__dirname, "../../../testdata/validation-tree/version/deploy/game.properties"));
  await expect(page.getByTestId("workspace-next")).toHaveText("检查并发布");
  await page.getByTestId("workspace-next").click();
  await expect(page.getByTestId("publish-flow-panel")).toBeVisible();
  expect(await page.evaluate(() => document.documentElement.scrollWidth <= window.innerWidth)).toBeTruthy();
});
