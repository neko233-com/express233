import { expect, test, type Page } from "@playwright/test";
import { loginAsRoot } from "./helpers";

async function createProject(page: Page, prefix: string) {
  const response = await page.request.post("/api/projects", { data: { name: `${prefix}-${Date.now()}` } });
  expect(response.ok()).toBeTruthy();
  return response.json();
}

async function createPublishedVersion(page: Page, project: { id: number }, version: string) {
  const base = `/api/projects/${project.id}/versions`;
  expect((await page.request.post(base, { data: { name: version } })).ok()).toBeTruthy();
  expect((await page.request.post(`${base}/${version}/files`, {
    multipart: { path: "game.bin", file: { name: "game.bin", mimeType: "application/octet-stream", buffer: Buffer.from("local deployment fixture") } },
  })).ok()).toBeTruthy();
  expect((await page.request.post(`${base}/${version}/publish`)).ok()).toBeTruthy();
}

async function openDelivery(page: Page, project: { name: string }) {
  await page.reload();
  await expect(page.getByTestId("app-shell")).toBeVisible();
  await page.getByTestId("project-list").getByText(project.name, { exact: true }).click();
  await page.locator('.project-tab[data-ptab="delivery"]').click();
  await expect(page.getByTestId("delivery-flow-panel")).toBeVisible();
}

