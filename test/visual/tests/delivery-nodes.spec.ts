import { expect, test } from "@playwright/test";
import { loginAsRoot } from "./helpers";

test("Push/Pull 统一节点清单展示心跳、拓扑与自动跟随策略", async ({ page }) => {
  await loginAsRoot(page);

  const originalYAML = await page.evaluate(async () => (await fetch("/api/server-yaml")).json().then((x) => x.content));

  const suffix = Date.now();
  const projectName = `delivery-cluster-${suffix}`;
  const serverID = `logic-visual-${suffix}`;
  await page.evaluate(async ({ projectName, serverID }) => {
    const yamlResponse = await fetch("/api/server-yaml", {
      method: "PUT", headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ content: `servers:\n  ${serverID}:\n    replacements: {}\n` }),
    });
    if (!yamlResponse.ok) throw new Error(await yamlResponse.text());
    const projectResponse = await fetch("/api/projects", {
      method: "POST", headers: { "Content-Type": "application/json" }, body: JSON.stringify({ name: projectName }),
    });
    if (!projectResponse.ok) throw new Error(await projectResponse.text());
    const heartbeatResponse = await fetch("/api/agent/nodes/heartbeat", {
      method: "POST", headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ project: projectName, server_id: serverID, role: "logic", environment: "visual", labels: ["role:logic", "env:visual"], os: "linux", arch: "amd64", status: "ok", heartbeat_interval_seconds: 30 }),
    });
    if (!heartbeatResponse.ok) throw new Error(await heartbeatResponse.text());
  }, { projectName, serverID });

  await page.reload();
  await page.getByTestId("project-list").getByText(projectName, { exact: true }).click();
  await page.getByRole("button", { name: "集群节点" }).click();
  await expect(page.getByTestId("cluster-panel")).toBeVisible();
  await expect(page.locator("#clusterPullCount")).toHaveText("1");
  const row = page.getByTestId("delivery-node-list").getByRole("row").filter({ hasText: serverID });
  await expect(row).toContainText("logic");
  await expect(row).toContainText("visual");
  await expect(row).toContainText("在线");
  await expect(row).toContainText("generation 0/0");
  const autoFollow = row.getByRole("checkbox", { name: "自动跟随新版本" });
  await autoFollow.check();
  await expect(autoFollow).toBeChecked();

  await page.evaluate(async (content) => {
    const response = await fetch("/api/server-yaml", {
      method: "PUT",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ content }),
    });
    if (!response.ok) throw new Error(await response.text());
  }, originalYAML);
});
