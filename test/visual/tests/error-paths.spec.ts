import { test, expect } from "@playwright/test";

// ─── helpers ───────────────────────────────────────────
async function login(page) {
  await page.goto("/");
  await expect(page.getByTestId("login-panel")).toBeVisible();
  await page.getByTestId("login-submit").click();
  await expect(page.getByTestId("app-shell")).toBeVisible();
  await expect(page.getByTestId("whoami")).toContainText("root");
}

async function createProject(page, name: string) {
  await page.getByTestId("new-project-input").fill(name);
  await page.getByTestId("add-project").click();
  await page.getByTestId("project-list").getByText(name, { exact: true }).click();
  await expect(page.getByTestId("cur-project")).toContainText(name);
}

async function createVersion(page, ver: string) {
  await page.getByTestId("new-version-input").fill(ver);
  await page.getByTestId("add-version").click();
  await page.getByTestId("version-list").getByText(ver).click();
  await expect(page.getByTestId("version-detail")).toBeVisible();
}

// ─── Tests ─────────────────────────────────────────────

test.describe("登录与认证", () => {
  test("错误密码登录失败", async ({ page }) => {
    await page.goto("/");
    await expect(page.getByTestId("login-panel")).toBeVisible();
    // 清空默认值，输入错误密码
    await page.locator("#pass").fill("wrong-password");
    await page.getByTestId("login-submit").click();
    // 应该显示错误信息
    await expect(page.locator("#loginErr")).toContainText(/\S+/, { timeout: 5_000 });
    // 不应该进入 app shell
    await expect(page.getByTestId("app-shell")).not.toBeVisible();
  });

  test("空用户名登录失败", async ({ page }) => {
    await page.goto("/");
    await expect(page.getByTestId("login-panel")).toBeVisible();
    await page.locator("#user").fill("");
    await page.getByTestId("login-submit").click();
    await expect(page.locator("#loginErr")).toContainText(/\S+/, { timeout: 5_000 });
  });
});

test.describe("项目操作", () => {
  test.beforeEach(async ({ page }) => {
    await login(page);
  });

  test("创建项目后列表可见", async ({ page }) => {
    const name = `proj-list-${Date.now()}`;
    await createProject(page, name);
    // 项目出现在列表中
    await expect(page.getByTestId("project-list").getByText(name, { exact: true })).toBeVisible();
  });

  test("空项目名不创建", async ({ page }) => {
    await page.getByTestId("new-project-input").fill("");
    await page.getByTestId("add-project").click();
    await expect(page.getByTestId("new-project-input")).toBeVisible();
    await expect(page.getByTestId("new-project-input")).toHaveValue("");
  });

  test("选择项目后显示工作区 tabs", async ({ page }) => {
    const name = `proj-tabs-${Date.now()}`;
    await createProject(page, name);
    await expect(page.getByRole("button", { name: "版本" })).toBeVisible();
    await expect(page.getByRole("button", { name: "拉取预览" })).toBeVisible();
    await expect(page.getByRole("button", { name: "团队" })).toBeVisible();
    await expect(page.getByRole("button", { name: "部署" })).toBeVisible();
    await expect(page.getByRole("button", { name: "差异" })).toBeVisible();
  });
});

test.describe("版本管理", () => {
  test.beforeEach(async ({ page }) => {
    await login(page);
    const name = `ver-mgr-${Date.now()}`;
    await createProject(page, name);
  });

  test("创建版本后显示 draft 状态", async ({ page }) => {
    await createVersion(page, "2.0.0");
    await expect(page.getByTestId("ver-status")).toContainText("draft");
  });

  test("删除版本需要确认", async ({ page }) => {
    await createVersion(page, "3.0.0");
    // 拦截 dialog
    let dialogSeen = false;
    page.on("dialog", (d) => {
      dialogSeen = true;
      d.dismiss();
    });
    await page.getByRole("button", { name: "删除版本" }).click();
    await expect.poll(() => dialogSeen, { timeout: 5_000 }).toBeTruthy();
  });
});

test.describe("拉取预览 — 未注册 serverId 报错", () => {
  test.beforeEach(async ({ page }) => {
    await login(page);
    const name = `preview-err-${Date.now()}`;
    await createProject(page, name);
    await createVersion(page, "1.0.0");
  });

  test("使用不存在的 server_id 预览应报错", async ({ page }) => {
    await page.getByRole("button", { name: "拉取预览" }).click();
    await page.getByTestId("preview-server-id").fill("nonexistent-server-id");
    // 拦截 alert dialog
    let alertMsg = "";
    page.once("dialog", (d) => {
      alertMsg = d.message();
      d.accept();
    });
    await page.getByTestId("preview-submit").click();
    // 应该弹出错误提示
    await expect.poll(() => alertMsg).toContain("server_id", { timeout: 10_000 });
  });

  test("空 server_id 预览应报错", async ({ page }) => {
    await page.getByRole("button", { name: "拉取预览" }).click();
    // server_id 留空
    let alertMsg = "";
    page.once("dialog", (d) => {
      alertMsg = d.message();
      d.accept();
    });
    await page.getByTestId("preview-submit").click();
    await expect.poll(() => alertMsg, { timeout: 5_000 }).toBeTruthy();
  });
});

