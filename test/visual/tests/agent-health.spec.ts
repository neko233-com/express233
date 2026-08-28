import { expect, test } from "@playwright/test";
import { loginAsRoot } from "./helpers";

test.describe("Agent 与 SSH 节点", () => {
  test.beforeEach(async ({ page }) => {
    await loginAsRoot(page);
  });

  test("能力清单、定时间隔、单次检查和历史均可操作", async ({ page }) => {
    const name = `agent-node-${Date.now()}`;
    await page.evaluate(async (hostName) => {
      const response = await fetch("/api/push/hosts", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({
          name: hostName, address: "127.0.0.1", port: 1, username: "root", auth_mode: "agent",
          host_key: "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA",
          health_check_enabled: true, health_check_interval_seconds: 3600,
        }),
      });
      if (!response.ok) throw new Error(await response.text());
    }, name);

    await page.getByTestId("nav-release").click();
    await page.getByTestId("release-tab-ssh").click();
    await expect(page.getByTestId("agent-panel")).toBeVisible();
    await expect(page.getByTestId("agent-summary")).toContainText("Agent API 操作");
    await expect(page.getByTestId("agent-capabilities")).toContainText("/api/push/hosts/{hostID}/check");
    const row = page.getByTestId("agent-host-list").getByRole("row").filter({ hasText: name });
    await expect(row).toContainText("1 小时/次");
    await row.getByRole("button", { name: "立即检查" }).click();
    await expect(page.getByTestId("agent-check-history")).toBeVisible();
    await expect(page.getByTestId("agent-check-history")).toContainText("失败");
    await expect(page.getByTestId("agent-check-history")).toContainText("手动");
  });

  test("移动端仍能访问节点与 API 清单", async ({ page }) => {
    await page.setViewportSize({ width: 390, height: 844 });
    await page.getByTestId("nav-release").click();
    await page.getByTestId("release-tab-ssh").click();
    await expect(page.getByTestId("agent-summary")).toBeVisible();
    await expect(page.getByTestId("agent-host-list")).toBeVisible();
    await expect(page.getByTestId("agent-capabilities")).toBeVisible();
  });
});
