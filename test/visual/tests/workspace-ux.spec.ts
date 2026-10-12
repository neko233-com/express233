import { expect, test, type Page } from "@playwright/test";
import path from "path";
import { loginAsRoot } from "./helpers";

const fixture = path.join(__dirname, "../../../testdata/validation-tree/version/deploy/game.properties");

async function createProject(page: Page, prefix: string) {
  const name = `${prefix}-${Date.now()}`;
  await page.getByTestId("new-project-input").fill(name);
  await page.getByTestId("add-project").click();
  // Do not select the sidebar entry: creation itself must enter the workspace.
  await expect(page.getByTestId("cur-project")).toHaveText(name);
  await expect(page.locator("#ptab-versions")).toBeVisible();
  return name;
}

async function createVersion(page: Page, name: string) {
  await page.getByTestId("new-version-input").fill(name);
  await page.getByTestId("add-version").click();
  await expect(page.locator("#verDetailTitle")).toHaveText(name);
  await expect(page.getByTestId("ver-status")).toContainText("draft");
}

async function uploadFixture(page: Page) {
  await page.getByTestId("file-input").setInputFiles(fixture);
  await expect(page.getByTestId("file-list")).toContainText("game.properties");
  await expect(page.getByTestId("workspace-next")).toHaveText("检查并发布");
}

