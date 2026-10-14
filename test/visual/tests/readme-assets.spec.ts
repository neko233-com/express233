import { test, expect, type Page } from "@playwright/test";
import path from "path";
import fs from "fs";
import { randomUUID } from "crypto";
import { loginAsRoot } from "./helpers";

const assetsDir = process.env.EXPRESS233_README_ASSETS_DIR
  ? path.resolve(process.env.EXPRESS233_README_ASSETS_DIR)
  : path.resolve(__dirname, "../../../.assets");
const serverIDs = ["1001", "1002", "1003"];

// Capture actual application data without replacing responses or visible UI.
async function capture(page: Page, filename: string) {
  await expect(page.locator(".toast")).toHaveCount(0, { timeout: 10_000 });
  await page.evaluate(async () => {
    await document.fonts.ready;
    window.scrollTo(0, 0);
    document.querySelector(".main")?.scrollTo(0, 0);
  });
  await page.mouse.move(0, 0);
  await page.screenshot({ path: path.join(assetsDir, filename), animations: "disabled" });
}

async function seedVersion(page: Page, projectID: number, version: string, published: boolean) {
  const base = `/api/projects/${projectID}/versions`;
  expect((await page.request.post(base, { data: { name: version } })).ok()).toBeTruthy();
  const files = [
    {
      name: "config/game.properties", tags: "all",
      content: `# Game demo: shared template\napp.name=game-demo\napp.version=${version}\nserver.id=0\nserver.name=demo-template\nserver.port=18000\nregion=demo\nmax.players=${version === "1.2.0" ? 1000 : 1500}\nmaintenance=false\n`,
    },
    { name: "config/events.json", tags: "all", content: JSON.stringify({ season: "demo-season", dailyResetHour: 5, rewardMultiplier: version === "1.2.0" ? 1 : 1.2 }, null, 2) + "\n" },
    { name: "bin/linux-amd64/game-server", tags: "linux-amd64", content: `Documentation fixture for game-demo ${version}.\nNot an executable game server.\n` },
    { name: "scripts/restart.sh", tags: "linux-amd64", content: "#!/bin/sh\n# Documentation fixture only; no service is started.\necho 'game-demo: local documentation fixture'\n" },
  ];
  for (const file of files) {
    expect((await page.request.post(`${base}/${version}/files`, {
      multipart: { path: file.name, tags: file.tags, file: { name: path.basename(file.name), mimeType: "text/plain", buffer: Buffer.from(file.content) } },
    })).ok()).toBeTruthy();
  }
  if (published) expect((await page.request.post(`${base}/${version}/publish`)).ok()).toBeTruthy();
}

async function selectVersion(page: Page, version: string) {
  await page.locator('.project-tab[data-ptab="versions"]').click();
  await page.getByTestId("version-list").getByText(version, { exact: true }).click();
  await expect(page.locator("#verDetailTitle")).toHaveText(version);
  await page.getByTestId("file-list").locator(".tree-file").filter({ hasText: "game.properties" }).locator(".file-path").click();
  await expect(page.locator("#filePreviewBody")).toContainText(`app.version=${version}`);
}

