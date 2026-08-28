import { test, expect } from "@playwright/test";
import path from "path";
import fs from "fs";
import { loginAsRoot } from "./helpers";

const assetsDir = path.resolve(__dirname, "../../../.assets");

test.describe("README 功能截图", () => {
  test.beforeAll(() => {
    fs.mkdirSync(assetsDir, { recursive: true });
  });

  test.beforeEach(async ({ page }) => {
    await loginAsRoot(page);
  });

  test("capture feature screenshots", async ({ page }) => {
    test.setTimeout(120_000);
    const defaultYaml = await page.evaluate(async () => (await fetch("/api/server-yaml")).json().then((x) => x.content));
    const projectName = `readme-demo-${Date.now()}`;
    const fixture = path.join(__dirname, "../../../testdata/validation-tree/version/deploy/game.properties");

    await page.getByTestId("new-project-input").fill(projectName);
    await page.getByTestId("add-project").click();
    await page.getByTestId("project-list").getByText(projectName, { exact: true }).click();

    await page.getByTestId("new-version-input").fill("1.0.0");
    await page.getByTestId("add-version").click();
    await page.getByTestId("version-list").getByText("1.0.0").click();
    await page.getByTestId("upload-tags").fill("linux-amd64");
    await page.getByTestId("file-input").setInputFiles(fixture);
    await expect(page.getByTestId("file-list")).toContainText("linux-amd64", { timeout: 15_000 });
    await page.screenshot({ path: path.join(assetsDir, "file-tags.png"), fullPage: true });

    await page.getByTestId("publish-version").click();
    await page.locator(".modal-card").getByRole("button", { name: "发布" }).click();
    await expect(page.getByTestId("version-list")).toContainText("published", { timeout: 15_000 });

    await page.locator('.project-tab[data-ptab="versions"]').click();
    await page.getByTestId("new-version-input").fill("2.0.0");
    await page.getByTestId("add-version").click();
    await page.getByTestId("version-list").getByText("2.0.0").click();
    await page.getByTestId("file-input").setInputFiles(fixture);
    await expect(page.getByTestId("file-list")).toContainText("game.properties", { timeout: 15_000 });
    await page.evaluate(async (name) => {
      const projects = await fetch("/api/projects").then((response) => response.json());
      const project = projects.find((item) => item.name === name);
      const body = new FormData();
      body.append("file", new Blob(["mysql.url=jdbc:mysql://db.v2:3306/game\n"], { type: "text/plain" }), "game.properties");
      const response = await fetch(`/api/projects/${project.id}/versions/2.0.0/files`, { method: "POST", body });
      if (!response.ok) throw new Error(await response.text());
    }, projectName);
    await page.getByTestId("version-list").getByText("2.0.0").click();
    await page.getByTestId("publish-version").click();
    await page.locator(".modal-card").getByRole("button", { name: "发布" }).click();
    await expect(page.getByTestId("version-list")).toContainText("published", { timeout: 15_000 });
    await page.locator('.project-tab[data-ptab="versions"]').click();
    await page.screenshot({ path: path.join(assetsDir, "multi-version.png"), fullPage: true });

    await page.getByRole("button", { name: "差异" }).click();
    await page.locator("#diffFromVer").selectOption("1.0.0");
    await page.locator("#diffToVer").selectOption("2.0.0");
    await page.locator("#diffServerId").fill("visual-s1");
    await page.getByRole("button", { name: "生成 diff" }).click();
    await expect(page.locator("#versionDiffOut")).toContainText("game.properties", { timeout: 15_000 });
    await page.screenshot({ path: path.join(assetsDir, "version-diff.png"), fullPage: true });

    await page.evaluate(async () => {
      await fetch("/api/server-yaml", {
        method: "PUT",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({
          content: `servers:
  visual-s1:
    replacements:
      game.properties:
        mysql.url: "jdbc:mysql://db.prod:3306/game_live"
`,
        }),
      });
    });
    await page.locator('.project-tab[data-ptab="versions"]').click();
    await page.getByTestId("version-list").getByText("1.0.0").click();
    await page.locator('.project-tab[data-ptab="preview"]').click();
    await page.getByTestId("preview-server-id").fill("visual-s1");
    await page.getByTestId("preview-submit").click();
    await expect(page.getByTestId("preview-table")).toContainText("game.properties", { timeout: 15_000 });
    await expect(page.getByTestId("preview-rendered-body")).toContainText("db.prod", { timeout: 15_000 });
    await page.screenshot({ path: path.join(assetsDir, "template-replacement.png"), fullPage: true });

    await page.getByTestId("nav-storage").click();
    await expect(page.getByTestId("storage-panel")).toBeVisible();
    await page.getByTestId("storage-reindex").click();
    await expect(page.getByTestId("storage-tree")).toContainText(projectName, { timeout: 15_000 });
    await page.screenshot({ path: path.join(assetsDir, "storage-space.png"), fullPage: true });

    await page.evaluate(async (content) => {
      await fetch("/api/server-yaml", {
        method: "PUT",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ content }),
      });
    }, defaultYaml);
  });
});