test.describe("简化工作区交互", () => {
  test.beforeEach(async ({ page }) => {
    await loginAsRoot(page);
  });

  test("创建项目自动进入工作区，下一步随草稿内容推进", async ({ page }) => {
    const projectName = await createProject(page, "ux-next");
    await page.reload();
    await expect(page.getByTestId("cur-project")).toHaveText(projectName);
    await expect(page.locator("#ptab-versions")).toBeVisible();
    await expect(page.locator("#workspaceNextTitle")).toHaveText("创建第一个版本");
    await expect(page.getByTestId("new-version-input")).toHaveValue("0.0.1");
    await page.getByTestId("workspace-next").click();
    await expect(page.getByTestId("new-version-input")).toBeFocused();

    await createVersion(page, "0.0.1");
    await expect(page.getByTestId("new-version-input")).toHaveValue("0.0.2");
    await expect(page.locator("#workspaceNextTitle")).toHaveText("上传版本文件");
    await expect(page.getByTestId("workspace-next")).toHaveText("上传文件");
    await expect(page.getByTestId("tag-batch-paths")).not.toBeVisible();
    await expect(page.locator("#versionRetentionLimited")).not.toBeVisible();

    await uploadFixture(page);
    await page.getByTestId("workspace-next").click();
    await expect(page.locator("#ptab-publish")).toBeVisible();
    await page.locator("#btnFlowValidate").click();
    await expect(page.getByTestId("flow-validate-result")).toBeVisible();
    await expect(page.getByTestId("flow-validate-result")).toContainText("可以发布");
  });

  test("发布检查在当前页说明失败，上传修复后可重新检查", async ({ page }) => {
    await createProject(page, "ux-validation");
    await createVersion(page, "1.0.0");
    await page.locator('.project-tab[data-ptab="publish"]').click();
    await page.locator("#btnFlowValidate").click();
    await expect(page.getByTestId("flow-validate-result")).toContainText("暂时无法发布");
    await expect(page.getByTestId("flow-validate-result")).toContainText("版本目录为空");
    await expect(page.getByTestId("workspace-next")).toHaveText("查看问题");

    await page.getByRole("button", { name: "版本", exact: true }).click();
    await uploadFixture(page);
    await page.getByTestId("workspace-next").click();
    await expect(page.getByTestId("flow-validate-result")).not.toContainText("版本目录为空");
    await page.locator("#btnFlowValidate").click();
    await expect(page.getByTestId("flow-validate-result")).toContainText("可以发布");
  });

  test("无分服配置仍可发布，发布后可进入部署并自动选中新草稿", async ({ page }) => {
    const original = await page.request.get("/api/server-yaml");
    expect(original.ok()).toBeTruthy();
    const { content } = await original.json();
    try {
      const clear = await page.request.put("/api/server-yaml", { data: { content: "servers: {}\n" } });
      expect(clear.ok()).toBeTruthy();
      await createProject(page, "ux-no-servers");
      await createVersion(page, "1.0.0");
      await uploadFixture(page);
      await page.getByTestId("workspace-next").click();
      await expect(page.locator("#publishConfigCheck")).toContainText("可跳过");
      await expect(page.locator("#publishPreviewCheck")).toContainText("可选");
      await page.locator("#btnFlowValidate").click();
      await expect(page.getByTestId("flow-validate-result")).toContainText("可以发布");
      await expect(page.locator("#btnFlowPublish")).toBeEnabled();
      await page.locator("#btnFlowPublish").click();
      await page.locator(".modal-card").getByRole("button", { name: "发布", exact: true }).click();
      await expect(page.locator("#btnPublishedDeploy")).toBeVisible();
      await expect(page.getByTestId("workspace-next")).toHaveText("前往部署");
      await page.locator("#btnPublishedDeploy").click();
      await expect(page.getByTestId("delivery-flow-panel")).toBeVisible();
      await expect(page.locator("#deliveryVersionName")).toHaveText("1.0.0");

      await page.getByRole("button", { name: "版本", exact: true }).click();
      await expect(page.getByTestId("ver-status")).toContainText("published");
      await expect(page.getByTestId("file-input")).toBeDisabled();
      await expect(page.locator(".upload-zone")).not.toBeVisible();
      await createVersion(page, "1.0.1");
      await expect(page.getByTestId("file-input")).toBeEnabled();
      await expect(page.locator(".upload-zone")).toBeVisible();
      await expect(page.getByTestId("file-list")).not.toContainText("game.properties");
      await expect(page.getByTestId("workspace-next")).toHaveText("上传文件");
    } finally {
      const restore = await page.request.put("/api/server-yaml", { data: { content } });
      expect(restore.ok()).toBeTruthy();
    }
  });

  test("预览失败会撤销上次确认，不再允许确认旧结果", async ({ page }) => {
    await createProject(page, "ux-preview");
    await createVersion(page, "1.0.0");
    await uploadFixture(page);
    await page.getByRole("button", { name: "拉取预览", exact: true }).click();
    await page.getByTestId("preview-server-id").fill("visual-s1");
    await page.getByTestId("preview-submit").click();
    await expect(page.getByTestId("preview-rendered-body")).toContainText("9001");
    await page.locator("#btnConfirmPreview").click();
    await expect(page.locator("#publishPreviewCheck")).toContainText("已确认");

    await page.getByRole("button", { name: "拉取预览", exact: true }).click();
    await page.getByTestId("preview-server-id").fill("missing-ux-server");
    await expect(page.locator("#btnConfirmPreview")).toBeDisabled();
    await page.getByTestId("preview-submit").click();
    await expect(page.getByTestId("preview-table")).toContainText("预览失败");
    await expect(page.locator("#btnConfirmPreview")).toBeDisabled();
    await expect(page.getByTestId("preview-table")).not.toContainText("port=9001");
    await page.locator('.project-tab[data-ptab="publish"]').click();
    await expect(page.locator("#publishPreviewCheck")).not.toContainText("已确认");
  });

  test("批量上传中切换版本，所有文件仍归属上传开始时的草稿", async ({ page }) => {
    const projectName = await createProject(page, "ux-upload-isolation");
    await createVersion(page, "1.0.0");
    await createVersion(page, "1.0.1");
    await page.getByTestId("version-list").getByText("1.0.0", { exact: true }).click();
    await expect(page.locator("#verDetailTitle")).toHaveText("1.0.0");
    const uploads: string[] = [];
    let releaseFirst!: () => void;
    const release = new Promise<void>((resolve) => { releaseFirst = resolve; });
    await page.route("**/api/projects/*/versions/*/files", async (route) => {
      if (route.request().method() === "POST") {
        uploads.push(route.request().url());
        if (uploads.length === 1) {
          await release;
        }
      }
      await route.continue();
    });
    try {
      await page.getByTestId("file-input").setInputFiles([
        { name: "logic-server.txt", mimeType: "text/plain", buffer: Buffer.from("local fixture one") },
        { name: "gateway-server.txt", mimeType: "text/plain", buffer: Buffer.from("local fixture two") },
      ]);
      await expect.poll(() => uploads.length).toBe(1);
      await page.getByTestId("version-list").getByText("1.0.1", { exact: true }).click();
      await expect(page.locator("#verDetailTitle")).toHaveText("1.0.1");
      releaseFirst();
      await expect(page.getByTestId("workspace-next")).toBeEnabled();
      await expect.poll(() => uploads.length).toBe(2);
      expect(uploads.every((url) => url.includes("/versions/1.0.0/files"))).toBeTruthy();
      await expect(page.locator("#verDetailTitle")).toHaveText("1.0.1");
      await expect(page.getByTestId("file-list")).not.toContainText("logic-server.txt");
      await expect(page.getByTestId("file-list")).not.toContainText("gateway-server.txt");

      const projects = await (await page.request.get("/api/projects")).json();
      const project = projects.find((item: { name: string }) => item.name === projectName);
      const firstFiles = await page.request.get(`/api/projects/${project.id}/versions/1.0.0/files`);
      expect(firstFiles.ok()).toBeTruthy();
      expect(await firstFiles.text()).toContain("logic-server.txt");
      expect(await firstFiles.text()).toContain("gateway-server.txt");
      const secondFiles = await page.request.get(`/api/projects/${project.id}/versions/1.0.1/files`);
      expect(secondFiles.ok()).toBeTruthy();
      expect(await secondFiles.text()).not.toMatch(/logic-server|gateway-server/);
    } finally {
      releaseFirst();
      await page.unroute("**/api/projects/*/versions/*/files");
    }
  });

  test("分服参数表单保留值类型与启动环境，支持添加并阻止重复参数", async ({ page }) => {
    const serverID = `ux-form-${Date.now()}`;
    const entry = {
      replacements: {
        "game.properties": { port: 9002, name: "logic-original", enabled: true },
      },
      post_hook_env: { SERVER_ID: serverID, GAME_ENV: "local-fixture" },
    };
    const created = await page.request.put(`/api/servers/${serverID}`, { data: entry });
    expect(created.ok()).toBeTruthy();
    try {
      await createProject(page, "ux-config-form");
      await createVersion(page, "1.0.0");
      await uploadFixture(page);
      await page.locator('.project-tab[data-ptab="config"]').click();
      await page.locator(`[data-server-config="${serverID}"]`).click();
      const form = page.getByTestId("server-override-form");
      await expect(form).toBeVisible();
      await expect(page.locator("#serverConfigJSON")).not.toBeVisible();
      await expect(form.getByLabel("game.properties / port", { exact: true })).toHaveValue("9002");
      const saves: string[] = [];
      page.on("request", (request) => {
        if (request.method() === "PUT" && request.url().endsWith(`/api/servers/${serverID}`)) saves.push(request.url());
      });
      await form.getByLabel("game.properties / port", { exact: true }).fill("invalid-number");
      await page.locator("#btnSaveServerConfig").click();
      await expect(page.getByTestId("server-config-panel")).toBeVisible();
      expect(saves).toHaveLength(0);
      await form.getByLabel("game.properties / port", { exact: true }).fill("9010");
      await form.getByLabel("game.properties / name", { exact: true }).fill("logic-updated");
      await form.getByLabel("game.properties / enabled", { exact: true }).selectOption("false");
      await page.getByRole("button", { name: "版本", exact: true }).click();
      await page.locator('.project-tab[data-ptab="config"]').click();
      await expect(form.getByLabel("game.properties / name", { exact: true })).toHaveValue("logic-updated");
      await expect(page.locator("#serverConfigSaveState")).toContainText("未保存");
      // Saving existing fields must not require filling the optional add-parameter row.
      await page.locator("#btnSaveServerConfig").click();
      await expect(page.locator("#ptab-preview")).toBeVisible();
      const saved = await (await page.request.get(`/api/servers/${serverID}`)).json();
      expect(saved.entry.replacements["game.properties"]).toEqual({ port: 9010, name: "logic-updated", enabled: false });
      expect(saved.entry.post_hook_env).toEqual(entry.post_hook_env);

      await page.locator('.project-tab[data-ptab="config"]').click();
      await expect(form.getByLabel("game.properties / port", { exact: true })).toHaveValue("9010");
      await form.getByLabel("覆盖文件名", { exact: true }).fill("game.properties");
      await form.getByLabel("覆盖参数名", { exact: true }).fill("capacity");
      await form.getByLabel("覆盖参数值", { exact: true }).fill("100");
      await form.getByLabel("参数值类型", { exact: true }).selectOption("number");
      await form.getByRole("button", { name: "添加参数", exact: true }).click();
      await expect(form.getByLabel("game.properties / capacity", { exact: true })).toHaveValue("100");

      await form.getByLabel("覆盖文件名", { exact: true }).fill("game.properties");
      await form.getByLabel("覆盖参数名", { exact: true }).fill("capacity");
      await form.getByLabel("覆盖参数值", { exact: true }).fill("200");
      await form.getByRole("button", { name: "添加参数", exact: true }).click();
      await expect(form.getByRole("status")).toContainText("已存在");
      await expect(form.getByLabel("game.properties / capacity", { exact: true })).toHaveValue("100");
      await page.locator("#btnSaveServerConfig").click();
      await expect(page.locator("#ptab-preview")).toBeVisible();
      const final = await (await page.request.get(`/api/servers/${serverID}`)).json();
      expect(final.entry.replacements["game.properties"]).toEqual({ port: 9010, name: "logic-updated", enabled: false, capacity: 100 });
      expect(final.entry.post_hook_env).toEqual(entry.post_hook_env);
    } finally {
      const removed = await page.request.delete(`/api/servers/${serverID}`);
      expect(removed.ok()).toBeTruthy();
    }
  });
});
