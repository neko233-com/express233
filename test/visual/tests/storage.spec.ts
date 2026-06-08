import { test, expect } from "@playwright/test";
import path from "path";

test.describe("存储空间", () => {
  test.beforeEach(async ({ page }) => {
    await page.goto("/");
    await page.getByTestId("login-submit").click();
    await expect(page.getByTestId("app-shell")).toBeVisible();
  });

  test("总览、索引搜索与目录树", async ({ page }) => {
    const projectName = `storage-${Date.now()}`;
    await page.getByTestId("new-project-input").fill(projectName);
    await page.getByTestId("add-project").click();
    await page.getByTestId("project-list").getByText(projectName, { exact: true }).click();
    await page.getByTestId("new-version-input").fill("1.0.0");
    await page.getByTestId("add-version").click();
    await page.getByTestId("version-list").getByText("1.0.0").click();
    const fixture = path.join(__dirname, "../../../testdata/validation-tree/version/deploy/game.properties");
    await page.getByTestId("file-input").setInputFiles(fixture);
    await expect(page.getByTestId("file-list")).toContainText("game.properties", { timeout: 15_000 });

    await page.getByTestId("nav-storage").click();
    await expect(page.getByTestId("storage-panel")).toBeVisible();
    await expect(page.getByTestId("storage-stats")).toContainText("项目", { timeout: 15_000 });
    await expect(page.getByTestId("storage-bars")).toBeVisible();
    await expect(page.getByTestId("storage-tree")).toContainText(projectName, { timeout: 15_000 });

    await page.getByTestId("storage-reindex").click();
    await page.getByTestId("storage-search").fill("game.properties");
    await expect(page.getByTestId("storage-search-hits")).toContainText("game.properties", { timeout: 10_000 });
  });
});