test.describe("拉取预览 — 正常流程", () => {
  test.beforeEach(async ({ page }) => {
    await login(page);
    const name = `preview-ok-${Date.now()}`;
    await createProject(page, name);
    await createVersion(page, "1.0.0");
  });

  test("已注册 server_id 预览成功显示替换结果", async ({ page }) => {
    // 上传 fixture 文件
    const fixture = "D:\\Code\\neko233-Projects\\express233\\testdata\\validation-tree\\version\\deploy\\game.properties";
    await page.getByTestId("file-input").setInputFiles(fixture);
    await expect(page.getByTestId("file-list")).toContainText("game.properties", { timeout: 15_000 });

    // 切换到拉取预览 tab
    await page.getByRole("button", { name: "拉取预览" }).click();
    await page.getByTestId("preview-server-id").fill("visual-s1");
    await page.getByTestId("preview-submit").click();

    // 预览表格显示配置键变更
    await expect(page.getByTestId("preview-table")).toContainText("game.properties", { timeout: 10_000 });
    // 替换后全文包含 server_id 对应的值
    await expect(page.getByTestId("preview-rendered-body")).toContainText("9001", { timeout: 10_000 });
  });
});

test.describe("发布前检查与发布", () => {
  test.beforeEach(async ({ page }) => {
    await login(page);
    const name = `pub-${Date.now()}`;
    await createProject(page, name);
    await createVersion(page, "1.0.0");
  });

  test("发布前检查通过才能发布", async ({ page }) => {
    const fixture = "D:\\Code\\neko233-Projects\\express233\\testdata\\validation-tree\\version\\deploy\\game.properties";
    await page.getByTestId("file-input").setInputFiles(fixture);
    await expect(page.getByTestId("file-list")).toContainText("game.properties", { timeout: 15_000 });

    await page.getByTestId("validate-version").click();
    await expect(page.getByTestId("validate-result")).toContainText("可以发布", { timeout: 10_000 });
  });

  test("发布后状态变为 published 且出现下载按钮", async ({ page }) => {
    const fixture = "D:\\Code\\neko233-Projects\\express233\\testdata\\validation-tree\\version\\deploy\\game.properties";
    await page.getByTestId("file-input").setInputFiles(fixture);
    await expect(page.getByTestId("file-list")).toContainText("game.properties", { timeout: 15_000 });

    page.once("dialog", (d) => d.accept());
    await page.getByTestId("publish-version").click();
    await expect(page.getByTestId("version-list")).toContainText("published", { timeout: 15_000 });
    await expect(page.getByTestId("ver-status")).toContainText("published", { timeout: 5_000 });
    await expect(page.getByTestId("download-version")).toBeVisible();
  });
});

test.describe("部署命令 tab", () => {
  test.beforeEach(async ({ page }) => {
    await login(page);
    const name = `deploy-tab-${Date.now()}`;
    await createProject(page, name);
    await createVersion(page, "1.0.0");
  });

  test("部署 tab 显示复制命令", async ({ page }) => {
    await page.getByRole("button", { name: "部署" }).click();
    await expect(page.getByTestId("deploy-cmd")).toBeVisible();
    await expect(page.getByTestId("deploy-cmd")).toContainText("express233-cli pull");
  });

  test("复制按钮可点击", async ({ page }) => {
    await page.getByRole("button", { name: "部署" }).click();
    await expect(page.getByTestId("deploy-cmd")).toBeVisible();
    // 复制按钮存在且可点击
    const copyBtn = page.getByRole("button", { name: "复制脚本" });
    await expect(copyBtn).toBeVisible();
    await copyBtn.click();
  });
});

test.describe("差异 tab", () => {
  test("差异 tab 可切换显示", async ({ page }) => {
    await login(page);
    const name = `diff-tab-${Date.now()}`;
    await createProject(page, name);
    await createVersion(page, "1.0.0");
    await page.getByRole("button", { name: "差异" }).click();
    // diff tab 面板可见
    await expect(page.locator("#ptab-diff")).toBeVisible();
    await expect(page.locator("#diffFromVer")).toBeVisible();
  });
});

test.describe("团队 tab", () => {
  test("团队 tab 显示邀请控件", async ({ page }) => {
    await login(page);
    const name = `team-tab-${Date.now()}`;
    await createProject(page, name);
    await page.getByRole("button", { name: "团队" }).click();
    await expect(page.locator("#ptab-team")).toBeVisible();
    await expect(page.locator("#inviteRole")).toBeVisible();
    await expect(page.getByRole("button", { name: "生成邀请链接" })).toBeVisible();
  });
});

