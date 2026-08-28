import { expect, test } from "@playwright/test";
import path from "path";

test("SSH 资源与可重复发布任务分离，执行日志只读保留", async ({ page }) => {
  await page.goto("/");
  await page.getByTestId("login-submit").click();
  await expect(page.getByTestId("app-shell")).toBeVisible();

  const projectName = `release-task-${Date.now()}`;
  const hostName = `release-node-${Date.now()}`;
  await page.getByTestId("new-project-input").fill(projectName);
  await page.getByTestId("add-project").click();
  await page.getByTestId("project-list").getByText(projectName, { exact: true }).click();
  await page.getByTestId("new-version-input").fill("1.0.0");
  await page.getByTestId("add-version").click();
  await page.getByTestId("version-list").getByText("1.0.0").click();
  await page.getByTestId("file-input").setInputFiles(path.join(__dirname, "../../../testdata/validation-tree/version/deploy/game.properties"));
  await page.getByTestId("validate-version").click();
  await expect(page.getByTestId("validate-result")).toContainText("可以发布", { timeout: 10_000 });
  await page.getByTestId("publish-version").click();
  await page.locator(".modal-card").getByRole("button", { name: "发布" }).click();
  await expect(page.getByTestId("ver-status")).toContainText("published", { timeout: 15_000 });

  await page.evaluate(async ({ hostName }) => {
    const hostResponse = await fetch("/api/push/hosts", { method: "POST", headers: { "Content-Type": "application/json" }, body: JSON.stringify({ name: hostName, address: "127.0.0.1", port: 1, username: "root", auth_mode: "agent", host_key: "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA", health_check_enabled: true, health_check_interval_seconds: 3600 }) });
    if (!hostResponse.ok) throw new Error(await hostResponse.text());
    const host = await hostResponse.json();
    const bindingResponse = await fetch(`/api/push/hosts/${host.id}/servers`, { method: "POST", headers: { "Content-Type": "application/json" }, body: JSON.stringify({ server_id: "visual-s1", labels: "test", remote_root: "/opt/game-servers", os: "linux", arch: "amd64" }) });
    if (!bindingResponse.ok) throw new Error(await bindingResponse.text());
  }, { hostName });

  await page.getByTestId("nav-release").click();
  await page.getByTestId("release-tab-ssh").click();
  await expect(page.getByTestId("ssh-resource-list")).toContainText(hostName);
  await page.getByTestId("project-list").getByText(projectName, { exact: true }).click();
  await page.getByRole("button", { name: "发布任务" }).click();
  await expect(page.getByTestId("release-task-editor")).toBeVisible();
  await expect(page.locator("#ptab-push")).not.toContainText("SSH 主机与服务器绑定");

  await page.locator("#pushTaskName").fill("逻辑服重复发布");
  await page.locator("#pushTaskServerIDs").fill("visual-s1");
  await page.getByRole("button", { name: "保存任务" }).click();
  const taskRow = page.getByTestId("release-task-list").getByRole("row").filter({ hasText: "逻辑服重复发布" });
  await expect(taskRow).toContainText("0");
  await taskRow.getByRole("button", { name: "预演" }).click();
  await expect(page.locator("#ptab-pushlogs")).toBeVisible();
  await expect(page.locator("#ptab-pushlogs")).toContainText("不可删除 · 保留 30 天");
  await expect(page.getByTestId("app-shell")).toContainText("逻辑服重复发布");

  await page.getByTestId("release-tab-tasks").click();
  let repeatedRow = page.getByTestId("release-task-list").getByRole("row").filter({ hasText: "逻辑服重复发布" });
  await expect(repeatedRow).toContainText("1");

  await page.getByTestId("release-tab-hooks").click();
  await expect(page.getByTestId("release-hook-editor")).toBeVisible();
  await page.locator("#releaseHookName").fill("虚构逻辑服自动发布");
  await page.locator("#releaseHookTask").selectOption({ label: "逻辑服重复发布" });
  await page.locator("#releaseHookDebounce").fill("1");
  await page.getByRole("button", { name: "保存 Hook" }).click();
  let hookRow = page.getByTestId("release-hook-list").getByRole("row").filter({ hasText: "虚构逻辑服自动发布" });
  await expect(hookRow).toContainText("已启用");
  await hookRow.getByRole("button", { name: "立即触发" }).click();
  hookRow = page.getByTestId("release-hook-list").getByRole("row").filter({ hasText: "虚构逻辑服自动发布" });
  await hookRow.getByRole("button", { name: "立即触发" }).click();
  await expect(page.getByTestId("release-hook-events")).toContainText("合并触发");
  await expect(page.getByTestId("release-hook-events")).toContainText("最终派发", { timeout: 8_000 });
  hookRow = page.getByTestId("release-hook-list").getByRole("row").filter({ hasText: "虚构逻辑服自动发布" });
  await hookRow.locator("label.switch").click();
  await expect(hookRow).toContainText("已停用");
  await hookRow.getByRole("button", { name: "删除" }).click();
  await page.locator(".modal-card").getByRole("button", { name: "确认" }).click();
  await expect(page.getByTestId("release-hook-list")).not.toContainText("虚构逻辑服自动发布");

  await page.getByTestId("release-tab-tasks").click();
  repeatedRow = page.getByTestId("release-task-list").getByRole("row").filter({ hasText: "逻辑服重复发布" });
  await repeatedRow.getByRole("button", { name: "删除" }).click();
  await page.locator(".modal-card").getByRole("button", { name: "确认" }).click();
  await expect(page.getByTestId("release-task-list")).not.toContainText("逻辑服重复发布");
  await page.getByTestId("release-tab-logs").click();
  await expect(page.locator("#ptab-pushlogs")).toContainText("逻辑服重复发布");
});
