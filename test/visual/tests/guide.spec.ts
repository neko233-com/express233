import { expect, test } from "@playwright/test";
import { loginAsRoot } from "./helpers";

test("文档目录展示 Gitea/GitHub/Agent 官方接入说明且不泄露运行信息", async ({ page }) => {
  await loginAsRoot(page);
  await page.getByTestId("nav-guide").click();
  await expect(page.getByTestId("guide-panel")).toBeVisible();
  await expect(page.getByTestId("guide-topic-list")).toContainText("Gitea Actions 接入");
  await expect(page.getByTestId("guide-topic-list")).toContainText("GitHub Actions 接入");
  await page.getByTestId("guide-topic-list").getByRole("button", { name: "Gitea Actions 接入" }).click();
  await expect(page.getByTestId("guide-content")).toContainText("Gitea Actions");
  await expect(page.getByTestId("guide-content")).not.toContainText("root/root");
});