test.describe("server.yaml 页签", () => {
  test.beforeEach(async ({ page }) => {
    await login(page);
  });

  test("server.yaml 编辑器可加载内容", async ({ page }) => {
    await page.locator('.sidebar-nav-item[data-global="server"]').click();
    await expect(page.getByTestId("server-yaml-editor")).toBeVisible();
    const yaml = await page.getByTestId("server-yaml-editor").inputValue();
    expect(yaml).toContain("visual-s1");
  });

  test("server.yaml 保存按钮可点击", async ({ page }) => {
    await page.locator('.sidebar-nav-item[data-global="server"]').click();
    await expect(page.getByTestId("server-yaml-editor")).toBeVisible();
    // 拦截保存成功的 alert
    let alertMsg = "";
    page.once("dialog", (d) => {
      alertMsg = d.message();
      d.accept();
    });
    await page.getByRole("button", { name: "保存" }).click();
    await expect.poll(() => alertMsg).toContain("保存", { timeout: 5_000 });
  });

  test("server.yaml 页面预览 — 未注册 server_id 报错", async ({ page }) => {
    await page.locator('.sidebar-nav-item[data-global="server"]').click();
    await page.locator("#previewServerId").fill("this-server-does-not-exist");
    await page.locator("#previewProject").fill("some-project");
    await page.locator("#previewVersion").fill("1.0.0");
    let alertMsg = "";
    page.once("dialog", (d) => {
      alertMsg = d.message();
      d.accept();
    });
    await page.getByRole("button", { name: "预览 diff" }).click();
    await expect.poll(() => alertMsg, { timeout: 10_000 }).toBeTruthy();
  });
});

test.describe("系统设置页签", () => {
  test.beforeEach(async ({ page }) => {
    await login(page);
  });

  test("系统设置显示账号和审计 tab", async ({ page }) => {
    await page.locator('.sidebar-nav-item[data-global="settings"]').click();
    await expect(page.locator("#stab-users")).toBeVisible();
    // 切换审计 tab
    await page.locator('#settingsTabs .seg-tab[data-stab="audit"]').click();
    await expect(page.locator("#stab-audit")).toBeVisible();
    await expect(page.locator("#stab-users")).not.toBeVisible();
  });

  test("创建用户", async ({ page }) => {
    await page.locator('.sidebar-nav-item[data-global="settings"]').click();
    const username = `testuser-${Date.now()}`;
    await page.locator("#newUser").fill(username);
    await page.locator("#newUserPass").fill("testpass123");
    await page.locator("#newUserRole").selectOption("viewer");
    await page.getByRole("button", { name: "创建" }).click();
    await expect(page.locator("#userTable")).toContainText(username, { timeout: 5_000 });
  });

  test("用户表显示 root 用户", async ({ page }) => {
    await page.locator('.sidebar-nav-item[data-global="settings"]').click();
    await expect(page.locator("#userTable")).toContainText("root");
  });
});

test.describe("侧边栏导航", () => {
  test.beforeEach(async ({ page }) => {
    await login(page);
  });

  test("导航高亮跟随点击", async ({ page }) => {
    // 默认工作区高亮
    await expect(page.locator('.sidebar-nav-item[data-global="workspace"]')).toHaveClass(/active/);

    // 点击 server.yaml
    await page.locator('.sidebar-nav-item[data-global="server"]').click();
    await expect(page.locator('.sidebar-nav-item[data-global="server"]')).toHaveClass(/active/);
    await expect(page.locator('.sidebar-nav-item[data-global="workspace"]')).not.toHaveClass(/active/);

    // 点回工作区
    await page.locator('.sidebar-nav-item[data-global="workspace"]').click();
    await expect(page.locator('.sidebar-nav-item[data-global="workspace"]')).toHaveClass(/active/);
  });

  test("项目搜索过滤", async ({ page }) => {
    const nameA = `search-a-${Date.now()}`;
    const nameB = `search-b-${Date.now()}`;
    await createProject(page, nameA);
    await createProject(page, nameB);

    // 搜索 nameA
    await page.locator("#projectSearch").fill("search-a");
    await expect(page.getByTestId("project-list").getByText(nameA, { exact: true })).toBeVisible();
    await expect(page.getByTestId("project-list").getByText(nameB, { exact: true })).not.toBeVisible();

    // 清空搜索
    await page.locator("#projectSearch").fill("");
    await expect(page.getByTestId("project-list").getByText(nameA, { exact: true })).toBeVisible();
    await expect(page.getByTestId("project-list").getByText(nameB, { exact: true })).toBeVisible();
  });
});
