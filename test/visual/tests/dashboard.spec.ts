import { test, expect } from "@playwright/test";
import { loginAsRoot } from "./helpers";

test.describe("发布数据大盘", () => {
  test.beforeEach(async ({ page }) => {
    await loginAsRoot(page);
    await expect(page).toHaveTitle("express233 · 发布控制台");
  });

  test("上传事件进入按日汇总、趋势图和最近记录", async ({ page }) => {
    const projectName = `dashboard-visual-${Date.now()}`;
    await page.getByTestId("new-project-input").fill(projectName);
    await page.getByTestId("add-project").click();
    await page.getByTestId("project-list").getByText(projectName, { exact: true }).click();
    await page.getByTestId("new-version-input").fill("1.0.0");
    await page.getByTestId("add-version").click();
    await page.getByTestId("version-list").getByText("1.0.0").click();

    await page.evaluate(async (name) => {
      const projects = await fetch("/api/projects").then((response) => response.json());
      const project = projects.find((item) => item.name === name);
      const body = new FormData();
      body.append("file", new Blob(["dashboard-payload"], { type: "application/octet-stream" }), "game.bin");
      const response = await fetch(`/api/projects/${project.id}/versions/1.0.0/files`, { method: "POST", body });
      if (!response.ok) throw new Error(await response.text());
    }, projectName);

    await page.getByTestId("nav-dashboard").click();
    await expect(page.getByTestId("dashboard-panel")).toBeVisible();
    await page.getByTestId("dashboard-project-filter").selectOption({ label: projectName });
    await expect(page.getByTestId("dashboard-kpis")).toContainText("上传请求");
    await expect(page.getByTestId("dashboard-kpis")).toContainText("17 B");
    await expect(page.getByTestId("dashboard-chart").locator("svg")).toBeVisible();
    await expect(page.getByTestId("dashboard-quality-chart").locator("svg")).toBeVisible();
    await expect(page.getByTestId("dashboard-health")).toContainText("当前交付态势");
    await expect(page.getByTestId("dashboard-daily-table")).toContainText(new Date().toISOString().slice(0, 10));
    await expect(page.getByTestId("dashboard-records-table")).toContainText(projectName);
    await expect(page.getByTestId("dashboard-records-table")).toContainText("game.bin");
  });

  test("移动端保持筛选、指标和明细可访问", async ({ page }) => {
    await page.setViewportSize({ width: 390, height: 844 });
    await page.getByTestId("nav-dashboard").click();
    await expect(page.getByTestId("dashboard-panel")).toBeVisible();
    await expect(page.getByTestId("dashboard-days-filter")).toBeVisible();
    await expect(page.getByTestId("dashboard-kpis")).toBeVisible();
    await expect(page.getByTestId("dashboard-health")).toBeVisible();
    await expect(page.getByTestId("dashboard-daily-table")).toBeVisible();
  });
});
