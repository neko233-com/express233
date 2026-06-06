import { test, expect } from "@playwright/test";
import path from "path";

test.describe("express233 控制台全流程", () => {
  test.beforeEach(async ({ page }) => {
    await page.goto("/");
    await expect(page.getByTestId("login-panel")).toBeVisible();
    await page.getByTestId("login-submit").click();
    await expect(page.getByTestId("app-shell")).toBeVisible();
    await expect(page.getByTestId("whoami")).toContainText("root");
  });

  test("登录 → 项目/版本 → 上传 → 预览 → 发布前检查 → 发布", async ({ page }) => {
    const projectName = `visual-${Date.now()}`;

    await page.getByTestId("new-project-input").fill(projectName);
    await page.getByTestId("add-project").click();
    await page.getByTestId("project-list").getByText(projectName, { exact: true }).click();
    await expect(page.getByTestId("cur-project")).toContainText(projectName);

    await page.getByTestId("new-version-input").fill("1.0.0");
    await page.getByTestId("add-version").click();
    await page.getByTestId("version-list").getByText("1.0.0").click();
    await expect(page.getByTestId("version-detail")).toBeVisible();
    await expect(page.getByTestId("ver-status")).toContainText("draft");

    const fixture = path.join(__dirname, "../../../testdata/validation-tree/version/deploy/game.properties");
    await page.getByTestId("file-input").setInputFiles(fixture);
    await expect(page.getByTestId("file-list")).toContainText("game.properties", { timeout: 15_000 });

    await page.getByRole("button", { name: "拉取预览" }).click();
    await page.getByTestId("preview-server-id").fill("visual-s1");
    await page.getByTestId("preview-submit").click();
    await expect(page.getByTestId("preview-table")).toContainText("game.properties", { timeout: 10_000 });
    await expect(page.getByTestId("preview-rendered-body")).toContainText("9001", { timeout: 10_000 });

    await page.getByRole("button", { name: "版本" }).click();
    await page.getByTestId("validate-version").click();
    const validate = page.getByTestId("validate-result");
    await expect(validate).toContainText("可以发布", { timeout: 10_000 });

    page.once("dialog", (d) => d.accept());
    await page.getByTestId("publish-version").click();
    await expect(page.getByTestId("version-list")).toContainText("published", { timeout: 15_000 });
    await expect(page.getByTestId("ver-status")).toContainText("published", { timeout: 5_000 });
    await expect(page.getByTestId("download-version")).toBeVisible();
    await expect(page.getByTestId("deploy-cmd")).toContainText("visual-s1");
  });

  test("版本搜索过滤列表", async ({ page }) => {
    const projectName = `version-search-${Date.now()}`;

    await page.getByTestId("new-project-input").fill(projectName);
    await page.getByTestId("add-project").click();
    await page.getByTestId("project-list").getByText(projectName, { exact: true }).click();

    await page.getByTestId("new-version-input").fill("1.0.0");
    await page.getByTestId("add-version").click();
    await page.getByTestId("new-version-input").fill("2.0.0");
    await page.getByTestId("add-version").click();

    await page.getByTestId("version-search").fill("2.0");
    await expect(page.getByTestId("version-list")).toContainText("2.0.0");
    await expect(page.getByTestId("version-list")).not.toContainText("1.0.0");
  });

  test("演示项目引导素材包含 JSON/YAML/properties 替换", async ({ page }) => {
    await page.locator("#btnDemoProject").click();
    await expect(page.getByTestId("cur-project")).toContainText("demo-game", { timeout: 20_000 });
    await expect(page.getByTestId("version-list")).toContainText("2.0.0", { timeout: 20_000 });
    await page.getByTestId("version-list").getByText("2.0.0").click();
    await expect(page.getByTestId("version-detail")).toBeVisible();
    await page.getByRole("button", { name: "拉取预览" }).click();
    await page.getByTestId("preview-server-id").fill("game-logic-01");
    await page.getByTestId("preview-submit").click();
    await expect(page.getByTestId("preview-table")).toContainText("game.properties", { timeout: 10_000 });
    await expect(page.getByTestId("preview-table")).toContainText("application.yaml");
    await expect(page.getByTestId("preview-table")).toContainText("settings.json");
    await expect(page.getByTestId("preview-rendered-body")).toContainText("game-logic-01");
  });

  test("新手引导可跳过并记录", async ({ page }) => {
    await page.locator("#btnStartOnboarding").click();
    await expect(page.locator(".driver-popover")).toBeVisible({ timeout: 10_000 });
    await page.locator(".driver-popover-close-btn").click();
    await expect
      .poll(() =>
        page.evaluate(() =>
          Object.keys(localStorage).some((k) => k.startsWith("express233_onboarding_v1_default_root"))
        )
      )
      .toBeTruthy();
  });

  test("server.yaml 页签保存与拉取预览", async ({ page }) => {
    await page.locator('.sidebar-nav-item[data-global="server"]').click();
    await expect(page.getByTestId("server-yaml-editor")).toBeVisible();
    const yaml = await page.getByTestId("server-yaml-editor").inputValue();
    expect(yaml).toContain("visual-s1");

    await page.locator('.sidebar-nav-item[data-global="workspace"]').click();
    const projectName = `yaml-tab-${Date.now()}`;
    await page.getByTestId("new-project-input").fill(projectName);
    await page.getByTestId("add-project").click();
    await page.getByTestId("project-list").getByText(projectName, { exact: true }).click();
    await page.getByTestId("new-version-input").fill("1.0.1");
    await page.getByTestId("add-version").click();
    await page.getByTestId("version-list").getByText("1.0.1").click();

    await page.getByRole("button", { name: "拉取预览" }).click();
    await page.getByTestId("preview-server-id").fill("visual-s1");
    await page.getByTestId("preview-submit").click();
    await expect(page.getByTestId("preview-rendered-body")).toBeVisible();
  });

  test("API 文档可打开", async ({ page, context }) => {
    const [doc] = await Promise.all([
      context.waitForEvent("page"),
      page.getByRole("link", { name: "API 文档" }).click(),
    ]);
    await doc.waitForLoadState();
    await expect(doc).toHaveURL(/\/docs\//);
    await doc.close();
  });
});
