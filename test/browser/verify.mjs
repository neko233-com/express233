import assert from "node:assert/strict";
import { writeFileSync } from "node:fs";
import { join } from "node:path";
import { tab, testID as tid, text, withBrowser } from "./harness.mjs";

const start = performance.now();
const fileName = "game.properties";
const projectName = "game-demo";
const serverID = "1001";
const source = "server.port=18000\nmax.players=1000\n";
const ready = (id) => ["wait", tid(id)];
const click = (id) => ["click", tid(id)];
const form = (id, value) => ["fill", tid(id), value];
const noOverflow = ["wait", "--fn", "document.documentElement.scrollWidth <= window.innerWidth"];
const noSidebarOverflow = ["wait", "--fn", "(() => { const list = document.querySelector('.project-list'); return list && list.scrollWidth <= list.clientWidth; })()"];
const top = ["eval", "document.querySelector('.main').scrollTop = 0; window.scrollTo(0, 0); true"];
const noToast = ["wait", ".toast", "--state", "hidden"];
const button = (name) => ["find", "role", "button", "click", "--name", name];

await withBrowser(async ({ base, batch, api, capture, dir, fixturePassword }) => {
  const fixture = join(dir, fileName);
  writeFileSync(fixture, source);
  const initial = await fetch(base + "/projects/1/preview?version=1.0.0");
  assert.equal(initial.status, 200);
  assert.equal(initial.headers.get("cache-control"), "no-cache");
  const html = await initial.text();
  const mainAsset = html.match(/src="(\/assets\/[^"]+\.js)"/)?.[1];
  assert.ok(mainAsset, "Packaged HTML references a hashed Vite entry");
  const served = await fetch(base + mainAsset, { method: "HEAD" });
  assert.equal(served.status, 200);
  assert.match(served.headers.get("cache-control") ?? "", /immutable/);
  assert.equal((await fetch(base + "/assets/missing.js")).status, 404);
  assert.equal((await fetch(base + "/api/openapi.yaml")).status, 200);
  assert.equal((await fetch(base + "/docs/")).status, 200);

  for (const n of [1001, 1002, 1003]) {
    await api(`/api/servers/${n}`, "PUT", {
      replacements: { [fileName]: { "server.port": 17000 + n } },
      post_hook: "", post_hook_env: {},
    });
  }

  batch("登录、创建项目与草稿、上传并浏览制品", [
    ["set", "viewport", "1440", "900"], ["open", base], ["wait", "#user"],
    ["fill", "#user", "root"], ["fill", "#pass", fixturePassword], click("login-submit"), ready("app-shell"),
    form("new-project-input", projectName), click("add-project"), text(tid("cur-project"), projectName),
    ["wait", tid("new-version-input")], form("new-version-input", "1.0.0"), click("add-version"), text(tid("ver-status"), "draft"),
    form("upload-tags", "linux-amd64"), ["upload", tid("file-input"), fixture],
    text(tid("file-list"), "linux-amd64"), ["click", ".file-path"], text("#filePreviewBody", "server.port=18000"),
    ["click", "#versionAdvanced > summary"], form("tag-batch-paths", fileName), form("tag-batch-tags", "linux"),
    ["select", tid("tag-batch-mode"), "add"], click("tag-batch-apply"),
    ["wait", "--fn", `[...document.querySelectorAll('.file-list .badge')].some(x => x.textContent === 'linux')`],
    top, ...capture("file-tags"),
  ]);
  const projects = await api("/api/projects");
  const project = projects.find((row) => row.name === projectName);
  assert.ok(project);
  const route = `${base}/projects/${project.id}`;
  const path = `/api/projects/${project.id}`;
  const longProjects = Array.from({ length: 12 }, (_, i) => `fixture-game-logic-server-web-release-region-${String(i + 1).padStart(2, "0")}`);
  for (const name of longProjects) await api("/api/projects", "POST", { name }, 201);

  batch("宽屏项目目录、搜索与长名称", [
    ["set", "viewport", "2560", "1380"], ["open", base],
    text(tid("project-directory"), longProjects[11]), noOverflow, noSidebarOverflow,
    ["wait", "--fn", "document.querySelector('.page-header').getBoundingClientRect().left - document.querySelector('.main').getBoundingClientRect().left < 48"],
    ["fill", "#projectSearch", "region-12"], text(tid("project-list"), longProjects[11]),
    ["wait", "--fn", "document.querySelectorAll('.project-directory li').length === 1"], noSidebarOverflow,
    ["press", "Control+a"], ["press", "Backspace"], text(tid("project-directory"), projectName), ...capture("desktop-projects"),
    ["click", `${tid("project-directory")} a[href="/projects/${project.id}/versions"]`],
    text(tid("cur-project"), projectName), ["set", "viewport", "1440", "900"],
  ]);

  batch("分服表单、预览确认、版本发布与下载", [
    ["click", tab("config")], ["wait", '[data-server-config="1001"]'], ["click", '[data-server-config="1001"]'],
    ["wait", '[aria-label="game.properties / server.port"]'], ...capture("server-config"),
    ["click", tab("preview")], ready("preview-server-id"), form("preview-server-id", serverID), click("preview-submit"),
    text(tid("preview-rendered-body"), "server.port=18001"), text("#verPreviewOriginalBody", "server.port=18000"),
    ...capture("template-replacement"),
    ["click", "#btnConfirmPreview"], text("#publishPreviewCheck", "已确认"), ["click", "#btnFlowValidate"],
    text(tid("flow-validate-result"), "可以发布"), ["click", "#btnFlowPublish"], ["wait", 'dialog[open]'],
    ["click", 'dialog [data-modal="ok"]'], ["wait", "#btnPublishedDeploy"], ["click", tab("versions")],
    text(tid("ver-status"), "published"), ["wait", tid("download-version")],
    ["wait", "--fn", 'document.querySelector("#fileInput").disabled'],
  ]);
  assert.equal((await api(path + "/versions")).find((row) => row.version === "1.0.0").status, "published");

  // Build fixture tasks and test-network bindings without connecting to SSH.
  const host = await api("/api/push/hosts", "POST", {
    name: "game-node-a", address: "192.0.2.10", port: 22, username: "fixture",
    auth_mode: "agent", health_check_enabled: false,
    host_key: "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA",
  }, 201);
  for (const n of [1001, 1002, 1003]) {
    await api(`/api/push/hosts/${host.id}/servers`, "POST", {
      project_name: projectName, server_id: String(n), labels: "test", content_tags: "linux-amd64",
      remote_root: "/opt/game-servers", os: "linux", arch: "amd64",
    }, 201);
  }
  const other = "1.0.1";
  await api(path + "/versions", "POST", { name: other }, 201);
  const body = new FormData();
  body.append("file", new File([source.replace("1000", "1500")], fileName));
  await api(`${path}/versions/${other}/files`, "POST", body);
  await api(`${path}/versions/${other}/publish`, "POST", undefined);
  const draft = await api(path + "/versions", "POST", { name: "1.1.0" }, 201);
  const tagBody = new FormData();
  tagBody.append("file", new File([source], fileName));
  tagBody.append("tags", "linux-amd64");
  await api(`${path}/versions/${draft.version}/files`, "POST", tagBody);

  batch("深链刷新、版本差异、桌面文件树", [
    ["open", `${route}/versions?version=${draft.version}`], text("#verDetailTitle", draft.version),
    text(tid("file-list"), "linux-amd64"), ["click", ".file-path"], text("#filePreviewBody", "max.players=1000"),
    top, noOverflow, ...capture("multi-version"),
    ["click", tab("diff")], ["wait", "#diffFromVer"], ["select", "#diffFromVer", "1.0.0"],
    ["select", "#diffToVer", other], ["fill", "#diffServerId", serverID],
    ["find", "role", "button", "click", "--name", "生成 diff"], text("#versionDiffOut", "max.players=1500"),
    ...capture("version-diff"),
    ["back"], text("#verDetailTitle", draft.version),
  ]);

  batch("部署预演、逐服明细、正式部署确认及取消", [
    ["click", tab("delivery")], text("#deliveryVersionName", other),
    text(tid("delivery-target-list"), "1003"), click("quick-deploy-preview"),
    text("#deliveryLatestMeta", "未连接 SSH，未变更服务器"), text(tid("delivery-latest-targets"), "预演通过"),
    ["wait", "#pushLogList a[aria-current=true]"], button("查看预演明细"),
    text("dialog[open]", "1003"), ["press", "Escape"], ["wait", "dialog[open]", "--state", "hidden"],
    top, ...(capture("deployment-workflow").length ? [noToast, ...capture("deployment-workflow")] : []),
    click("quick-deploy"), text("dialog[open]", "SSH 部署 3 个目标"),
    ...capture("deployment-confirm"), ["click", "dialog .modal-actions button:first-child"], ["wait", "dialog[open]", "--state", "hidden"],
  ]);
  const history = await api(path + "/push-deployments");
  assert.equal(history.length, 1);
  assert.equal(JSON.parse(history[0].selector).dry_run, true);

  batch("高级发布独立页签、固定版本与目标任务", [
    ["click", tab("automation")], ready("automation-flow-panel"), button("新建任务"),
    ["fill", "#pushTaskName", "logic-subset"], ["select", "#pushTaskVersion", "1.0.0"],
    ["select", "#pushTaskTargetMode", "fixed"], ["fill", "#pushTaskServerIDs", "1001, 1003"],
    ["click", "#btnSaveAndRunPushTask"], text("dialog[open]", "目标：1001, 1003"),
    ["click", "dialog .modal-actions button:first-child"], text(tid("release-task-list"), "logic-subset"),
    top, ...(capture("advanced-release").length ? [noToast, ...capture("advanced-release")] : []),
    button("预演"), text("#deliveryLatestMeta", "1.0.0"), text(tid("delivery-latest-targets"), "1003"),
    ["wait", "--fn", `document.querySelectorAll('${tid("delivery-latest-targets")} tbody tr').length === 2`],
    ["reload"], text("#deliveryLatestMeta", "1.0.0"), text(tid("delivery-latest-targets"), "预演通过"),
  ]);
  const taskPreview = (await api(path + "/push-deployments"))[0];
  assert.equal(taskPreview.version, "1.0.0");
  assert.equal(JSON.parse(taskPreview.selector).dry_run, true);
  const previewDetail = await api(path + "/push-deployments/" + taskPreview.id);
  assert.deepEqual(previewDetail.targets.map((target) => target.server_id).sort(), ["1001", "1003"]);

  batch("自动 Hook 创建、深链、合并窗口和取消", [
    ["open", `${route}/automation?section=hooks`], ["wait", `${tid("new-release-hook")}:enabled`],
    click("new-release-hook"), ["wait", "#releaseHookName"],
    ["fill", "#releaseHookName", "sample-hook"], ["select", "#releaseHookTask", "logic-subset · 固定目标：1001, 1003"],
    ["fill", "#releaseHookDebounce", "3600"], ["uncheck", "#releaseHookEnabled"], button("保存 Hook"),
    text(tid("release-hook-list"), "已停用"),
    ["reload"], text(tid("release-hook-list"), "sample-hook"),
    ["wait", "#deliveryAutomationToggle:enabled"], button("启用全部"),
    ["click", 'dialog [data-modal="ok"]'], text(tid("release-hook-list"), "已启用"),
    button("触发 Hook"), text("dialog[open]", other), ["click", 'dialog [data-modal="ok"]'],
    text(tid("release-hook-list"), "等待中"), text(tid("release-hook-events"), "首次触发"),
    button("停用全部"), ["click", 'dialog [data-modal="ok"]'], text(tid("release-hook-events"), "等待已取消"),
    top, ...(capture("release-hooks").length ? [noToast, ...capture("release-hooks")] : []),
  ]);
  const [task] = await api(path + "/push-tasks");
  assert.deepEqual(task.server_ids, ["1001", "1003"]);
  assert.equal(task.version, "1.0.0");
  assert.equal((await api(path + "/push-deployments")).length, 2, "Only the two previews were submitted");

  batch("节点、存储索引、交付总览、系统设置与指南", [
    ["click", tab("cluster")], text(tid("delivery-node-list"), "1002"),
    click("nav-agent"), text(tid("agent-host-list"), "game-node-a"), text(tid("agent-capabilities"), "/api/push/hosts/{hostID}/check"),
    click("nav-storage"), text(tid("storage-tree"), projectName), click("storage-reindex"),
    text(".toast", "索引已重建"), form("storage-search", fileName), text(tid("storage-search-hits"), fileName),
    ["click", `${tid("storage-search-hits")} button:first-of-type`], text(tid("storage-detail"), fileName),
    top, ...(capture("storage-space").length ? [noToast, ...capture("storage-space")] : []),
    click("nav-dashboard"), text(tid("dashboard-kpis"), "上传请求"), ["wait", `${tid("dashboard-chart")} svg`],
    text(tid("dashboard-records-table"), projectName),
    ["click", '[data-global="settings"]'], text("#userTable", "已隐藏"),
    ["click", '#settingsTabs [data-stab="audit"]'], ["wait", "#auditTable"],
    ["click", '#settingsTabs [data-stab="login-security"]'], ["wait", "#loginProtectionEnabled"],
    click("nav-guide"), text(tid("guide-topic-list"), "Gitea Actions 接入"),
  ]);

  const [errors] = batch("紧凑桌面、审批、失效深链和退出登录", [
    ["set", "viewport", "1280", "720"], ["open", `${route}/versions?version=${draft.version}`],
    text("#verDetailTitle", draft.version), noOverflow, noSidebarOverflow,
    ["wait", "--fn", "(() => { const selected = document.querySelector('.project-item.selected'); const list = document.querySelector('.project-list'); if (!selected || !list) return false; const row = selected.getBoundingClientRect(); const viewport = list.getBoundingClientRect(); return row.top >= viewport.top - 1 && row.bottom <= viewport.bottom + 1; })()"],
    ["wait", "--fn", "(() => { const tabs = document.querySelector('.project-tabs'); return tabs && tabs.scrollWidth <= tabs.clientWidth; })()"],
    text(tid("workspace-next"), "检查并发布"),
    ["click", tid("workspace-next")], ready("publish-flow-panel"), noOverflow,
    button("提交审批"), text(tid("publish-flow-panel"), "pending_review"),
    button("驳回"), ["click", 'dialog [data-modal="ok"]'], text(tid("publish-flow-panel"), "rejected"),
    ["open", `${route}/publish?version=9.9.9`], text(tid("publish-flow-panel"), "9.9.9 已删除或不可用"),
    ["wait", tid("publish-version"), "--state", "hidden"], ["select", "#publishVersionCheck", draft.version],
    ["reload"], text(tid("publish-flow-panel"), "rejected"), noOverflow, ...capture("desktop-publish"),
    button("退出"), ready("login-panel"),
    ["errors"],
  ]).slice(-1);
  assert.equal(errors?.errors?.length ?? (Array.isArray(errors) ? errors.length : 0), 0, "Browser page errors");
});
console.log(`Headless Express233 verification finished in ${((performance.now() - start) / 1000).toFixed(2)} s`);