test.describe("部署范围与版本安全", () => {
  test.beforeEach(async ({ page, baseURL }) => {
    expect(new URL(baseURL!).hostname).toMatch(/^(127\.0\.0\.1|localhost|\[::1\])$/);
    await loginAsRoot(page);
  });

  test("固定目标保存并发布保留子集，取消最终确认不执行部署", async ({ page }) => {
    const project = await createProject(page, "safe-subset");
    await createPublishedVersion(page, project, "1.0.0");
    await openDelivery(page, project);
    await page.locator("#deliveryTasksDetails > summary").click();
    await page.locator("#pushTaskName").fill("只部署逻辑服子集");
    await page.locator("#pushTaskTargetMode").selectOption("fixed");
    await page.locator("#pushTaskServerIDs").fill("logic-a, logic-c");
    const runs: string[] = [];
    await page.route(`**/api/projects/${project.id}/push-tasks/*/run`, async (route) => {
      runs.push(route.request().url());
      await route.abort();
    });
    const savedRequest = page.waitForRequest((request) => request.method() === "POST" && request.url().endsWith(`/api/projects/${project.id}/push-tasks`));
    await page.locator("#btnSaveAndRunPushTask").click();
    expect((await savedRequest).postDataJSON()).toMatchObject({ server_ids: ["logic-a", "logic-c"] });
    const modal = page.locator(".modal-card");
    await expect(modal).toBeVisible();
    await expect(modal).toContainText("目标：logic-a, logic-c");
    await expect(modal).not.toContainText("全部匹配服务器");
    await modal.getByRole("button", { name: "取消", exact: true }).click();
    await expect(modal).not.toBeVisible();
    expect(runs).toEqual([]);
    const tasks = await (await page.request.get(`/api/projects/${project.id}/push-tasks`)).json();
    expect(tasks).toHaveLength(1);
    expect(tasks[0].server_ids).toEqual(["logic-a", "logic-c"]);
    expect(tasks[0].run_count).toBe(0);
  });

  test("部署最新版本按发布时间选择，不按创建顺序或版本号选择", async ({ page }) => {
    const project = await createProject(page, "safe-latest");
    await createPublishedVersion(page, project, "1.0.0");
    await createPublishedVersion(page, project, "2.0.0");
    const versions = await (await page.request.get(`/api/projects/${project.id}/versions`)).json();
    // Control timestamp ordering without wall-clock sleeps: the older-created
    // version was published later (for example after a delayed approval).
    const reversePublication = [
      { ...versions.find((item: { version: string }) => item.version === "2.0.0"), created_at: "2031-01-02 00:00:00", published_at: "2031-01-03 00:00:00" },
      { ...versions.find((item: { version: string }) => item.version === "1.0.0"), created_at: "2031-01-01 00:00:00", published_at: "2031-01-04 00:00:00" },
    ];
    await page.route(`**/api/projects/${project.id}/versions`, async (route) => {
      await route.fulfill({ json: reversePublication });
    });
    await page.route(`**/api/projects/${project.id}/delivery-nodes`, (route) => route.fulfill({ json: [
      { id: "push:fixture", delivery_mode: "push", server_id: "logic-a", host_name: "fixture-node", status: "unknown" },
    ] }));
    await page.route(`**/api/projects/${project.id}/push-deployments`, async (route) => {
      if (route.request().method() === "POST") {
        await route.fulfill({ status: 201, json: { id: 903, version: "1.0.0", status: "success", selector: JSON.stringify({ dry_run: true }) } });
      } else await route.continue();
    });
    await openDelivery(page, project);
    await expect(page.locator("#deliveryVersionName")).toHaveText("1.0.0");
    const previewRequest = page.waitForRequest((request) => request.method() === "POST" && request.url().endsWith(`/api/projects/${project.id}/push-deployments`));
    await page.getByTestId("quick-deploy-preview").click();
    expect((await previewRequest).postDataJSON()).toMatchObject({ version: "1.0.0", dry_run: true, all_project_targets: true });
    await page.locator("#deliveryTasksDetails > summary").click();
    await expect(page.locator('#pushTaskVersion option[value=""]')).toContainText("1.0.0");
    await page.getByRole("button", { name: "版本", exact: true }).click();
    await expect(page.getByTestId("version-list").locator(".latest-published")).toContainText("1.0.0");
  });

  test("固定版本删除后编辑任务保留失效选择，必须明确选择替代策略", async ({ page }) => {
    const project = await createProject(page, "safe-missing-version");
    await createPublishedVersion(page, project, "1.0.0");
    await createPublishedVersion(page, project, "2.0.0");
    const response = await page.request.post(`/api/projects/${project.id}/push-tasks`, {
      data: { name: "固定历史版本", version: "1.0.0", server_ids: ["logic-a"], tags: ["test"], tag_match: "all" },
    });
    expect(response.ok()).toBeTruthy();
    const task = await response.json();
    expect((await page.request.delete(`/api/projects/${project.id}/versions/1.0.0?confirm=yes`)).ok()).toBeTruthy();
    await openDelivery(page, project);
    await page.locator("#deliveryTasksDetails > summary").click();
    const row = page.getByTestId("release-task-list").getByRole("row").filter({ hasText: "固定历史版本" });
    await row.getByRole("button", { name: "编辑", exact: true }).click();
    await expect(page.locator("#pushTaskVersion")).toHaveValue("1.0.0");
    await expect(page.locator('#pushTaskVersion option[value="1.0.0"]')).toBeDisabled();
    await expect(page.locator('#pushTaskVersion option[value="1.0.0"]')).toContainText("已删除或不可用");
    await page.locator("#pushTaskName").fill("只修改任务名称");
    const writes: unknown[] = [];
    page.on("request", (request) => {
      if (request.method() === "PUT" && request.url().endsWith(`/api/projects/${project.id}/push-tasks/${task.id}`)) writes.push(request.postDataJSON());
    });
    await page.locator("#btnSavePushTask").click();
    await expect(page.locator(".toast").filter({ hasText: "请明确选择一个已发布版本" })).toBeVisible();
    expect(writes).toEqual([]);
    const unchanged = await (await page.request.get(`/api/projects/${project.id}/push-tasks`)).json();
    expect(unchanged[0]).toMatchObject({ version: "1.0.0", name: "固定历史版本" });

    await page.locator("#pushTaskVersion").selectOption("");
    await page.locator("#btnSavePushTask").click();
    await expect.poll(() => writes.length).toBe(1);
    expect(writes[0]).toMatchObject({ version: "", name: "只修改任务名称", server_ids: ["logic-a"] });
    await expect(page.getByTestId("release-task-list")).toContainText("每次取最新已发布");
  });

  test("预演成功与真实部署历史明确区分，预演明细说明未连接SSH", async ({ page }) => {
    const project = await createProject(page, "safe-preview-result");
    await createPublishedVersion(page, project, "2.0.0");
    const target = { host_name: "fixture-node", server_id: "logic-a", status: "success", completed_at: "2031-01-02 00:00:00" };
    // Completed records are read-only fixtures; no real SSH run is issued.
    const actual = { id: 901, project_id: project.id, version: "1.0.0", status: "success", selector: JSON.stringify({ dry_run: false }), created_at: "2031-01-01 00:00:00", targets: [{ ...target, output: "fixture deployment completed" }] };
    const preview = { id: 902, project_id: project.id, version: "2.0.0", status: "success", selector: JSON.stringify({ dry_run: true }), created_at: "2031-01-02 00:00:00", targets: [{ ...target, output: "dry-run: target selected; no SSH connection made" }] };
    await page.route(`**/api/projects/${project.id}/push-deployments`, (route) => route.fulfill({ json: [preview, actual] }));
    await page.route(`**/api/projects/${project.id}/push-deployments/901`, (route) => route.fulfill({ json: actual }));
    await page.route(`**/api/projects/${project.id}/push-deployments/902`, (route) => route.fulfill({ json: preview }));
    await openDelivery(page, project);
    await expect(page.locator("#deliveryLatestMeta")).toContainText("预演 #902");
    await expect(page.locator("#deliveryLatestMeta")).toContainText("未连接 SSH，未变更服务器");
    await expect(page.getByTestId("delivery-latest-targets")).toContainText("预演通过");
    await page.locator("#deliveryHistoryDetails > summary").click();
    const previewRow = page.locator("#pushLogList tbody tr").filter({ has: page.locator('[data-push-log="902"]') });
    const actualRow = page.locator("#pushLogList tbody tr").filter({ has: page.locator('[data-push-log="901"]') });
    await expect(previewRow).toContainText("预演通过");
    await expect(previewRow).toContainText("未连接 SSH");
    await expect(actualRow).toContainText("部署");
    await expect(actualRow).toContainText("成功");
    await expect(actualRow).not.toContainText("预演通过");
    await previewRow.getByRole("button", { name: "查看预演明细" }).click();
    await expect(page.locator(".modal-card")).toContainText("预演 #902");
    await expect(page.locator(".modal-card")).toContainText("未连接 SSH，未变更服务器");
  });

  test("长日志在桌面和移动端内部滚动，关闭按钮始终可达", async ({ page }) => {
    const project = await createProject(page, "safe-long-log");
    const output = "[fixture] simulated log line; no remote connection.\n".repeat(5_000);
    expect(Buffer.byteLength(output)).toBeGreaterThanOrEqual(250_000);
    const deployment = {
      id: 904, project_id: project.id, version: "1.0.0", status: "success",
      selector: JSON.stringify({ dry_run: true }), created_at: "2031-01-02 00:00:00",
      targets: [{ host_name: "fixture-node", server_id: "logic-a", status: "success", output }],
    };
    await page.route(`**/api/projects/${project.id}/push-deployments`, (route) => route.fulfill({ json: [deployment] }));
    await page.route(`**/api/projects/${project.id}/push-deployments/904`, (route) => route.fulfill({ json: deployment }));
    await page.setViewportSize({ width: 1440, height: 900 });
    await openDelivery(page, project);
    await page.locator("#deliveryHistoryDetails > summary").click();
    for (const viewport of [{ width: 1440, height: 900 }, { width: 390, height: 844 }]) {
      await page.setViewportSize(viewport);
      await page.locator('[data-push-log="904"]').click();
      const dialog = page.getByRole("dialog");
      const close = dialog.locator('[data-modal="ok"]');
      await expect(dialog).toBeInViewport({ ratio: 1 });
      await expect(close).toBeInViewport({ ratio: 1 });
      const message = dialog.locator(".modal-message");
      expect(await message.evaluate((element) => element.scrollHeight > element.clientHeight)).toBeTruthy();
      await message.evaluate((element) => { element.scrollTop = element.scrollHeight; });
      expect(await message.evaluate((element) => element.scrollTop > 0)).toBeTruthy();
      await expect(close).toBeInViewport({ ratio: 1 });
      await close.click();
      await expect(dialog).not.toBeVisible();
    }
  });
});