test.describe("README 功能截图", () => {
  test.use({ viewport: { width: 1440, height: 900 }, deviceScaleFactor: 1 });

  test("capture feature screenshots", async ({ page, baseURL }) => {
    test.setTimeout(120_000);
    // Never create documentation fixtures against a remote or production API.
    expect(new URL(baseURL!).hostname).toMatch(/^(127\.0\.0\.1|localhost|\[::1\])$/);
    fs.mkdirSync(assetsDir, { recursive: true });
    await loginAsRoot(page);
    // Fresh names also isolate retries when a previous attempt could not clean up.
    const projectName = `game-demo-${randomUUID().slice(0, 8)}`;
    const { tenant_slug: tenantSlug } = await (await page.request.get("/api/me")).json();
    const projectPath = `userdata/${tenantSlug}/projects/${projectName}`;
    const previousServerEntries = new Map<string, unknown>();
    const hostIDs: number[] = [];
    let projectID: number | undefined;

    try {
      const created = await page.request.post("/api/projects", { data: { name: projectName } });
      // A 200 response reuses an existing project; it must never become ours to delete.
      expect(created.status(), "The screenshot fixture must own a newly created project").toBe(201);
      projectID = (await created.json()).id;
      expect(projectID).toBeDefined();

      // Preserve each touched entry instead of replacing the suite's complete YAML.
      for (const [index, id] of serverIDs.entries()) {
        const endpoint = `/api/servers/${id}`;
        const existing = await page.request.get(endpoint);
        expect([200, 404]).toContain(existing.status());
        previousServerEntries.set(id, existing.ok() ? (await existing.json()).entry : null);
        expect((await page.request.put(endpoint, { data: {
          replacements: { "game.properties": {
            "server.id": Number(id), "server.name": `Demo Realm ${index + 1}`,
            "server.port": 18001 + index, region: "demo-east",
          } },
          post_hook_env: { SERVER_ID: id },
        } })).ok()).toBeTruthy();
      }
      await seedVersion(page, projectID!, "1.2.0", true);
      await seedVersion(page, projectID!, "1.2.1", true);
      await seedVersion(page, projectID!, "1.3.0", false);

      // Reserved addresses and fictitious names only. Health checks are off;
      // the only deployment request below has dry_run=true.
      for (const [index, ids] of [["1001", "1002"], ["1003"]].entries()) {
        const response = await page.request.post("/api/push/hosts", { data: {
          name: index === 0 ? "game-node-a" : "game-node-b", address: `192.0.2.${10 + index * 10}`,
          port: 22, username: "demo-operator", auth_mode: "agent",
          host_key: "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA",
          health_check_enabled: false,
        } });
        expect(response.ok()).toBeTruthy();
        const host = await response.json();
        hostIDs.push(host.id);
        for (const serverID of ids) {
          expect((await page.request.post(`/api/push/hosts/${host.id}/servers`, { data: {
            project_name: projectName, server_id: serverID, labels: "demo", remote_root: "/srv/game-demo",
            skip_backup: false, os: "linux", arch: "amd64",
          } })).ok()).toBeTruthy();
        }
      }

      await page.reload();
      await expect(page.getByTestId("app-shell")).toBeVisible();
      await page.getByTestId("project-list").getByText(projectName, { exact: true }).click();
      await selectVersion(page, "1.3.0");
      await expect(page.getByTestId("ver-status")).toHaveText("draft");
      await expect(page.getByTestId("file-list")).toContainText("linux-amd64");
      await capture(page, "file-tags.png");

      await selectVersion(page, "1.2.1");
      await expect(page.getByTestId("ver-status")).toHaveText("published");
      await expect(page.getByTestId("version-list").locator(".latest-published")).toContainText("1.2.1");
      await capture(page, "multi-version.png");

      await page.locator('.project-tab[data-ptab="config"]').click();
      await page.locator('[data-server-config="1001"]').click();
      await expect(page.getByTestId("server-override-form").getByLabel("game.properties / server.port", { exact: true })).toHaveValue("18001");
      await expect(page.locator("#serverConfigTitle")).toHaveText("1001");
      await capture(page, "server-config.png");

      await page.locator('.project-tab[data-ptab="preview"]').click();
      await page.getByTestId("preview-server-id").fill("1001");
      await page.getByTestId("preview-submit").click();
      await expect(page.getByTestId("preview-rendered-body")).toContainText("server.port=18001");
      await expect(page.locator("#verPreviewOriginalBody")).toContainText("server.port=18000");
      await expect(page.locator("#btnConfirmPreview")).toBeEnabled();
      await capture(page, "template-replacement.png");

      await page.locator('.project-tab[data-ptab="diff"]').click();
      await page.locator("#diffFromVer").selectOption("1.2.0");
      await page.locator("#diffToVer").selectOption("1.2.1");
      await page.locator("#diffServerId").fill("1001");
      await page.getByRole("button", { name: "生成 diff" }).click();
      await expect(page.locator("#versionDiffOut")).toContainText("game.properties");
      await page.locator("#versionDiffOut [data-diff-index]").filter({ hasText: "game.properties" }).click();
      await expect(page.locator("#versionDiffOut")).toContainText("max.players=1500");
      await capture(page, "version-diff.png");

      await page.locator('.project-tab[data-ptab="delivery"]').click();
      await expect(page.locator("#deliveryVersionName")).toHaveText("1.2.1");
      await expect(page.locator("#deliveryTargetSummary")).toHaveText("SSH 部署 3 个目标");
      await expect(page.getByTestId("delivery-target-list").locator("tbody tr")).toHaveCount(3);
      const previewRequest = page.waitForRequest((request) => request.method() === "POST" && request.url().endsWith(`/api/projects/${projectID}/push-deployments`));
      await page.getByTestId("quick-deploy-preview").click();
      expect((await previewRequest).postDataJSON()).toMatchObject({ version: "1.2.1", dry_run: true });
      await expect(page.getByTestId("delivery-latest-targets").locator("tbody tr")).toHaveCount(3);
      await expect(page.locator("#deliveryLatestMeta")).toContainText("未连接 SSH，未变更服务器");
      await page.locator("#deliveryHistoryDetails > summary").click();
      await capture(page, "deployment-workflow.png");

      await page.getByTestId("quick-deploy").click();
      const dialog = page.getByRole("dialog");
      await expect(dialog).toContainText("版本：1.2.1（最新已发布）");
      for (const id of serverIDs) await expect(dialog).toContainText(id);
      await expect(dialog.getByRole("button", { name: "取消", exact: true })).toBeInViewport({ ratio: 1 });
      await capture(page, "deployment-confirm.png");
      await dialog.getByRole("button", { name: "取消", exact: true }).click();
      await expect(dialog).not.toBeVisible();
      const deployments = await (await page.request.get(`/api/projects/${projectID}/push-deployments`)).json();
      expect(deployments).toHaveLength(1);
      expect(JSON.parse(deployments[0].selector).dry_run).toBe(true);

      await page.getByTestId("nav-storage").click();
      await expect(page.getByTestId("storage-panel")).toBeVisible();
      await expect(page.getByTestId("storage-tree")).toContainText(projectName);
      await page.getByTestId("storage-reindex").click();
      await expect(page.locator(".toast").filter({ hasText: "索引已重建" })).toBeVisible();
      await expect(page.locator(".toast")).toHaveCount(0, { timeout: 10_000 });
      await expect(page.getByTestId("storage-tree")).toContainText(projectName);
      await expect(page.getByTestId("storage-tree")).toContainText("1.3.0");
      // Collapse completed versions so all three versions fit, then inspect
      // the draft file without taking any storage deletion action.
      for (const version of ["1.2.0", "1.2.1"]) {
        await page.getByTestId("storage-tree").locator(`[data-storage-folder="${projectPath}/${version}"]`).click();
        await expect(page.locator("#storageDetail")).toContainText(`${projectName}/${version}`);
      }
      await page.getByTestId("storage-tree").locator(`.storage-tree-row[data-path="${projectPath}/1.3.0/config/game.properties"]`).click();
      await expect(page.locator("#storageDetail")).toContainText("1.3.0/config/game.properties");
      await capture(page, "storage-space.png");
    } finally {
      // Retain the unique project and its deployment history in the disposable
      // test data directory for diagnosis; retries must not depend on project deletion.
      // Remove only our SSH hosts and restore the server entries we touched.
      for (const hostID of hostIDs) {
        expect.soft((await page.request.delete(`/api/push/hosts/${hostID}`)).ok()).toBeTruthy();
      }
      for (const [id, entry] of previousServerEntries) {
        const endpoint = `/api/servers/${id}`;
        const restored = entry === null
          ? await page.request.delete(endpoint)
          : await page.request.put(endpoint, { data: entry });
        expect.soft(restored.ok(), `Restore only the touched server entry ${id}`).toBeTruthy();
      }
    }
  });
});
