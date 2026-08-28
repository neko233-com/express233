import { expect, test } from "@playwright/test";
import { loginAsRoot } from "./helpers";

test("项目版本页可发现并保存已发布版本保留策略", async ({ page }) => {
  await loginAsRoot(page);

  const projectName = `retention-visual-${Date.now()}`;
  await page.getByTestId("new-project-input").fill(projectName);
  await page.getByTestId("add-project").click();
  await page.getByTestId("project-list").getByText(projectName, { exact: true }).click();

  const card = page.getByTestId("version-retention-card");
  await expect(card).toBeVisible();
  await expect(card).toContainText("版本保留策略");
  await card.locator("#versionRetentionLimited").check();
  await card.locator("#versionRetentionCount").fill("3");
  await page.getByTestId("save-version-retention").click();
  await expect(card.locator("#versionRetentionBadge")).toHaveText("最近 3 个");

  const project = await page.evaluate(async (name) => {
    const projects = await fetch("/api/projects").then((response) => response.json());
    return projects.find((item) => item.name === name);
  }, projectName);
  expect(project.max_published_versions).toBe(3);
});
