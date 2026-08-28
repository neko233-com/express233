const TOKEN_KEY = "express233_jwt";

const getToken = () => localStorage.getItem(TOKEN_KEY);
const setToken = (t) => {
  if (t) localStorage.setItem(TOKEN_KEY, t);
  else localStorage.removeItem(TOKEN_KEY);
};

const api = (path, opts = {}) => {
  const headers = { ...(opts.headers || {}) };
  const t = getToken();
  if (t) headers.Authorization = `Bearer ${t}`;
  return fetch(path, { credentials: "include", ...opts, headers }).then(async (r) => {
    if (r.status === 401) {
      setToken(null);
      showLogin();
      throw new Error("请先登录");
    }
    if (!r.ok) {
      throw new Error(await readErrorMessage(r));
    }
    if (r.status === 204) return null;
    return r.json();
  });
};

async function readErrorMessage(response) {
  const fallback = response.statusText || `HTTP ${response.status}`;
  const text = await response.text().catch(() => "");
  if (!text) return fallback;
  try {
    const j = JSON.parse(text);
    if (j.error) return String(j.error);
    if (j.message) return String(j.message);
  } catch (_) {}
  return text;
}

let state = {
  projects: [],
  projectFilter: "",
  projectId: null,
  projectName: null,
  username: null,
  versions: [],
  versionFilter: "",
  fileFilter: "",
  fileRows: [],
  version: null,
  versionStatus: null,
  isAdmin: false,
  isRoot: false,
  role: "viewer",
  tenantSlug: null,
  projectRole: null,
  projectMaxPublishedVersions: 0,
  pendingInviteToken: null,
  globalView: "workspace",
  projectTab: "versions",
  releaseSection: "tasks",
  serverEntries: [],
  serverConfigFilter: "",
  selectedServerID: null,
  previewConfirmed: false,
};

let fileTree = null;
let serverIDs = [];
let fileTreeModulePromise = null;
let filePreviewRequestID = 0;
const navigationHistory = [];
const recentUserTokens = new Map();
let navigationRestoring = false;
const collapsedFileTreeFolders = {
  version: new Set(),
  diff: new WeakMap(),
  storage: new Set(),
};

function showToast(message, type = "ok", timeout = 3200) {
  const host = document.getElementById("toastHost");
  if (!host) return;
  const el = document.createElement("div");
  el.className = `toast ${type}`;
  el.textContent = message;
  host.appendChild(el);
  window.setTimeout(() => el.remove(), timeout);
}

function showConfirm({ title = "确认操作", message = "", confirmText = "确认", cancelText = "取消", danger = false } = {}) {
  return showModal({ title, message, confirmText, cancelText, danger, mode: "confirm" });
}

function showPrompt({ title = "输入内容", message = "", value = "", confirmText = "保存", cancelText = "取消" } = {}) {
  return showModal({ title, message, value, confirmText, cancelText, mode: "prompt" });
}

function showModal({ title, message, value = "", confirmText, cancelText, danger = false, mode }) {
  const host = document.getElementById("modalHost");
  if (!host) return Promise.resolve(mode === "prompt" ? null : false);
  return new Promise((resolve) => {
    host.classList.remove("hidden");
    host.innerHTML = `<div class="modal-card" role="dialog" aria-modal="true" aria-labelledby="modalTitle">
      <h2 id="modalTitle" class="modal-title">${escapeHtml(title)}</h2>
      <p class="modal-message">${escapeHtml(message)}</p>
      ${mode === "prompt" ? `<input class="input modal-input" value="${escapeAttr(value)}" />` : ""}
      <div class="modal-actions">
        <button type="button" class="btn btn-secondary" data-modal="cancel">${escapeHtml(cancelText)}</button>
        <button type="button" class="btn ${danger ? "btn-danger" : "btn-primary"}" data-modal="ok">${escapeHtml(confirmText)}</button>
      </div>
    </div>`;
    const input = host.querySelector(".modal-input");
    const close = (result) => {
      host.classList.add("hidden");
      host.innerHTML = "";
      resolve(result);
    };
    host.querySelector("[data-modal='cancel']").onclick = () => close(mode === "prompt" ? null : false);
    host.querySelector("[data-modal='ok']").onclick = () => close(mode === "prompt" ? input.value : true);
    host.onclick = (e) => {
      if (e.target === host) close(mode === "prompt" ? null : false);
    };
    host.onkeydown = (e) => {
      if (e.key === "Escape") close(mode === "prompt" ? null : false);
      if (e.key === "Enter" && mode === "prompt") close(input.value);
    };
    if (input) {
      input.focus();
      input.select();
    } else {
      host.querySelector("[data-modal='ok']").focus();
    }
  });
}

function showLogin() {
  document.title = "express233 · 登录";
  document.getElementById("login").classList.remove("hidden");
  document.getElementById("app").classList.add("hidden");
}

function showApp(username) {
  try {
  document.title = "express233 · 发布控制台";
  state.username = username;
  document.getElementById("login").classList.add("hidden");
  document.getElementById("app").classList.remove("hidden");
  const who = document.getElementById("who");
  who.textContent = state.tenantSlug ? `${username} @ ${state.tenantSlug}` : username;
  const av = document.querySelector(".user-avatar");
  if (av && username) av.textContent = username.charAt(0).toUpperCase();
  if (state.isAdmin) {
    document.querySelectorAll(".admin-only").forEach((el) => el.classList.remove("hidden"));
    loadUsers();
    loadAuditLogs();
    if (state.isRoot) loadSystemUpdateStatus();
  }
  if (state.isAdmin || state.role === "operator") {
    document.querySelectorAll(".operator-only").forEach((el) => el.classList.remove("hidden"));
  }
  navigationHistory.length = 0;
  setGlobalView("workspace");
  loadProjects();
  loadServerYaml();
  loadServerIDs();
  parseInviteHash();
  scheduleOnboarding();
  } catch (showAppErr) {
    console.error("show app failed:", showAppErr);
    throw showAppErr;
  }
}

function navigationSnapshot() {
  return {
    globalView: state.globalView,
    projectTab: state.projectTab,
    releaseSection: state.releaseSection,
    projectId: state.projectId,
  };
}

function navigationKey(item) {
  return [item.globalView, item.projectTab, item.releaseSection, item.projectId || ""].join(":");
}

function navigationLabel(item) {
  const project = state.projects.find((entry) => entry.id === item.projectId)?.name || (item.projectId === state.projectId ? state.projectName : null);
  if (item.globalView === "workspace") {
    const tabs = { versions: "Artifact 版本", config: "Server 配置", preview: "合成预览", publish: "发布", diff: "差异", cluster: "集群节点", team: "团队", deploy: "部署脚本" };
    return project ? `项目工作区 · ${tabs[item.projectTab] || "项目"}` : "项目工作区";
  }
  if (item.globalView === "release") {
    const sections = { tasks: "发布任务", logs: "发布日志", hooks: "自动 Hook", ssh: "SSH 机器" };
    return `远程交付 · ${sections[item.releaseSection] || "发布任务"}`;
  }
  return { dashboard: "交付总览", guide: "文档目录", server: "高级配置", storage: "存储空间", settings: "系统设置" }[item.globalView] || "上一步";
}

function updateNavigationBackUI() {
  const bar = document.getElementById("navigationBackBar");
  const button = document.getElementById("btnNavigateBack");
  const current = document.getElementById("navigationCurrent");
  const previous = navigationHistory[navigationHistory.length - 1];
  if (!bar || !button) return;
  bar.classList.toggle("hidden", !previous);
  document.getElementById("navigationBackLabel").textContent = previous ? `返回 ${navigationLabel(previous)}` : "上一步";
  current.textContent = navigationLabel(navigationSnapshot());
  button.disabled = !previous;
}

function rememberNavigation() {
  if (navigationRestoring) return;
  const current = navigationSnapshot();
  const previous = navigationHistory[navigationHistory.length - 1];
  if (!previous || navigationKey(previous) !== navigationKey(current)) navigationHistory.push(current);
  if (navigationHistory.length > 50) navigationHistory.shift();
}

function navigateGlobalView(view) {
  if (view === state.globalView) return;
  rememberNavigation();
  setGlobalView(view);
}

function navigateProjectTab(tab) {
  if (state.globalView === "workspace" && state.projectTab === tab) return;
  rememberNavigation();
  setGlobalView("workspace");
  setProjectTab(tab);
}

function navigateReleaseSection(section = "tasks") {
  if (state.globalView === "release" && state.releaseSection === section) return;
  rememberNavigation();
  setGlobalView("release");
  setReleaseSection(section);
}

async function navigateBack() {
  const target = navigationHistory.pop();
  if (!target) return;
  navigationRestoring = true;
  try {
    if (target.projectId && target.projectId !== state.projectId) {
      const project = state.projects.find((entry) => entry.id === target.projectId);
      if (project) await selectProject(project, { remember: false });
    }
    state.projectTab = target.projectTab || "versions";
    state.releaseSection = target.releaseSection || "tasks";
    setGlobalView(target.globalView || "workspace");
    if (target.globalView === "workspace") setProjectTab(state.projectTab);
    if (target.globalView === "release") setReleaseSection(state.releaseSection);
  } finally {
    navigationRestoring = false;
    updateNavigationBackUI();
  }
}

function setGlobalView(view) {
  state.globalView = view;
  document.querySelectorAll(".sidebar-nav-item[data-global]").forEach((b) => {
    b.classList.toggle("active", b.dataset.global === view);
  });
  document.getElementById("globalServer").classList.toggle("hidden", view !== "server");
  document.getElementById("globalStorage").classList.toggle("hidden", view !== "storage");
  document.getElementById("globalSettings").classList.toggle("hidden", view !== "settings");
  document.getElementById("globalDashboard").classList.toggle("hidden", view !== "dashboard");
  document.getElementById("globalGuide").classList.toggle("hidden", view !== "guide");
  document.getElementById("globalRelease").classList.toggle("hidden", view !== "release");
  const inProject = view === "workspace" && state.projectId;
  document.getElementById("projectWorkspace").classList.toggle("hidden", !inProject);
  document.getElementById("emptyProject").classList.toggle("hidden", inProject || view !== "workspace");
  if (view === "settings" && state.isAdmin) {
    loadUsers();
    loadAuditLogs();
    loadLoginProtection();
    if (state.isRoot) loadSystemUpdateStatus();
  }
  if (view === "storage") loadStorageOverview();
  if (view === "dashboard") loadDashboard();
  if (view === "guide") loadGuideDirectory();
  if (view === "release") {
    loadReleaseWorkspace();
    setReleaseSection(state.releaseSection || "tasks");
  }
  updateNavigationBackUI();
}

function setProjectTab(tab) {
  if (tab === "hooks") {
    setGlobalView("release");
    setReleaseSection("hooks");
    return;
  }
  state.projectTab = tab;
  const workspace = document.getElementById("projectWorkspace");
  if (workspace) workspace.dataset.activeTab = tab;
  if (tab !== "pushlogs" && pushLogRefreshTimer) {
    window.clearTimeout(pushLogRefreshTimer);
    pushLogRefreshTimer = null;
  }
  if (tab !== "hooks" && releaseHookRefreshTimer) {
    window.clearTimeout(releaseHookRefreshTimer);
    releaseHookRefreshTimer = null;
  }
  document.querySelectorAll(".project-tab").forEach((b) => {
    b.classList.toggle("active", b.dataset.ptab === tab);
  });
  ["versions", "config", "preview", "publish", "cluster", "team", "deploy", "hooks", "diff"].forEach((t) => {
    const el = document.getElementById("ptab-" + t);
    if (el) el.classList.toggle("hidden", t !== tab);
  });
  if (tab === "deploy") generateDeployScript();
  if (tab === "config") loadServerConfigs();
  if (tab === "cluster") loadDeliveryNodes();
  if (tab === "hooks") loadReleaseHooks();
  updateWorkflowUI();
  updateNavigationBackUI();
}

function setReleaseSection(section) {
  if (section === "ssh" && !state.isAdmin) section = "tasks";
  if (section !== "hooks" && releaseHookRefreshTimer) {
    window.clearTimeout(releaseHookRefreshTimer);
    releaseHookRefreshTimer = null;
  }
  state.releaseSection = section;
  document.querySelectorAll("[data-release-section]").forEach((button) => button.classList.toggle("active", button.dataset.releaseSection === section));
  document.getElementById("releaseProjectContext")?.classList.toggle("hidden", section === "ssh");
  document.getElementById("globalAgent")?.classList.toggle("hidden", section !== "ssh");
  const ready = !!state.projectId;
  document.getElementById("releaseProjectEmpty")?.classList.toggle("hidden", section === "ssh" || ready);
  document.getElementById("releaseProjectContent")?.classList.toggle("hidden", section === "ssh" || !ready);
  document.getElementById("ptab-push")?.classList.toggle("hidden", section !== "tasks");
  document.getElementById("ptab-pushlogs")?.classList.toggle("hidden", section !== "logs");
  document.getElementById("ptab-hooks")?.classList.toggle("hidden", section !== "hooks" || !ready);
  if (section === "ssh" && state.isAdmin) loadAgentWorkspace();
  if (section === "hooks") loadReleaseHooks();
  updateNavigationBackUI();
}

function targetTag(serverID = state.selectedServerID) {
  return state.projectName && serverID ? `${state.projectName}|${serverID}` : "—";
}

function updateWorkflowUI() {
  const version = state.version || "选择 Artifact 版本";
  const selectedServer = state.selectedServerID;
  const count = (state.serverEntries || []).length;
  const flowVersion = document.getElementById("flowVersion");
  const flowServers = document.getElementById("flowServers");
  const flowServerDetail = document.getElementById("flowServerDetail");
  const flowPreview = document.getElementById("flowPreview");
  const flowPublish = document.getElementById("flowPublish");
  if (flowVersion) flowVersion.textContent = state.version ? `Version ${version} 已选择` : version;
  if (flowServers) flowServers.textContent = selectedServer ? `Server ${selectedServer} 已配置` : `${count} 个 Server 配置`;
  if (flowServerDetail) flowServerDetail.textContent = selectedServer ? targetTag(selectedServer) : (count ? "请选择交付目标" : "尚未配置");
  if (flowPreview) flowPreview.textContent = state.previewConfirmed ? "合成预览已确认" : "合成预览待确认";
  if (flowPublish) flowPublish.textContent = state.versionStatus === "published" ? `Version ${version} 已发布` : "尚未发布";
  const previewTag = document.getElementById("previewTargetTag");
  if (previewTag) previewTag.textContent = selectedServer ? targetTag(selectedServer) : "选择 Server ID";
  const publishProject = document.getElementById("publishProjectName");
  const publishVersion = document.getElementById("publishVersionName");
  const publishTarget = document.getElementById("publishTargetName");
  if (publishProject) publishProject.textContent = state.projectName || "—";
  if (publishVersion) publishVersion.textContent = state.version || "—";
  if (publishTarget) publishTarget.textContent = selectedServer ? targetTag(selectedServer) : "—";
  const versionCheck = document.getElementById("publishVersionCheck");
  const configCheck = document.getElementById("publishConfigCheck");
  const previewCheck = document.getElementById("publishPreviewCheck");
  if (versionCheck) versionCheck.textContent = state.version ? `Artifact ${state.version} 已选择` : "未选择 Artifact 版本";
  if (configCheck) configCheck.textContent = selectedServer ? `Server ${selectedServer} 已配置` : "未选择 Server 配置";
  if (previewCheck) previewCheck.textContent = state.previewConfirmed ? "预览已确认" : "预览尚未确认";
  const flowPublishButton = document.getElementById("btnFlowPublish");
  if (flowPublishButton) flowPublishButton.disabled = !(state.version && selectedServer && state.previewConfirmed) || state.versionStatus === "published";
  const latest = latestPublishedVersion();
  const latestVersion = document.getElementById("workspaceLatestVersion");
  const latestState = document.getElementById("workspaceLatestState");
  const workspaceTarget = document.getElementById("workspaceTarget");
  const workspaceTargetState = document.getElementById("workspaceTargetState");
  if (latestVersion) latestVersion.textContent = latest?.version || "—";
  if (latestState) latestState.textContent = latest ? "已发布 · 不可变制品" : "暂无已发布版本";
  if (workspaceTarget) workspaceTarget.textContent = selectedServer || "待选择";
  if (workspaceTargetState) workspaceTargetState.textContent = selectedServer ? targetTag(selectedServer) : "进入 Server 配置选择";
}

function setVersionStatusBadge(status) {
  const el = document.getElementById("verStatus");
  if (!el) return;
  el.textContent = status || "—";
  el.className = "badge";
  if (status === "published") el.classList.add("badge-ok");
  else if (status === "draft") el.classList.add("badge-draft");
  else if (status === "pending_review") el.classList.add("badge-warn");
}

async function init() {
  const saved = getToken();
  try {
    const me = await api("/api/me");
    if (me.token) setToken(me.token);
    state.isAdmin = me.is_admin;
    state.isRoot = !!me.is_root;
    state.role = me.role || (me.is_admin ? "admin" : "viewer");
    state.tenantSlug = me.tenant_slug || null;
    showApp(me.username);
  } catch (initErr) {
    if (saved) console.warn("saved session expired:", initErr);
    if (saved) setToken(null);
    showLogin();
  }
  await parseInviteHash();
}

async function loadServerIDs() {
  try {
    const d = await api("/api/server-ids");
    serverIDs = d.server_ids || [];
    document.querySelectorAll(".server-id-input").forEach(setupServerIDCombobox);
  } catch (_) {}
}

async function loadServerConfigs() {
  const list = document.getElementById("serverConfigList");
  if (!list) return;
  if (!state.isAdmin) {
    list.innerHTML = '<div class="agent-empty">Server 配置由系统管理员维护。</div>';
    return;
  }
  try {
    const payload = await api("/api/servers");
    state.serverEntries = payload.servers || [];
    renderServerConfigList();
    if (state.selectedServerID) {
      const current = state.serverEntries.find((item) => item.server_id === state.selectedServerID);
      if (current) selectServerConfig(current);
      else state.selectedServerID = null;
    }
    updateWorkflowUI();
  } catch (error) {
    list.innerHTML = `<div class="agent-error">${escapeHtml(error.message)}</div>`;
  }
}

function renderServerConfigList() {
  const list = document.getElementById("serverConfigList");
  if (!list) return;
  const query = state.serverConfigFilter;
  const rows = (state.serverEntries || []).filter((item) => {
    const haystack = `${item.server_id} ${targetTag(item.server_id)}`.toLowerCase();
    return !query || haystack.includes(query);
  });
  if (!rows.length) {
    list.innerHTML = `<div class="agent-empty">${query ? "没有匹配配置" : "还没有 Server 配置"}</div>`;
    return;
  }
  list.innerHTML = rows.map((item) => {
    const replacementCount = Object.keys(item.entry?.replacements || {}).length;
    return `<button type="button" class="server-config-row ${item.server_id === state.selectedServerID ? "active" : ""}" data-server-config="${escapeAttr(item.server_id)}"><span><strong>${escapeHtml(item.server_id)}</strong><code>${escapeHtml(targetTag(item.server_id))}</code></span><small>${replacementCount} 个模板文件</small></button>`;
  }).join("");
  list.querySelectorAll("[data-server-config]").forEach((button) => {
    button.onclick = () => selectServerConfig(state.serverEntries.find((item) => item.server_id === button.dataset.serverConfig));
  });
}

function selectServerConfig(item) {
  if (!item) return;
  state.selectedServerID = item.server_id;
  state.previewConfirmed = false;
  document.getElementById("serverConfigEmpty")?.classList.add("hidden");
  document.getElementById("serverConfigEditor")?.classList.remove("hidden");
  document.getElementById("serverConfigTitle").textContent = item.server_id;
  document.getElementById("serverConfigTarget").textContent = targetTag(item.server_id);
  document.getElementById("configSourceVersion").textContent = state.version || "未选版本";
  document.getElementById("configSourceServer").textContent = item.server_id;
  const editor = document.getElementById("serverConfigJSON");
  editor.value = JSON.stringify(item.entry || { replacements: {} }, null, 2);
  document.getElementById("serverConfigSaveState").textContent = "已保存";
  const files = Object.entries(item.entry?.replacements || {});
  document.getElementById("serverReplacementFiles").innerHTML = files.length ? files.map(([name, values], index) => `<button type="button" class="replacement-file-row ${index === 0 ? "active" : ""}"><strong>${escapeHtml(name)}</strong><small>${Object.keys(values || {}).length} 个覆盖项</small></button>`).join("") : '<div class="agent-empty">尚无覆盖项，在右侧 JSON 中添加 replacements。</div>';
  const previewInput = document.getElementById("verPreviewServerId");
  if (previewInput) previewInput.value = item.server_id;
  renderServerConfigList();
  updateWorkflowUI();
}

async function createServerConfig(copyFrom = null) {
  const serverID = await showPrompt({ title: copyFrom ? "复制 Server 配置" : "新建 Server 配置", message: "输入 Server ID；目标标签将自动生成 project|serverId。", value: copyFrom ? `${copyFrom.server_id}-copy` : "" });
  if (serverID == null) return;
  const clean = serverID.trim();
  if (!clean || clean.includes("|")) return showToast("Server ID 不能为空且不能包含 |", "warn");
  if (state.serverEntries.some((item) => item.server_id === clean)) return showToast("Server ID 已存在", "warn");
  const entry = copyFrom ? copyFrom.entry : { replacements: {}, post_hook: "", post_hook_env: {} };
  try {
    const created = await api(`/api/servers/${encodeURIComponent(clean)}`, { method: "PUT", headers: { "Content-Type": "application/json" }, body: JSON.stringify(entry) });
    state.selectedServerID = created.server_id;
    await loadServerIDs();
    await loadServerConfigs();
    showToast(copyFrom ? "配置已复制" : "Server 配置已创建");
  } catch (error) { showToast(error.message, "err"); }
}

async function saveServerConfig() {
  if (!state.selectedServerID) return;
  let entry;
  try { entry = JSON.parse(document.getElementById("serverConfigJSON").value); }
  catch (error) { showToast(`JSON 格式错误：${error.message}`, "err"); return; }
  try {
    await api(`/api/servers/${encodeURIComponent(state.selectedServerID)}`, { method: "PUT", headers: { "Content-Type": "application/json" }, body: JSON.stringify(entry) });
    document.getElementById("serverConfigSaveState").textContent = "已保存";
    await loadServerIDs();
    await loadServerConfigs();
    navigateProjectTab("preview");
    scheduleDeployPreview();
    showToast("Server 配置已保存");
  } catch (error) { showToast(error.message, "err"); }
}

async function deleteServerConfig() {
  if (!state.selectedServerID) return;
  if (!await showConfirm({ title: "删除 Server 配置", message: `删除 ${targetTag()}？SSH 绑定不会自动删除。`, danger: true })) return;
  try {
    await api(`/api/servers/${encodeURIComponent(state.selectedServerID)}`, { method: "DELETE" });
    state.selectedServerID = null;
    document.getElementById("serverConfigEditor")?.classList.add("hidden");
    document.getElementById("serverConfigEmpty")?.classList.remove("hidden");
    await loadServerIDs();
    await loadServerConfigs();
    showToast("Server 配置已删除");
  } catch (error) { showToast(error.message, "err"); }
}

function setupServerIDCombobox(input) {
  if (input.dataset.serverIdReady) return;
  input.dataset.serverIdReady = "1";
  const box = input.closest(".server-id-combobox");
  const options = box?.querySelector(".server-id-options");
  if (!options) return;
  let active = -1;
  const render = () => {
    const q = input.value.trim().toLowerCase();
    const rows = serverIDs.filter((id) => id.toLowerCase().includes(q)).slice(0, 50);
    active = -1;
    options.innerHTML = rows.map((id) => {
      const at = id.toLowerCase().indexOf(q);
      const label = !q ? escapeHtml(id) : `${escapeHtml(id.slice(0, at))}<mark>${escapeHtml(id.slice(at, at + q.length))}</mark>${escapeHtml(id.slice(at + q.length))}`;
      return `<button type="button" class="server-id-option" data-server-id="${escapeAttr(id)}">${label}</button>`;
    }).join("") || '<div class="server-id-empty">无匹配 server_id</div>';
    options.classList.toggle("hidden", !rows.length && !q);
    options.querySelectorAll("[data-server-id]").forEach((button) => button.onclick = () => { input.value = button.dataset.serverId; input.dispatchEvent(new Event("input", { bubbles: true })); options.classList.add("hidden"); input.focus(); });
  };
  input.onfocus = render;
  input.addEventListener("input", render);
  input.addEventListener("keydown", (event) => {
    const rows = [...options.querySelectorAll("[data-server-id]")];
    if (event.key === "ArrowDown" || event.key === "ArrowUp") { event.preventDefault(); active = Math.max(0, Math.min(rows.length - 1, active + (event.key === "ArrowDown" ? 1 : -1))); rows.forEach((r, i) => r.classList.toggle("active", i === active)); rows[active]?.focus(); }
    if (event.key === "Escape") options.classList.add("hidden");
  });
  document.addEventListener("click", (event) => { if (!box.contains(event.target)) options.classList.add("hidden"); });
}

async function loadReleaseWorkspace() {
  const select = document.getElementById("releaseProject");
  if (!select) return;
  const selected = String(state.projectId || "");
  select.innerHTML = '<option value="">选择项目</option>' + state.projects.map((p) => `<option value="${p.id}">${escapeHtml(p.name)}</option>`).join("");
  select.value = selected;
  const ready = !!state.projectId;
  const cards = document.getElementById("releaseProjectCards");
  if (cards) {
    cards.innerHTML = state.projects.map((p) => `<button type="button" class="btn btn-secondary" data-release-project="${p.id}">${escapeHtml(p.name)}</button>`).join("") || '<span class="hint">暂无可访问项目</span>';
    cards.querySelectorAll("[data-release-project]").forEach((button) => button.onclick = async () => {
      document.getElementById("releaseProject").value = button.dataset.releaseProject;
      document.getElementById("releaseProject").dispatchEvent(new Event("change"));
    });
  }
  setReleaseSection(state.releaseSection || "tasks");
  if (ready) { await loadPushTasks(); await loadPushLogs(); }
}

function updateDeployCmd() {
  // now handled by generateDeployScript()
}

let deployOS = "linux";

document.querySelectorAll("#deployOsTabs .seg-tab").forEach((btn) => {
  btn.onclick = () => {
    document.querySelectorAll("#deployOsTabs .seg-tab").forEach((b) => b.classList.remove("active"));
    btn.classList.add("active");
    deployOS = btn.dataset.os;
    generateDeployScript();
  };
});

["deployServerId", "deployToken", "deployTempDir"].forEach((id) => {
  document.getElementById(id)?.addEventListener("input", generateDeployScript);
});

function generateDeployScript() {
  const el = document.getElementById("deployCmd");
  if (!el) return;
  const sid = document.getElementById("deployServerId")?.value.trim() || "<server_id>";
  const token = document.getElementById("deployToken")?.value.trim() || "<your_pull_token>";
  const tmpDir = document.getElementById("deployTempDir")?.value.trim() || "";
  const project = state.projectName || "<project>";
  const version = state.version || "";
  const central = window.location.origin;
  const verFlag = version ? ` --version ${version}` : "";

  if (deployOS === "linux") {
    const tmp = tmpDir || `/tmp/express233-staging-${sid}`;
    el.textContent = `#!/bin/bash
set -euo pipefail
# express233 一键部署脚本 — ${project} / ${sid}
# 生成时间: ${new Date().toISOString().slice(0, 10)}

EXPRESS233_SERVER="${central}"
EXPRESS233_TOKEN="${token}"
PROJECT="${project}"
SERVER_ID="${sid}"
STAGING_DIR="${tmp}"
GAME_ROOT="\${GAME_ROOT:-/opt/game-servers}"
FINAL_DIR="\${GAME_ROOT}/\${SERVER_ID}"

# 1. 检查并安装 express233-cli
if ! command -v express233-cli &>/dev/null; then
  echo "[install] downloading express233-cli..."
  curl -fsSL "\${EXPRESS233_SERVER}/cli/install.sh" | bash 2>/dev/null \\
    || { echo "请手动安装: https://github.com/neko233-com/express233/releases"; exit 1; }
fi
echo "[ok] express233-cli $(express233-cli version 2>/dev/null || echo dev)"

# 2. 拉取到临时目录
echo "[pull] \${PROJECT} → \${STAGING_DIR}"
rm -rf "\${STAGING_DIR}"
mkdir -p "\${STAGING_DIR}"
express233-cli pull \\
  --server "\${EXPRESS233_SERVER}" \\
  --token "\${EXPRESS233_TOKEN}" \\
  --project "\${PROJECT}" \\
  --server-id "\${SERVER_ID}"${verFlag} \\
  --dest "\${STAGING_DIR}" \\
  --skip-hook

# 3. 停止旧服务
PID_FILE="\${FINAL_DIR}/run/server.pid"
if [ -f "\${PID_FILE}" ]; then
  OLD_PID=$(cat "\${PID_FILE}")
  if kill -0 "\${OLD_PID}" 2>/dev/null; then
    echo "[stop] killing PID \${OLD_PID}..."
    kill "\${OLD_PID}" 2>/dev/null || true
    for i in $(seq 1 10); do kill -0 "\${OLD_PID}" 2>/dev/null || break; sleep 1; done
    kill -0 "\${OLD_PID}" 2>/dev/null && kill -9 "\${OLD_PID}" 2>/dev/null
  fi
  rm -f "\${PID_FILE}"
fi

# 4. 替换文件（保留 logs/ 和 run/）
mkdir -p "\${FINAL_DIR}/logs" "\${FINAL_DIR}/run"
if command -v rsync &>/dev/null; then
  rsync -a --delete --exclude='logs/' --exclude='run/' "\${STAGING_DIR}/" "\${FINAL_DIR}/"
else
  find "\${FINAL_DIR}" -mindepth 1 -maxdepth 1 ! -name logs ! -name run -exec rm -rf {} +
  cp -a "\${STAGING_DIR}/"* "\${FINAL_DIR}/" 2>/dev/null || true
fi

# 5. 启动新服务
if [ -f "\${FINAL_DIR}/scripts/restart.sh" ]; then
  chmod +x "\${FINAL_DIR}/scripts/restart.sh"
  SERVER_ID="\${SERVER_ID}" bash "\${FINAL_DIR}/scripts/restart.sh"
fi

# 6. 清理
rm -rf "\${STAGING_DIR}"
echo "[done] ${sid} deployed to \${FINAL_DIR}"`;
  } else {
    const tmp = tmpDir || `$env:TEMP\\express233-staging-${sid}`;
    el.textContent = `# express233 一键部署脚本 — ${project} / ${sid}
# 保存为 deploy-${sid}.ps1 执行
$ErrorActionPreference = "Stop"

$EXPRESS233_SERVER = "${central}"
$EXPRESS233_TOKEN  = "${token}"
$PROJECT           = "${project}"
$SERVER_ID         = "${sid}"
$STAGING_DIR       = "${tmp}"
$GAME_ROOT         = if ($env:GAME_ROOT) { $env:GAME_ROOT } else { "C:\\game-servers" }
$FINAL_DIR         = Join-Path $GAME_ROOT $SERVER_ID

# 1. 检查并安装 express233-cli
if (-not (Get-Command express233-cli -ErrorAction SilentlyContinue)) {
  Write-Host "[install] downloading express233-cli..." -ForegroundColor Cyan
  try {
    Invoke-Expression ((Invoke-WebRequest -Uri "https://raw.githubusercontent.com/neko233-com/express233/main/scripts/install.ps1" -UseBasicParsing).Content)
    $env:PATH += ";$(Join-Path $env:LOCALAPPDATA "express233")"
  } catch {
    Write-Host "请手动安装: https://github.com/neko233-com/express233/releases" -ForegroundColor Red
    exit 1
  }
}
Write-Host "[ok] express233-cli $(express233-cli version)" -ForegroundColor Green

# 2. 拉取到临时目录
Write-Host "[pull] $PROJECT -> $STAGING_DIR" -ForegroundColor Cyan
if (Test-Path $STAGING_DIR) { Remove-Item $STAGING_DIR -Recurse -Force }
New-Item -ItemType Directory -Path $STAGING_DIR -Force | Out-Null

$pullArgs = @(
  "pull",
  "--server", $EXPRESS233_SERVER,
  "--token", $EXPRESS233_TOKEN,
  "--project", $PROJECT,
  "--server-id", $SERVER_ID,
  "--dest", $STAGING_DIR,
  "--skip-hook"
)${version ? `\n$pullArgs += @("--version", "${version}")` : ""}
& express233-cli @pullArgs

# 3. 停止旧服务
$PID_FILE = Join-Path $FINAL_DIR "run\\server.pid"
if (Test-Path $PID_FILE) {
  $oldPid = Get-Content $PID_FILE
  $proc = Get-Process -Id $oldPid -ErrorAction SilentlyContinue
  if ($proc) {
    Write-Host "[stop] stopping PID $oldPid..." -ForegroundColor Yellow
    Stop-Process -Id $oldPid -Force -ErrorAction SilentlyContinue
    Start-Sleep -Seconds 2
  }
  Remove-Item $PID_FILE -Force -ErrorAction SilentlyContinue
}

# 4. 替换文件（保留 logs/ 和 run/）
$logDir = Join-Path $FINAL_DIR "logs"
$runDir = Join-Path $FINAL_DIR "run"
New-Item -ItemType Directory -Path $logDir, $runDir -Force | Out-Null

# 删除旧文件（保留 logs/ run/）
Get-ChildItem $FINAL_DIR -Exclude "logs","run" | Remove-Item -Recurse -Force
# 复制新文件
Copy-Item -Path (Join-Path $STAGING_DIR "*") -Destination $FINAL_DIR -Recurse -Force

# 5. 启动新服务
$restartScript = Join-Path $FINAL_DIR "scripts\\restart.ps1"
if (Test-Path $restartScript) {
  $env:SERVER_ID = $SERVER_ID
  & $restartScript
}

# 6. 清理
Remove-Item $STAGING_DIR -Recurse -Force -ErrorAction SilentlyContinue
Write-Host "[done] ${sid} deployed to $FINAL_DIR" -ForegroundColor Green`;
  }
}

document.getElementById("btnCopyDeploy")?.addEventListener("click", () => {
  const t = document.getElementById("deployCmd")?.textContent;
  if (t) navigator.clipboard.writeText(t);
});

document.getElementById("btnDownloadDeploy")?.addEventListener("click", () => {
  const content = document.getElementById("deployCmd")?.textContent;
  if (!content) return;
  const ext = deployOS === "linux" ? "sh" : "ps1";
  const sid = document.getElementById("deployServerId")?.value.trim() || "deploy";
  const blob = new Blob([content], { type: "text/plain" });
  const a = document.createElement("a");
  a.href = URL.createObjectURL(blob);
  a.download = `deploy-\${sid}.\${ext}`;
  a.click();
  URL.revokeObjectURL(a.href);
});

document.getElementById("btnLogin").onclick = async () => {
  try {
    const u = document.getElementById("user").value;
    const p = document.getElementById("pass").value;
    const me = await api("/api/login", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ username: u, password: p }),
    });
    if (me.token) setToken(me.token);
    state.isAdmin = me.is_admin;
    state.isRoot = !!me.is_root;
    state.role = me.role || (me.is_admin ? "admin" : "viewer");
    state.tenantSlug = me.tenant_slug || null;
    showApp(me.username);
  } catch (e) {
    document.getElementById("loginErr").textContent = e.message;
  }
};

document.getElementById("btnLogout").onclick = async () => {
  try {
    await api("/api/logout", { method: "POST" });
  } catch (_) {}
  setToken(null);
  location.href = "/";
};

document.querySelectorAll(".sidebar-nav-item[data-global]").forEach((btn) => {
  btn.onclick = () => {
    navigateGlobalView(btn.dataset.global);
  };
});

document.getElementById("btnNavigateBack")?.addEventListener("click", navigateBack);
document.getElementById("btnReturnProject")?.addEventListener("click", () => navigateGlobalView("workspace"));

document.getElementById("releaseProject")?.addEventListener("change", async (event) => {
  const project = state.projects.find((item) => item.id === Number(event.target.value));
  if (!project) return loadReleaseWorkspace();
  if (project.id !== state.projectId) rememberNavigation();
  state.projectId = project.id;
  state.projectName = project.name;
  state.versions = (await api(`/api/projects/${project.id}/versions`)) || [];
  await loadReleaseWorkspace();
});
document.getElementById("btnReleaseReload")?.addEventListener("click", loadReleaseWorkspace);

const releaseHookPanel = document.getElementById("ptab-hooks");
if (releaseHookPanel) document.getElementById("globalRelease")?.appendChild(releaseHookPanel);
const agentPanel = document.getElementById("globalAgent");
if (agentPanel) document.getElementById("globalRelease")?.appendChild(agentPanel);

document.querySelectorAll(".project-tab").forEach((btn) => {
  btn.onclick = () => btn.dataset.ptab === "hooks" ? navigateReleaseSection("hooks") : navigateProjectTab(btn.dataset.ptab);
});

document.querySelectorAll("[data-flow-target]").forEach((btn) => {
  btn.addEventListener("click", () => navigateProjectTab(btn.dataset.flowTarget));
});

document.querySelectorAll("[data-project-action='release']").forEach((btn) => {
  btn.addEventListener("click", () => {
    navigateReleaseSection(btn.dataset.releaseTarget || "tasks");
  });
});
document.querySelectorAll("[data-release-section]").forEach((button) => {
  button.addEventListener("click", () => navigateReleaseSection(button.dataset.releaseSection));
});

document.getElementById("btnPushAddHost")?.addEventListener("click", async () => {
  try {
    const authMode = document.getElementById("pushHostAuth").value;
    await api("/api/push/hosts", { method: "POST", headers: { "Content-Type": "application/json" }, body: JSON.stringify({
      name: document.getElementById("pushHostName").value.trim(), address: document.getElementById("pushHostAddress").value.trim(),
      port: Number(document.getElementById("pushHostPort").value || 22), username: document.getElementById("pushHostUser").value.trim(), auth_mode: authMode,
      credential: document.getElementById("pushHostCredential").value, host_key: document.getElementById("pushHostKey").value.trim(),
      health_check_enabled: document.getElementById("pushHostHealthEnabled").checked,
      health_check_interval_seconds: Number(document.getElementById("pushHostHealthInterval").value || 3600),
    }) });
    ["pushHostName", "pushHostAddress", "pushHostUser", "pushHostCredential", "pushHostKey"].forEach((id) => { document.getElementById(id).value = ""; });
    document.getElementById("pushHostEditor")?.classList.add("hidden");
    await loadAgentWorkspace(); showToast("SSH 资源已保存，凭据不可回读");
  } catch (e) { showToast(e.message, "err"); }
});
document.getElementById("btnPushAddBinding")?.addEventListener("click", async () => {
  if (!selectedPushHostID) return showToast("请先选择 SSH 配置", "err");
  const projectName = document.getElementById("pushBindingProject").value;
  if (!projectName) return showToast("请选择项目", "warn");
  try {
    await api(`/api/push/hosts/${selectedPushHostID}/servers${editingPushBindingID ? `/${editingPushBindingID}` : ""}`, { method: editingPushBindingID ? "PUT" : "POST", headers: { "Content-Type": "application/json" }, body: JSON.stringify({
      project_name: projectName, server_id: document.getElementById("pushBindingServerID").value.trim(), labels: document.getElementById("pushBindingLabels").value.trim(),
      content_tags: document.getElementById("pushBindingContentTags").value.trim(), remote_root: document.getElementById("pushBindingRoot").value.trim(), os: "linux", arch: "amd64",
    }) });
    editingPushBindingID = null;
    pushBindings = await api(`/api/push/hosts/${selectedPushHostID}/servers`);
    const selectedHost = pushHosts.find((host) => host.id === selectedPushHostID);
    if (selectedHost) selectedHost.bindings = pushBindings;
    renderPushBindings();
    renderPushHosts();
    showToast("服务器绑定已保存");
  } catch (e) { showToast(e.message, "err"); }
});

function updatePushBindingTargetTag() {
  const projectName = document.getElementById("pushBindingProject")?.value || "";
  const serverID = document.getElementById("pushBindingServerID")?.value.trim() || "";
  const output = document.getElementById("pushBindingTargetTag");
  if (output) output.textContent = projectName && serverID ? `${projectName}|${serverID}` : "选择项目与 Server ID";
}

document.getElementById("pushBindingProject")?.addEventListener("change", updatePushBindingTargetTag);
document.getElementById("pushBindingServerID")?.addEventListener("input", updatePushBindingTargetTag);
document.getElementById("btnShowPushHostEditor")?.addEventListener("click", () => document.getElementById("pushHostEditor")?.classList.remove("hidden"));
document.getElementById("btnClosePushHostEditor")?.addEventListener("click", () => document.getElementById("pushHostEditor")?.classList.add("hidden"));
document.getElementById("btnCloseBindingDrawer")?.addEventListener("click", () => document.getElementById("sshBindingDrawer")?.classList.add("hidden"));
["sshSearch", "sshStatusFilter", "sshProjectFilter"].forEach((id) => document.getElementById(id)?.addEventListener("input", renderPushHosts));
document.getElementById("btnSavePushTask")?.addEventListener("click", savePushTask);
document.getElementById("btnSaveAndRunPushTask")?.addEventListener("click", async () => {
  document.getElementById("pushTaskServerIDs").value = "";
  const task = await savePushTask();
  if (task) await runPushTask(task.id, false);
});
document.getElementById("btnCancelPushTask")?.addEventListener("click", resetPushTaskEditor);
document.getElementById("btnSaveReleaseHook")?.addEventListener("click", saveReleaseHook);
document.getElementById("btnCancelReleaseHook")?.addEventListener("click", resetReleaseHookEditor);
document.getElementById("btnReloadReleaseHooks")?.addEventListener("click", loadReleaseHooks);
document.getElementById("btnReloadPushLogs")?.addEventListener("click", loadPushLogs);
document.getElementById("btnReloadDeliveryNodes")?.addEventListener("click", loadDeliveryNodes);

document.querySelectorAll("#settingsTabs .seg-tab").forEach((btn) => {
  btn.onclick = () => {
    document.querySelectorAll("#settingsTabs .seg-tab").forEach((b) => b.classList.remove("active"));
    btn.classList.add("active");
    document.getElementById("stab-users").classList.toggle("hidden", btn.dataset.stab !== "users");
    document.getElementById("stab-audit").classList.toggle("hidden", btn.dataset.stab !== "audit");
    document.getElementById("stab-login-security").classList.toggle("hidden", btn.dataset.stab !== "login-security");
    document.getElementById("stab-onboarding").classList.toggle("hidden", btn.dataset.stab !== "onboarding");
    if (btn.dataset.stab === "login-security") loadLoginProtection();
  };
});

document.getElementById("projectSearch")?.addEventListener("input", (e) => {
  state.projectFilter = e.target.value.trim().toLowerCase();
  renderProjectList();
});

document.getElementById("versionSearch")?.addEventListener("input", (e) => {
  state.versionFilter = e.target.value.trim().toLowerCase();
  renderVersionList();
});

document.querySelectorAll("input[name='versionRetentionMode']").forEach((input) => {
  input.addEventListener("change", updateVersionRetentionHint);
});
document.getElementById("versionRetentionCount")?.addEventListener("input", updateVersionRetentionHint);
document.getElementById("btnSaveVersionRetention")?.addEventListener("click", () => {
  saveVersionRetention().catch((error) => showToast(error.message, "error"));
});

document.getElementById("fileSearch")?.addEventListener("input", (e) => {
  state.fileFilter = e.target.value.trim().toLowerCase();
  renderFileList();
});

document.getElementById("serverConfigSearch")?.addEventListener("input", (event) => {
  state.serverConfigFilter = event.target.value.trim().toLowerCase();
  renderServerConfigList();
});
document.getElementById("btnNewServerConfig")?.addEventListener("click", () => createServerConfig());
document.getElementById("btnDuplicateServerConfig")?.addEventListener("click", () => createServerConfig(state.serverEntries.find((item) => item.server_id === state.selectedServerID)));
document.getElementById("btnSaveServerConfig")?.addEventListener("click", saveServerConfig);
document.getElementById("btnDeleteServerConfig")?.addEventListener("click", deleteServerConfig);
document.getElementById("btnOpenRawYaml")?.addEventListener("click", () => navigateGlobalView("server"));
document.getElementById("serverConfigJSON")?.addEventListener("input", () => {
  const stateLabel = document.getElementById("serverConfigSaveState");
  if (stateLabel) stateLabel.textContent = "未保存";
});

function renderProjectList() {
  const ul = document.getElementById("projectList");
  if (!ul) return;
  ul.innerHTML = "";
  const q = state.projectFilter;
  state.projects
    .filter((p) => !q || p.name.toLowerCase().includes(q))
    .forEach((p) => {
      const li = document.createElement("li");
      li.className = "project-item";
      li.setAttribute("role", "button");
      li.setAttribute("tabindex", "0");
      li.setAttribute("title", p.name);
      li.innerHTML = `<svg class="project-item-icon" aria-hidden="true" viewBox="0 0 24 24"><path d="M3 7a2 2 0 0 1 2-2h5l2 2h7a2 2 0 0 1 2 2v8a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2V7Z"/></svg><span class="project-item-name">${escapeHtml(p.name)}</span>`;
      li.onclick = () => selectProject(p);
      li.onkeydown = (event) => {
        if (event.key === "Enter" || event.key === " ") {
          event.preventDefault();
          selectProject(p);
        }
      };
      if (p.id === state.projectId) {
        li.classList.add("selected");
        li.setAttribute("aria-current", "page");
      }
      ul.appendChild(li);
    });
}

async function loadProjects() {
  state.projects = (await api("/api/projects")) || [];
  renderProjectList();
  populateDashboardProjects();
  return state.projects;
}

function canWriteProject() {
  return state.projectRole === "admin" || state.isAdmin;
}

function updateProjectWriteUI() {
  const w = canWriteProject();
  document.querySelectorAll(".project-write").forEach((el) => {
    el.classList.toggle("hidden", !w);
    if (el.tagName === "INPUT" || el.tagName === "BUTTON") el.disabled = !w;
  });
  const fileInput = document.getElementById("fileInput");
  if (fileInput) fileInput.disabled = !w;
  ["versionRetentionLimited", "versionRetentionUnlimited", "versionRetentionCount", "btnSaveVersionRetention"].forEach((id) => {
    const element = document.getElementById(id);
    if (element) element.disabled = !w;
  });
}

async function loadProjectTeam() {
  const box = document.getElementById("projectTeam");
  if (!box || !state.projectId) return;
  try {
    const members = await api(`/api/projects/${state.projectId}/members`);
    const ul = document.getElementById("memberList");
    ul.innerHTML = members
      .map((m) => `<li>${escapeHtml(m.username)} <span class="badge">${escapeHtml(m.role)}</span></li>`)
      .join("");
  } catch (e) {
    console.warn(e);
  }
}

async function selectProject(p, { remember = true } = {}) {
  const projectChanged = state.projectId !== p.id;
  if (remember && (projectChanged || state.globalView !== "workspace")) rememberNavigation();
  state.projectId = p.id;
  state.projectName = p.name;
  state.projectRole = p.my_role || (state.isAdmin ? "admin" : "viewer");
  state.projectMaxPublishedVersions = Number(p.max_published_versions || 0);
  state.version = null;
  state.versionStatus = null;
  if (projectChanged) {
    state.selectedServerID = null;
    state.previewConfirmed = false;
  }
  document.getElementById("curProject").textContent = p.name;
  const roleBadge = document.getElementById("projectRoleBadge");
  if (roleBadge) roleBadge.textContent = state.projectRole === "admin" ? "管理员" : "只读成员";
  document.getElementById("btnDelProject")?.classList.toggle("hidden", !canWriteProject());
  updateProjectWriteUI();
  loadProjectTeam();
  setGlobalView("workspace");
  setProjectTab(state.projectTab || "versions");
  await loadProjects();
  const refreshedProject = state.projects.find((item) => item.id === p.id);
  if (refreshedProject) state.projectMaxPublishedVersions = Number(refreshedProject.max_published_versions || 0);
  state.versions = (await api(`/api/projects/${p.id}/versions`)) || [];
  renderVersionList();
  renderVersionRetention();
  renderVersionActivity();
  if (state.isAdmin) await loadServerConfigs();
  if (state.projectTab === "cluster") await loadDeliveryNodes();
  // Populate diff dropdowns
  populateDiffDropdowns(state.versions);
  // Generate deploy script when switching to deploy tab
  generateDeployScript();
  const preferredVersion = latestPublishedVersion() || state.versions[0];
  if (preferredVersion) await selectVersion(preferredVersion);
  else document.getElementById("versionDetail").classList.add("hidden");
  updateWorkflowUI();
  return state.versions;
}

function renderVersionList() {
  const ul = document.getElementById("versionList");
  if (!ul) return;
  ul.innerHTML = "";
  const q = state.versionFilter;
  const latestPublished = latestPublishedVersion();
  (state.versions || [])
    .filter((v) => !q || v.version.toLowerCase().includes(q) || v.status.toLowerCase().includes(q))
    .forEach((v) => {
    const li = document.createElement("li");
    const status = versionStatusMeta(v.status);
    li.className = "version-item";
    if (v.version === latestPublished?.version) li.classList.add("latest-published");
    li.setAttribute("title", `${v.version} · ${status.label}`);
    const activityAt = v.published_at || v.created_at || "";
    li.innerHTML = `<span class="version-item-main"><span class="version-status-dot ${status.className}" aria-hidden="true"></span><span class="version-copy"><span><span class="version-name">${escapeHtml(v.version)}</span>${v.version === latestPublished?.version ? '<span class="latest-version-label">最新</span>' : ""}</span><small>${escapeHtml(v.published_at ? `发布于 ${activityAt}` : `创建于 ${activityAt}`)}</small></span></span><span class="version-status ${status.className}">${status.label}<span class="sr-only"> ${escapeHtml(v.status)}</span></span>`;
    li.onclick = () => selectVersion(v);
    if (v.version === state.version) li.classList.add("selected");
    ul.appendChild(li);
  });
}

function renderVersionRetention() {
  const maxPublishedVersions = Number(state.projectMaxPublishedVersions || 0);
  const limited = maxPublishedVersions > 0;
  const limitedRadio = document.getElementById("versionRetentionLimited");
  const unlimitedRadio = document.getElementById("versionRetentionUnlimited");
  const countInput = document.getElementById("versionRetentionCount");
  const badge = document.getElementById("versionRetentionBadge");
  if (limitedRadio) limitedRadio.checked = limited;
  if (unlimitedRadio) unlimitedRadio.checked = !limited;
  if (countInput) {
    if (limited) countInput.value = String(maxPublishedVersions);
    countInput.disabled = !limited || !canWriteProject();
  }
  if (badge) {
    badge.textContent = limited ? `最近 ${maxPublishedVersions} 个` : "无限保留";
    badge.className = limited ? "badge badge-ok" : "badge badge-draft";
  }
  updateVersionRetentionHint();
}

function updateVersionRetentionHint() {
  const hint = document.getElementById("versionRetentionHint");
  const countInput = document.getElementById("versionRetentionCount");
  const limited = !!document.getElementById("versionRetentionLimited")?.checked;
  if (countInput) countInput.disabled = !limited || !canWriteProject();
  if (!hint) return;
  if (!limited) {
    hint.textContent = "所有已发布版本都会保留，草稿不受影响。";
    return;
  }
  const count = Math.max(1, Number(countInput?.value || 1));
  hint.textContent = `保存后立即清理更早的已发布版本，仅保留最近 ${count} 个；草稿不受影响。`;
}

function renderVersionActivity() {
  const list = document.getElementById("versionActivityList");
  const summary = document.getElementById("versionActivitySummary");
  if (!list || !summary) return;
  const versions = state.versions || [];
  const publishedCount = versions.filter((version) => version.status === "published").length;
  summary.textContent = `${versions.length} 个版本 · ${publishedCount} 个已发布`;
  if (!versions.length) {
    list.innerHTML = '<li class="version-activity-empty">还没有版本活动</li>';
    return;
  }
  list.innerHTML = versions.slice(0, 5).map((version) => {
    const status = versionStatusMeta(version.status);
    const time = version.published_at || version.created_at || "—";
    return `<li><span class="activity-dot ${status.className}" aria-hidden="true"></span><div><strong>${escapeHtml(status.label)} ${escapeHtml(version.version)}</strong><small>${escapeHtml(time)}</small></div></li>`;
  }).join("");
}

async function saveVersionRetention() {
  if (!state.projectId || !canWriteProject()) return;
  const limited = !!document.getElementById("versionRetentionLimited")?.checked;
  const count = limited ? Number(document.getElementById("versionRetentionCount")?.value || 0) : 0;
  if (limited && (!Number.isInteger(count) || count < 1 || count > 999)) {
    showToast("保留数量必须是 1 到 999 的整数", "warn");
    return;
  }
  const publishedCount = (state.versions || []).filter((version) => version.status === "published").length;
  if (count > 0 && publishedCount > count) {
    const confirmed = await showConfirm({
      title: "更新版本保留策略",
      message: `保存后会立即删除最早的 ${publishedCount - count} 个已发布版本及其制品文件，草稿不会删除。确认继续？`,
      confirmText: "保存并清理",
      danger: true,
    });
    if (!confirmed) return;
  }
  const updated = await api(`/api/projects/${state.projectId}/version-retention`, {
    method: "PUT",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ max_published_versions: count }),
  });
  state.projectMaxPublishedVersions = Number(updated.max_published_versions || 0);
  state.projects = state.projects.map((project) => project.id === updated.id ? { ...project, ...updated } : project);
  state.versions = (await api(`/api/projects/${state.projectId}/versions`)) || [];
  renderVersionRetention();
  renderVersionList();
  renderVersionActivity();
  const selected = state.versions.find((version) => version.version === state.version) || latestPublishedVersion() || state.versions[0];
  if (selected) await selectVersion(selected);
  else document.getElementById("versionDetail")?.classList.add("hidden");
  showToast(count ? `已保留最近 ${count} 个已发布版本` : "已设置为无限保留");
}

function populateDiffDropdowns(versions) {
  const fromSel = document.getElementById("diffFromVer");
  const toSel = document.getElementById("diffToVer");
  if (!fromSel || !toSel) return;
  const output = document.getElementById("versionDiffOut");
  if ((versions || []).length < 2) {
    fromSel.disabled = true;
    toSel.disabled = true;
    if (output) output.innerHTML = '<div class="empty-state compact"><h3>还不能对比版本</h3><p>至少需要两个版本。请先发布第一个版本，再发布一个新版本后回来对比。</p></div>';
    return;
  }
  fromSel.disabled = false;
  toSel.disabled = false;
  const opts = versions.map((v) => `<option value="${escapeHtml(v.version)}">${escapeHtml(v.version)} (${v.status})</option>`).join("");
  fromSel.innerHTML = `<option value="">from 版本</option>` + opts;
  toSel.innerHTML = `<option value="">to 版本</option>` + opts;
}

function updateReviewButtons(status) {
  const canSubmit = status === "draft" || status === "rejected";
  const canPublish = status === "draft" || status === "pending_review";
  const canReject = status === "pending_review";
  const btnSubmit = document.getElementById("btnSubmitReview");
  const btnPublish = document.getElementById("btnPublish");
  const btnReject = document.getElementById("btnRejectReview");
  if (btnSubmit) btnSubmit.disabled = !canSubmit;
  if (btnPublish) btnPublish.disabled = !canPublish || status === "published";
  if (btnReject) btnReject.disabled = !canReject;
}

async function selectVersion(v) {
  state.version = v.version;
  state.versionStatus = v.status;
  state.previewConfirmed = false;
  document.getElementById("versionDetail").classList.remove("hidden");
  const detailTitle = document.getElementById("verDetailTitle");
  if (detailTitle) detailTitle.textContent = v.version;
  setVersionStatusBadge(v.status);
  document.getElementById("verCreated").textContent = v.created_at;
  document.getElementById("verPublished").textContent = v.published_at || "—";
  const provenance = document.getElementById("verProvenance");
  if (provenance) {
    const vcs = v.vcs || {};
    const artifact = v.artifact || {};
    const source = vcs.commit ? `${vcs.provider || "git"} · ${vcs.ref || "—"} · ${vcs.commit}` : "未绑定 VCS 来源（手工草稿可后续由 CI 写入）";
    const manifest = artifact.sha256 ? `制品清单 SHA-256 ${artifact.sha256} · ${artifact.file_count || 0} 文件 · ${artifact.bytes || 0} bytes` : "制品清单会在正式发布时生成";
    provenance.textContent = `${source}；${manifest}`;
  }
  document.getElementById("btnPublish").disabled = v.status === "published";
  document.getElementById("btnValidate").disabled = v.status === "published";
  updateReviewButtons(v.status);
  updateProjectWriteUI();
  renderVersionList();
  const dl = document.getElementById("btnDownloadVer");
  if (v.status === "published") {
    dl.classList.remove("hidden");
    dl.href = `/api/projects/${state.projectId}/versions/${encodeURIComponent(v.version)}/download`;
  } else {
    dl.classList.add("hidden");
  }
  document.getElementById("validateResult").innerHTML = "";
  document.getElementById("previewProject").value = state.projectName || "";
  document.getElementById("previewVersion").value = v.version;
  const rows = await loadVersionFileTags();
  try {
    const cfg = await api(`/api/projects/${state.projectId}/versions/${v.version}/config-files`);
    const dup = Object.entries(cfg.duplicates || {}).filter(([, n]) => n > 1);
    if (dup.length) {
      showFilePreviewMessage(`重复配置文件 basename: ${dup.map(([b]) => b).join(", ")}`);
    }
  } catch (_) {}
  renderFileDiffWorkspace([], document.getElementById("verPreviewTable"), { empty: "选择 server_id" });
  updateDeployCmd();
  tryAutoPreviewOnVersionSelect();
  updateWorkflowUI();
  return rows.map((r) => r.path);
}

async function loadVersionFileTags() {
  if (!state.projectId || !state.version) return [];
  const rows = (await api(`/api/projects/${state.projectId}/versions/${encodeURIComponent(state.version)}/file-tags`)) || [];
  state.fileRows = rows;
  renderFileList();
  return rows;
}

function renderFileList() {
  const fl = document.getElementById("fileList");
  if (!fl) return;
  if (!state.fileRows.length) {
    fl.innerHTML = `<div class="file-row muted">暂无文件</div>`;
    showFilePreviewMessage("当前版本无文件");
    return;
  }
  showFilePreviewMessage("点击左侧文件查看内容");
  const canEdit = canWriteProject() && state.versionStatus !== "published";
  const rows = filterFileRows(state.fileRows, state.fileFilter);
  if (!rows.length) {
    fl.innerHTML = `<div class="file-row muted">没有匹配文件</div>`;
    showFilePreviewMessage("没有匹配文件");
    return;
  }
  fl.innerHTML = buildFileTreeRows(rows, {
    treeId: "version",
    canEdit,
    rowAttr: (row) => `data-preview-index="${state.fileRows.indexOf(row)}"`,
    tags: (row) => displayTags(row.tags),
    actions: (row) =>
      canEdit
        ? `<button type="button" class="file-tag-action" data-action="edit-tags" data-index="${state.fileRows.indexOf(row)}">编辑</button>
           <button type="button" class="file-tag-action" data-action="clear-tags" data-index="${state.fileRows.indexOf(row)}">清空</button>`
        : "",
    forceExpand: !!state.fileFilter,
  });
  fl.querySelectorAll("[data-tree-folder]").forEach((btn) => {
    btn.onclick = () => {
      toggleTreeFolder("version", btn.dataset.treeFolder);
      renderFileList();
    };
  });
}

let pushHosts = [];
let pushBindings = [];
let selectedPushHostID = null;
let editingPushBindingID = null;
let pushTasks = [];
let editingPushTaskID = null;
let pushLogRefreshTimer = null;
let releaseHooks = [];
let releaseHookEvents = [];
let editingReleaseHookID = null;
let releaseHookRefreshTimer = null;
let deliveryNodes = [];

async function loadDeliveryNodes() {
  if (!state.projectId) return;
  try {
    deliveryNodes = (await api(`/api/projects/${state.projectId}/delivery-nodes`)) || [];
    renderDeliveryNodes();
  } catch (e) { showToast(e.message, "err"); }
}

function renderDeliveryNodes() {
  const box = document.getElementById("deliveryNodeList");
  if (!box) return;
  const push = deliveryNodes.filter((node) => node.delivery_mode === "push").length;
  const pull = deliveryNodes.filter((node) => node.delivery_mode === "pull").length;
  const online = deliveryNodes.filter((node) => node.online).length;
  const drift = deliveryNodes.filter((node) => node.drift).length;
  document.getElementById("clusterPushCount").textContent = push;
  document.getElementById("clusterPullCount").textContent = pull;
  document.getElementById("clusterOnlineCount").textContent = `${online}/${deliveryNodes.length}`;
  document.getElementById("clusterDriftCount").textContent = drift;
  if (!deliveryNodes.length) { box.innerHTML = '<div class="agent-empty">暂无节点。Pull Agent 心跳或 SSH server_id 绑定后自动出现。</div>'; return; }
  const options = (state.versions || []).filter((version) => version.status === "published").map((version) => `<option value="${escapeAttr(version.version)}">${escapeHtml(version.version)}</option>`).join("");
  box.innerHTML = `<table class="data-table cluster-node-table"><thead><tr><th>节点</th><th>模式</th><th>拓扑</th><th>版本</th><th>连接 / 心跳</th><th>策略</th></tr></thead><tbody>${deliveryNodes.map((node) => `<tr class="${node.drift ? "node-drift" : ""}"><td><strong>${escapeHtml(node.server_id)}</strong><small>${escapeHtml(node.os || "—")}/${escapeHtml(node.arch || "—")}</small><small>${escapeHtml((node.labels || []).join(" · "))}</small></td><td><span class="badge badge-${node.delivery_mode === "pull" ? "ok" : "draft"}">${node.delivery_mode === "pull" ? "Pull" : "Push"}</span></td><td>${escapeHtml(node.role || "未标角色")}<small>${escapeHtml(node.environment || node.host_name || "—")}</small></td><td><code>${escapeHtml(node.current_version || "未部署")}</code>${node.delivery_mode === "pull" ? `<span class="version-arrow">→</span><code>${escapeHtml(node.desired_version || "未设置")}</code><small>generation ${node.applied_generation}/${node.desired_generation} · ${node.drift ? "待收敛" : "已收敛"}</small>` : "<small>最近成功发布</small>"}</td><td><span class="node-presence ${node.online ? "online" : "offline"}"></span>${node.online ? "在线" : "离线"}<small>${escapeHtml(node.last_seen_at || "尚无检测记录")}</small></td><td>${node.delivery_mode === "pull" ? `<div class="cluster-policy"><select class="input input-sm" data-node-version="${escapeAttr(node.server_id)}"><option value="">选择已发布版本</option>${options}</select><label class="check-label"><input type="checkbox" data-node-auto="${escapeAttr(node.server_id)}" ${node.auto_follow ? "checked" : ""}/>自动跟随新版本</label><div><button type="button" class="btn btn-primary btn-sm project-write" data-apply-node="${escapeAttr(node.server_id)}">应用</button><button type="button" class="btn btn-danger btn-sm project-write" data-delete-node="${escapeAttr(node.server_id)}">移除</button></div></div>` : `<span class="badge badge-draft">任务 / Hook 驱动</span><small>${escapeHtml(node.host_name || "SSH 资源")}</small>`}</td></tr>`).join("")}</tbody></table>`;
  deliveryNodes.filter((node) => node.delivery_mode === "pull").forEach((node) => {
    const select = box.querySelector(`[data-node-version="${CSS.escape(node.server_id)}"]`); if (select) select.value = node.desired_version || "";
  });
  box.querySelectorAll("[data-apply-node]").forEach((button) => button.onclick = () => updateDeliveryNode(button.dataset.applyNode));
  box.querySelectorAll("[data-delete-node]").forEach((button) => button.onclick = () => deleteDeliveryNode(button.dataset.deleteNode));
  updateProjectWriteUI();
}

async function updateDeliveryNode(serverID) {
  const box = document.getElementById("deliveryNodeList");
  const version = box.querySelector(`[data-node-version="${CSS.escape(serverID)}"]`)?.value || "";
  const autoFollow = !!box.querySelector(`[data-node-auto="${CSS.escape(serverID)}"]`)?.checked;
  try { await api(`/api/projects/${state.projectId}/delivery-nodes/${encodeURIComponent(serverID)}/desired`, { method: "PUT", headers: { "Content-Type": "application/json" }, body: JSON.stringify({ version, auto_follow: autoFollow }) }); showToast("节点期望状态已更新"); await loadDeliveryNodes(); } catch (e) { showToast(e.message, "err"); }
}

async function deleteDeliveryNode(serverID) {
  if (!confirm(`移除 Pull 节点“${serverID}”的控制面状态？`)) return;
  try { await api(`/api/projects/${state.projectId}/delivery-nodes/${encodeURIComponent(serverID)}`, { method: "DELETE" }); await loadDeliveryNodes(); } catch (e) { showToast(e.message, "err"); }
}

function csvValues(value) {
  return String(value || "").split(",").map((v) => v.trim()).filter(Boolean);
}

function latestPublishedVersion() {
  return (state.versions || []).find((v) => v.status === "published") || null;
}

async function loadPushTasks() {
  if (!state.projectId) return;
  const latest = latestPublishedVersion();
  const version = document.getElementById("pushTaskVersion");
  const hint = document.getElementById("pushLatestHint");
  if (version) {
    version.innerHTML = `<option value="">最新已发布版本${latest ? `（${escapeHtml(latest.version)}）` : ""}</option>` + (state.versions || [])
      .filter((v) => v.status === "published")
      .map((v) => `<option value="${escapeAttr(v.version)}">${escapeHtml(v.version)}${v.version === latest?.version ? " · 最新" : ""}</option>`).join("");
  }
  if (hint) hint.textContent = latest ? `最新已发布版本：${latest.version}` : "暂无已发布版本";
  try {
    pushTasks = await api(`/api/projects/${state.projectId}/push-tasks`);
    renderPushTasks();
  } catch (e) { showToast(e.message, "err"); }
}

function resetPushTaskEditor() {
  editingPushTaskID = null;
  document.getElementById("pushTaskName").value = "";
  document.getElementById("pushTaskVersion").value = "";
  document.getElementById("pushTaskServerIDs").value = "";
  document.getElementById("pushTaskTags").value = "test";
  document.getElementById("pushTaskTagMatch").value = "all";
  document.getElementById("btnSavePushTask").textContent = "保存任务";
  document.getElementById("btnCancelPushTask").classList.add("hidden");
}

async function savePushTask() {
  if (!state.projectId) return;
  const payload = {
    name: document.getElementById("pushTaskName").value.trim(),
    version: document.getElementById("pushTaskVersion").value,
    server_ids: csvValues(document.getElementById("pushTaskServerIDs").value),
    tags: csvValues(document.getElementById("pushTaskTags").value || "test"),
    tag_match: document.getElementById("pushTaskTagMatch").value,
  };
  try {
    const path = editingPushTaskID ? `/api/projects/${state.projectId}/push-tasks/${editingPushTaskID}` : `/api/projects/${state.projectId}/push-tasks`;
    const task = await api(path, { method: editingPushTaskID ? "PUT" : "POST", headers: { "Content-Type": "application/json" }, body: JSON.stringify(payload) });
    showToast(editingPushTaskID ? "发布任务已更新" : "发布任务已保存");
    resetPushTaskEditor();
    await loadPushTasks();
    return task;
  } catch (e) { showToast(e.message, "err"); }
}

function renderPushTasks() {
  const box = document.getElementById("pushTaskList");
  if (!box) return;
  if (!pushTasks.length) {
    box.innerHTML = '<div class="empty-state compact"><h3>还没有发布任务</h3><p>先保存一个任务；SSH 节点与 server_id 绑定在全局 Agent 页面维护。</p></div>';
    return;
  }
  box.innerHTML = `<table class="data-table release-task-table"><thead><tr><th>任务</th><th>版本策略</th><th>目标筛选</th><th>执行</th><th>最近运行</th><th>操作</th></tr></thead><tbody>${pushTasks.map((task) => `<tr><td><strong>${escapeHtml(task.name)}</strong><small>#${task.id}</small></td><td>${task.version ? `<code>${escapeHtml(task.version)}</code>` : '<span class="badge badge-ok">每次取最新已发布</span>'}</td><td><small>server_id：${escapeHtml((task.server_ids || []).join(", ") || "全部")}</small><small>标签（${task.tag_match === "any" ? "任一" : "全部"}）：${escapeHtml((task.tags || []).join(", ") || "test")}</small></td><td><strong>${task.run_count || 0}</strong><small>次</small></td><td><small>${escapeHtml(task.last_run_at || "尚未执行")}</small></td><td class="release-task-actions"><button type="button" class="btn btn-secondary btn-sm" data-run-push-task="${task.id}" data-dry-run="true">预演</button><button type="button" class="btn btn-primary btn-sm" data-run-push-task="${task.id}">发布</button><button type="button" class="btn btn-ghost btn-sm" data-edit-push-task="${task.id}">编辑</button><button type="button" class="btn btn-danger btn-sm" data-delete-push-task="${task.id}">删除</button></td></tr>`).join("")}</tbody></table>`;
  box.querySelectorAll("[data-run-push-task]").forEach((button) => button.onclick = () => runPushTask(Number(button.dataset.runPushTask), button.dataset.dryRun === "true", button));
  box.querySelectorAll("[data-edit-push-task]").forEach((button) => button.onclick = () => editPushTask(Number(button.dataset.editPushTask)));
  box.querySelectorAll("[data-delete-push-task]").forEach((button) => button.onclick = () => deletePushTask(Number(button.dataset.deletePushTask)));
}

function editPushTask(taskID) {
  const task = pushTasks.find((item) => item.id === taskID);
  if (!task) return;
  editingPushTaskID = taskID;
  document.getElementById("pushTaskName").value = task.name;
  document.getElementById("pushTaskVersion").value = task.version || "";
  document.getElementById("pushTaskServerIDs").value = (task.server_ids || []).join(", ");
  document.getElementById("pushTaskTags").value = (task.tags || []).join(", ");
  document.getElementById("pushTaskTagMatch").value = task.tag_match || "all";
  document.getElementById("btnSavePushTask").textContent = "更新任务";
  document.getElementById("btnCancelPushTask").classList.remove("hidden");
}

function newIdempotencyKey(taskID, dryRun) {
  const suffix = globalThis.crypto?.randomUUID?.() || `${Date.now()}-${Math.random().toString(16).slice(2)}`;
  return `ui:${state.projectId}:${taskID}:${dryRun ? "preview" : "release"}:${suffix}`;
}

async function runPushTask(taskID, dryRun, triggerButton) {
  const task = pushTasks.find((item) => item.id === taskID);
  if (!task) return;
  if (!dryRun) {
    const version = task.version || latestPublishedVersion()?.version || "无可用版本";
    const targets = (task.server_ids || []).join(", ") || "全部匹配服务器（串行）";
    const confirmed = await showConfirm({ title: `发布 ${task.name}`, message: `版本：${version}\n目标：${targets}\n策略：先预拉取；SIGTERM 后等待正常退出；换包后必须通过启动与健康检查。`, confirmText: "确认发布", cancelText: "取消" });
    if (!confirmed) return;
  }
  const storageKey = `express233:push:${state.projectId}:${taskID}:${dryRun ? "preview" : "release"}`;
  const idempotencyKey = sessionStorage.getItem(storageKey) || newIdempotencyKey(taskID, dryRun);
  sessionStorage.setItem(storageKey, idempotencyKey);
  if (triggerButton) triggerButton.disabled = true;
  try {
    const deployment = await api(`/api/projects/${state.projectId}/push-tasks/${taskID}/run`, { method: "POST", headers: { "Content-Type": "application/json", "Idempotency-Key": idempotencyKey }, body: JSON.stringify({ dry_run: dryRun, idempotency_key: idempotencyKey }) });
    sessionStorage.removeItem(storageKey);
    showToast(deployment.replayed ? `已接续原执行 #${deployment.id}，未重复发布` : dryRun ? `预演 #${deployment.id} 完成` : `发布 #${deployment.id} 已进入串行队列`);
    await loadPushTasks();
    setGlobalView("release");
    navigateReleaseSection("logs");
  } catch (e) {
    showToast(`${e.message}；重试将接续同一执行`, "err", 5200);
  } finally {
    if (triggerButton) triggerButton.disabled = false;
  }
}

async function deletePushTask(taskID) {
  if (!await showConfirm({ title: "删除发布任务", message: "仅删除可复用任务定义，历史执行日志仍会保留 30 天。", danger: true })) return;
  try {
    await api(`/api/projects/${state.projectId}/push-tasks/${taskID}`, { method: "DELETE" });
    if (editingPushTaskID === taskID) resetPushTaskEditor();
    await loadPushTasks();
    showToast("任务已删除，历史日志未受影响");
  } catch (e) { showToast(e.message, "err"); }
}

function resetReleaseHookEditor() {
  editingReleaseHookID = null;
  document.getElementById("releaseHookName").value = "";
  document.getElementById("releaseHookTask").value = "";
  document.getElementById("releaseHookDebounce").value = "30";
  document.getElementById("releaseHookEnabled").checked = true;
  document.getElementById("btnSaveReleaseHook").textContent = "保存 Hook";
  document.getElementById("btnCancelReleaseHook").classList.add("hidden");
}

async function loadReleaseHooks() {
  if (!state.projectId) return;
  try {
    const [tasks, hooks, events] = await Promise.all([
      api(`/api/projects/${state.projectId}/push-tasks`),
      api(`/api/projects/${state.projectId}/release-hooks`),
      api(`/api/projects/${state.projectId}/release-hook-events?limit=100`),
    ]);
    pushTasks = tasks || [];
    releaseHooks = hooks || [];
    releaseHookEvents = events || [];
    const taskSelect = document.getElementById("releaseHookTask");
    if (taskSelect) {
      const selected = taskSelect.value;
      taskSelect.innerHTML = '<option value="">选择发布任务</option>' + pushTasks.map((task) => `<option value="${task.id}">${escapeHtml(task.name)}</option>`).join("");
      if (pushTasks.some((task) => String(task.id) === selected)) taskSelect.value = selected;
    }
    renderReleaseHooks();
    renderReleaseHookEvents();
    if (releaseHookRefreshTimer) window.clearTimeout(releaseHookRefreshTimer);
    if (state.globalView === "release" && state.releaseSection === "hooks" && releaseHooks.some((hook) => hook.pending_events > 0 || hook.last_status === "running")) {
      releaseHookRefreshTimer = window.setTimeout(loadReleaseHooks, 2000);
    }
  } catch (error) { showToast(error.message, "err"); }
}

async function saveReleaseHook() {
  const payload = {
    name: document.getElementById("releaseHookName").value.trim(),
    task_id: Number(document.getElementById("releaseHookTask").value),
    debounce_seconds: Number(document.getElementById("releaseHookDebounce").value || 30),
    enabled: document.getElementById("releaseHookEnabled").checked,
  };
  try {
    const path = editingReleaseHookID ? `/api/projects/${state.projectId}/release-hooks/${editingReleaseHookID}` : `/api/projects/${state.projectId}/release-hooks`;
    await api(path, { method: editingReleaseHookID ? "PUT" : "POST", headers: { "Content-Type": "application/json" }, body: JSON.stringify(payload) });
    showToast(editingReleaseHookID ? "Hook 已更新" : "Hook 已创建");
    resetReleaseHookEditor();
    await loadReleaseHooks();
  } catch (error) { showToast(error.message, "err"); }
}

function renderReleaseHooks() {
  const box = document.getElementById("releaseHookList");
  if (!box) return;
  if (!releaseHooks.length) {
    box.innerHTML = '<div class="empty-state compact"><h3>还没有自动 Hook</h3><p>先在“发布任务”创建可重复任务，再在这里关联并启用。</p></div>';
    return;
  }
  box.innerHTML = `<table class="data-table release-hook-table"><thead><tr><th>Hook</th><th>关联任务</th><th>开关</th><th>合并窗口</th><th>触发统计</th><th>最近状态</th><th>操作</th></tr></thead><tbody>${releaseHooks.map((hook) => `<tr><td><strong>${escapeHtml(hook.name)}</strong><small>#${hook.id}</small></td><td><strong>${escapeHtml(hook.task_name || "任务已失效")}</strong><small>任务 #${hook.task_id}</small></td><td><label class="switch"><input type="checkbox" data-toggle-release-hook="${hook.id}" ${hook.enabled ? "checked" : ""}/><span></span></label><small>${hook.enabled ? "已启用" : "已停用"}</small></td><td><strong>${hook.debounce_seconds} 秒</strong>${hook.pending_events ? `<small class="hook-pending">等待中 · 已合并 ${hook.pending_events} 次</small><small>${escapeHtml(hook.due_at)}</small>` : '<small>当前无等待任务</small>'}</td><td><strong>${hook.trigger_count || 0}</strong><small>触发 · ${hook.merge_count || 0} 合并 · ${hook.run_count || 0} 派发</small></td><td><span class="badge badge-${hook.last_status === "failed" ? "warn" : hook.last_status === "success" ? "ok" : "draft"}">${escapeHtml(hook.last_status || "尚未触发")}</span><small title="${escapeAttr(hook.last_error || "")}">${escapeHtml(hook.last_error || hook.last_trigger_at || "—")}</small></td><td class="release-task-actions"><button type="button" class="btn btn-secondary btn-sm" data-trigger-release-hook="${hook.id}" ${hook.enabled ? "" : "disabled"}>立即触发</button><button type="button" class="btn btn-ghost btn-sm" data-edit-release-hook="${hook.id}">编辑</button><button type="button" class="btn btn-danger btn-sm" data-delete-release-hook="${hook.id}">删除</button></td></tr>`).join("")}</tbody></table>`;
  box.querySelectorAll("[data-toggle-release-hook]").forEach((input) => input.onchange = () => toggleReleaseHook(Number(input.dataset.toggleReleaseHook), input.checked));
  box.querySelectorAll("[data-trigger-release-hook]").forEach((button) => button.onclick = () => triggerReleaseHook(Number(button.dataset.triggerReleaseHook)));
  box.querySelectorAll("[data-edit-release-hook]").forEach((button) => button.onclick = () => editReleaseHook(Number(button.dataset.editReleaseHook)));
  box.querySelectorAll("[data-delete-release-hook]").forEach((button) => button.onclick = () => deleteReleaseHook(Number(button.dataset.deleteReleaseHook)));
}

function editReleaseHook(hookID) {
  const hook = releaseHooks.find((item) => item.id === hookID);
  if (!hook) return;
  editingReleaseHookID = hookID;
  document.getElementById("releaseHookName").value = hook.name;
  document.getElementById("releaseHookTask").value = String(hook.task_id);
  document.getElementById("releaseHookDebounce").value = String(hook.debounce_seconds);
  document.getElementById("releaseHookEnabled").checked = hook.enabled;
  document.getElementById("btnSaveReleaseHook").textContent = "更新 Hook";
  document.getElementById("btnCancelReleaseHook").classList.remove("hidden");
}

async function toggleReleaseHook(hookID, enabled) {
  const hook = releaseHooks.find((item) => item.id === hookID);
  if (!hook) return;
  try {
    await api(`/api/projects/${state.projectId}/release-hooks/${hookID}`, { method: "PUT", headers: { "Content-Type": "application/json" }, body: JSON.stringify({ name: hook.name, task_id: hook.task_id, debounce_seconds: hook.debounce_seconds, enabled }) });
    showToast(enabled ? "Hook 已启用" : "Hook 已停用，等待中的触发已取消");
    await loadReleaseHooks();
  } catch (error) { showToast(error.message, "err"); await loadReleaseHooks(); }
}

async function triggerReleaseHook(hookID) {
  try {
    const hook = await api(`/api/projects/${state.projectId}/release-hooks/${hookID}/trigger`, { method: "POST", headers: { "Content-Type": "application/json" }, body: JSON.stringify({ source: "manual_ui" }) });
    showToast(`已进入合并窗口，将于 ${hook.due_at} 后派发`);
    await loadReleaseHooks();
  } catch (error) { showToast(error.message, "err"); }
}

async function deleteReleaseHook(hookID) {
  if (!await showConfirm({ title: "删除自动 Hook", message: "Hook 触发历史仍会按 30 天策略保留。通常只需关闭开关，无需删除。", danger: true })) return;
  try {
    await api(`/api/projects/${state.projectId}/release-hooks/${hookID}`, { method: "DELETE" });
    if (editingReleaseHookID === hookID) resetReleaseHookEditor();
    await loadReleaseHooks();
    showToast("Hook 已删除，历史记录未删除");
  } catch (error) { showToast(error.message, "err"); }
}

function renderReleaseHookEvents() {
  const box = document.getElementById("releaseHookEventList");
  if (!box) return;
  box.innerHTML = `<table class="data-table hook-event-table"><thead><tr><th>时间</th><th>Hook</th><th>事件</th><th>来源 / 版本</th><th>合并</th><th>结果</th></tr></thead><tbody>${releaseHookEvents.map((event) => `<tr><td>${escapeHtml(event.created_at)}</td><td><strong>${escapeHtml(event.hook_name)}</strong><small>#${event.hook_id}</small></td><td><span class="badge badge-${event.status === "failed" ? "warn" : event.status === "success" ? "ok" : "draft"}">${event.status === "cancelled" ? "等待已取消" : event.kind === "dispatch" ? "最终派发" : event.status === "merged" ? "合并触发" : "首次触发"}</span></td><td><small>${escapeHtml(event.source || "system")}</small><code>${escapeHtml(event.version)}</code></td><td>${event.merged_events || 1} 次</td><td><strong>${escapeHtml(event.deployment_status || event.status)}</strong>${event.deployment_id ? `<small>发布 #${event.deployment_id}</small>` : ""}</td></tr>`).join("")}</tbody></table>`;
}

function renderPushHosts() {
  const hostList = document.getElementById("pushHostList");
  if (!hostList) return;
  const query = document.getElementById("sshSearch")?.value.trim().toLowerCase() || "";
  const statusFilter = document.getElementById("sshStatusFilter")?.value || "";
  const projectFilter = document.getElementById("sshProjectFilter")?.value || "";
  const rows = pushHosts.filter((host) => {
    const tags = (host.bindings || []).map((binding) => binding.target_tag || `${binding.project_name || "未归属"}|${binding.server_id}`);
    const matchesText = !query || `${host.name} ${host.address} ${tags.join(" ")}`.toLowerCase().includes(query);
    const expectedStatus = statusFilter === "success" ? "ok" : statusFilter;
    const matchesStatus = !statusFilter || (host.last_check_status || "unknown") === expectedStatus;
    const matchesProject = !projectFilter || (host.bindings || []).some((binding) => binding.project_name === projectFilter);
    return matchesText && matchesStatus && matchesProject;
  });
  if (!rows.length) { hostList.innerHTML = '<div class="agent-empty">没有匹配机器</div>'; return; }
  hostList.innerHTML = `<table class="data-table ssh-machine-table"><thead><tr><th>机器</th><th>SSH 地址</th><th>健康状态</th><th>目标标签</th><th>最近检查</th><th>操作</th></tr></thead><tbody>${rows.map((h) => {
    const tags = (h.bindings || []).map((binding) => `<code class="target-tag">${escapeHtml(binding.target_tag || `${binding.project_name || "未归属"}|${binding.server_id}`)}</code>`).join("") || '<span class="hint">未绑定目标</span>';
    const checkMeta = `${formatAgentInterval(h.health_check_interval_seconds || 3600)} · ${h.last_check_latency_ms ? `${h.last_check_latency_ms} ms` : "—"}`;
    return `<tr class="${h.id === selectedPushHostID ? "selected-row" : ""}"><td><strong>${escapeHtml(h.name)}</strong><small>${(h.bindings || []).length} 个目标</small></td><td><code>${escapeHtml(h.username)}@${escapeHtml(h.address)}:${h.port}</code><small>${escapeHtml(h.auth_mode || "private_key")}</small></td><td>${agentHealthBadge(h)}</td><td><div class="target-tag-list">${tags}</div></td><td>${escapeHtml(h.last_check_at || "尚未检查")}<small>${checkMeta}</small></td><td><button type="button" class="btn btn-ghost btn-sm" data-check-push-host="${h.id}">立即检查</button><button type="button" class="btn btn-ghost btn-sm" data-push-host="${h.id}">目标绑定</button><button type="button" class="btn btn-danger btn-sm" data-del-push-host="${h.id}">删除</button></td></tr>`;
  }).join("")}</tbody></table>`;
  hostList.querySelectorAll("[data-push-host]").forEach((btn) => btn.onclick = async () => {
    selectedPushHostID = Number(btn.dataset.pushHost);
    const host = pushHosts.find((item) => item.id === selectedPushHostID);
    document.getElementById("sshDrawerTitle").textContent = host?.name || "机器绑定";
    document.getElementById("sshBindingDrawer")?.classList.remove("hidden");
    pushBindings = await api(`/api/push/hosts/${selectedPushHostID}/servers`);
    editingPushBindingID = null;
    renderPushBindings();
    renderPushHosts();
  });
  hostList.querySelectorAll("[data-check-push-host]").forEach((btn) => btn.onclick = () => runAgentHostCheck(Number(btn.dataset.checkPushHost), btn));
  hostList.querySelectorAll("[data-agent-enabled]").forEach((input) => input.onchange = () => saveAgentHostHealth(Number(input.dataset.agentEnabled)));
  hostList.querySelectorAll("[data-agent-interval]").forEach((select) => select.onchange = () => saveAgentHostHealth(Number(select.dataset.agentInterval)));
  hostList.querySelectorAll("[data-del-push-host]").forEach((btn) => btn.onclick = async () => { if (!await showConfirm({ title: "删除 SSH 资源", message: "会一并删除该资源下的服务器绑定。", danger: true })) return; await api(`/api/push/hosts/${btn.dataset.delPushHost}`, { method: "DELETE" }); selectedPushHostID = null; pushBindings = []; renderPushBindings(); await loadAgentWorkspace(); });
}

function agentIcon(name) {
  const paths = {
    activity: '<path d="M22 12h-4l-3 9L9 3l-3 9H2"/>',
    server: '<rect x="2" y="3" width="20" height="8" rx="2"/><rect x="2" y="13" width="20" height="8" rx="2"/><path d="M6 7h.01M6 17h.01"/>',
    route: '<circle cx="6" cy="19" r="3"/><circle cx="18" cy="5" r="3"/><path d="M6 16V8a3 3 0 0 1 3-3h6M18 8v8a3 3 0 0 1-3 3H9"/>',
    shield: '<path d="M12 22s8-4 8-10V5l-8-3-8 3v7c0 6 8 10 8 10Z"/><path d="m9 12 2 2 4-4"/>',
    clock: '<circle cx="12" cy="12" r="9"/><path d="M12 7v5l3 2"/>',
    history: '<path d="M3 12a9 9 0 1 0 3-6.7L3 8"/><path d="M3 3v5h5M12 7v5l4 2"/>',
  };
  return `<svg class="btn-svg" aria-hidden="true" viewBox="0 0 24 24">${paths[name] || paths.activity}</svg>`;
}

function agentHealthBadge(host) {
  const status = host.last_check_status || "unknown";
  const label = status === "ok" ? "可连接" : status === "failed" ? "连接失败" : "待检查";
  return `<span class="health-status is-${escapeAttr(status)}"><i></i>${label}</span>`;
}

function formatAgentInterval(seconds) {
  const value = Number(seconds || 3600);
  if (value % 86400 === 0) return `${value / 86400} 天/次`;
  if (value % 3600 === 0) return `${value / 3600} 小时/次`;
  return `${Math.round(value / 60)} 分钟/次`;
}

function agentIntervalOptions(selected) {
  const choices = [300, 900, 1800, 3600, 21600, 86400];
  if (!choices.includes(Number(selected))) choices.push(Number(selected));
  return choices.sort((a, b) => a - b).map((value) => `<option value="${value}" ${value === Number(selected) ? "selected" : ""}>${formatAgentInterval(value)}</option>`).join("");
}

async function loadAgentWorkspace() {
  const summary = document.getElementById("agentSummary");
  if (summary) summary.innerHTML = '<div class="agent-loading">正在读取节点与 API 能力…</div>';
  try {
    const [capabilityPayload, hosts] = await Promise.all([api("/api/agent/capabilities"), api("/api/push/hosts")]);
    const bindingLists = await Promise.all((hosts || []).map((host) => api(`/api/push/hosts/${host.id}/servers`).catch(() => [])));
    pushHosts = (hosts || []).map((host, index) => ({ ...host, bindings: bindingLists[index] || [] }));
    const projectOptions = '<option value="">选择项目</option>' + (state.projects || []).map((project) => `<option value="${escapeAttr(project.name)}">${escapeHtml(project.name)}</option>`).join("");
    const bindingProject = document.getElementById("pushBindingProject");
    if (bindingProject) bindingProject.innerHTML = projectOptions;
    const projectFilter = document.getElementById("sshProjectFilter");
    if (projectFilter) projectFilter.innerHTML = '<option value="">全部项目</option>' + (state.projects || []).map((project) => `<option value="${escapeAttr(project.name)}">${escapeHtml(project.name)}</option>`).join("");
    renderAgentSummary(capabilityPayload, pushHosts);
    renderAgentCapabilities(capabilityPayload.capabilities || []);
    renderPushHosts();
  } catch (error) {
    if (summary) summary.innerHTML = `<div class="agent-error">${escapeHtml(error.message)}</div>`;
    showToast(error.message, "err");
  }
}

function renderAgentSummary(payload, hosts) {
  const healthy = hosts.filter((host) => host.last_check_status === "ok").length;
  const failed = hosts.filter((host) => host.last_check_status === "failed").length;
  const enabled = hosts.filter((host) => host.health_check_enabled).length;
  document.getElementById("agentSummary").innerHTML = `
    <div class="agent-stat">${agentIcon("server")}<div><strong>${hosts.length}</strong><span>SSH 节点</span></div></div>
    <div class="agent-stat is-good">${agentIcon("activity")}<div><strong>${healthy}</strong><span>当前可连接</span></div></div>
    <div class="agent-stat ${failed ? "is-bad" : ""}">${agentIcon("shield")}<div><strong>${failed}</strong><span>连接失败</span></div></div>
    <div class="agent-stat">${agentIcon("clock")}<div><strong>${enabled}</strong><span>已启用定检</span></div></div>
    <div class="agent-stat">${agentIcon("route")}<div><strong>${(payload.capabilities || []).length}</strong><span>Agent API 操作</span></div></div>`;
}

function renderAgentCapabilities(capabilities) {
  const box = document.getElementById("agentCapabilities");
  if (!box) return;
  const groups = capabilities.reduce((result, item) => {
    (result[item.group] ||= []).push(item);
    return result;
  }, {});
  box.innerHTML = Object.entries(groups).map(([group, items]) => `<section class="api-group"><h4>${escapeHtml(group)}</h4>${items.map((item) => `<div class="api-row"><span class="http-method method-${item.method.toLowerCase()}">${escapeHtml(item.method)}</span><code>${escapeHtml(item.path)}</code><span class="api-description">${escapeHtml(item.description)}</span><span class="api-role">${escapeHtml(item.role)}</span></div>`).join("")}</section>`).join("");
}

function renderAgentHosts(hosts) {
  const box = document.getElementById("agentHostList");
  if (!box) return;
  if (!hosts.length) {
    box.innerHTML = '<div class="agent-empty">还没有 SSH 节点。请在本页“SSH 资源管理”创建第一台节点。</div>';
    return;
  }
  box.innerHTML = `<table class="data-table agent-host-table"><thead><tr><th>节点</th><th>连接</th><th>状态</th><th>定时检查</th><th>最近 / 下次</th><th>操作</th></tr></thead><tbody>${hosts.map((host) => `<tr><td><strong>${escapeHtml(host.name)}</strong><small>${escapeHtml(host.host_key_fingerprint || (host.host_key ? "Host key 已固定" : "Host key 待登记"))}</small></td><td><code>${escapeHtml(host.username)}@${escapeHtml(host.address)}:${host.port}</code><small>${escapeHtml(host.auth_mode)}</small></td><td>${agentHealthBadge(host)}<small>${host.last_check_latency_ms ? `${host.last_check_latency_ms} ms` : "—"}</small></td><td><label class="switch"><input type="checkbox" data-agent-enabled="${host.id}" ${host.health_check_enabled ? "checked" : ""}/><span></span></label><select class="input input-sm agent-interval" data-agent-interval="${host.id}" ${host.health_check_enabled ? "" : "disabled"}>${agentIntervalOptions(host.health_check_interval_seconds || 3600)}</select></td><td><small>${escapeHtml(host.last_check_at || "尚未检查")}</small><small class="next-check">${escapeHtml(host.next_check_at || "未安排")}</small></td><td><button type="button" class="btn btn-primary btn-sm" data-agent-check="${host.id}">${agentIcon("activity")}立即检查</button><button type="button" class="btn btn-ghost btn-sm" data-agent-history="${host.id}">${agentIcon("history")}历史</button></td></tr>`).join("")}</tbody></table>`;
  box.querySelectorAll("[data-agent-check]").forEach((button) => button.onclick = () => runAgentHostCheck(Number(button.dataset.agentCheck), button));
  box.querySelectorAll("[data-agent-history]").forEach((button) => button.onclick = () => loadAgentHostHistory(Number(button.dataset.agentHistory)));
  box.querySelectorAll("[data-agent-enabled]").forEach((input) => input.onchange = () => saveAgentHostHealth(Number(input.dataset.agentEnabled)));
  box.querySelectorAll("[data-agent-interval]").forEach((select) => select.onchange = () => saveAgentHostHealth(Number(select.dataset.agentInterval)));
}

async function saveAgentHostHealth(hostID) {
  const host = pushHosts.find((item) => item.id === hostID);
  if (!host) return;
  const enabled = document.querySelector(`[data-agent-enabled="${hostID}"]`).checked;
  const interval = Number(document.querySelector(`[data-agent-interval="${hostID}"]`).value);
  try {
    await api(`/api/push/hosts/${hostID}`, { method: "PUT", headers: { "Content-Type": "application/json" }, body: JSON.stringify({
      name: host.name, address: host.address, port: host.port, username: host.username, auth_mode: host.auth_mode, host_key: host.host_key,
      health_check_enabled: enabled, health_check_interval_seconds: interval,
    }) });
    showToast(enabled ? `已设为${formatAgentInterval(interval)}` : "已关闭定时检查");
    await loadAgentWorkspace();
  } catch (error) { showToast(error.message, "err"); await loadAgentWorkspace(); }
}

async function runAgentHostCheck(hostID, button) {
  if (button) { button.disabled = true; button.classList.add("is-checking"); button.innerHTML = `${agentIcon("activity")}检查中…`; }
  try {
    const result = await api(`/api/push/hosts/${hostID}/check`, { method: "POST" });
    showToast(result.status === "ok" ? `SSH 可连接，${result.latency_ms} ms` : `SSH 连接失败：${result.error}`, result.status === "ok" ? "ok" : "err");
    if (state.globalView === "release" && state.releaseSection === "ssh") { await loadAgentWorkspace(); await loadAgentHostHistory(hostID); }
    else await loadAgentWorkspace();
  } catch (error) { showToast(error.message, "err"); }
  finally { if (button) { button.disabled = false; button.classList.remove("is-checking"); } }
}

async function loadAgentHostHistory(hostID) {
  const box = document.getElementById("agentCheckHistory");
  const host = pushHosts.find((item) => item.id === hostID);
  box.classList.remove("hidden");
  box.innerHTML = '<div class="agent-loading">正在读取检查历史…</div>';
  try {
    const checks = await api(`/api/push/hosts/${hostID}/checks?limit=50`);
    box.innerHTML = `<div class="history-header"><div><h4>${escapeHtml(host?.name || "SSH 节点")} · 检查历史</h4><p>每条记录对应一次真实连接尝试，没有隐藏重试。</p></div><button type="button" class="btn btn-ghost btn-sm" data-close-history>关闭</button></div><div class="history-list">${checks.length ? checks.map((check) => `<div class="history-row"><span class="health-status is-${escapeAttr(check.status)}"><i></i>${check.status === "ok" ? "成功" : "失败"}</span><time>${escapeHtml(check.checked_at)}</time><span>${check.latency_ms} ms</span><span>${check.trigger === "manual" ? "手动" : "定时"}</span><code title="${escapeAttr(check.error || "")}">${escapeHtml(check.error || "连接与认证正常")}</code></div>`).join("") : '<div class="agent-empty">暂无检查记录</div>'}</div>`;
    box.querySelector("[data-close-history]").onclick = () => box.classList.add("hidden");
  } catch (error) { box.innerHTML = `<div class="agent-error">${escapeHtml(error.message)}</div>`; }
}

document.getElementById("btnAgentReload")?.addEventListener("click", loadAgentWorkspace);

function renderPushBindings() {
  const box = document.getElementById("pushBindingList");
  if (!box) return;
  box.innerHTML = pushBindings.length ? `<div class="binding-list-title">已绑定目标</div>${pushBindings.map((b) => `<article class="binding-card"><div class="binding-card-head"><code class="target-tag">${escapeHtml(b.target_tag || `${b.project_name || "未归属"}|${b.server_id}`)}</code><span><button type="button" class="btn btn-ghost btn-sm" data-edit-push-binding="${b.id}">编辑</button><button type="button" class="btn btn-danger btn-sm" data-del-push-binding="${b.id}">删除</button></span></div><label>远程根目录<code>${escapeHtml(b.remote_root)}</code></label><label>发布标签<span>${escapeHtml(b.labels)}</span></label><label>内容标签<span>${escapeHtml(b.content_tags || "全部")}</span></label></article>`).join("")}` : '<div class="agent-empty">尚未绑定项目目标</div>';
  box.querySelectorAll("[data-del-push-binding]").forEach((btn) => btn.onclick = async () => {
    await api(`/api/push/hosts/${selectedPushHostID}/servers/${btn.dataset.delPushBinding}`, { method: "DELETE" });
    pushBindings = await api(`/api/push/hosts/${selectedPushHostID}/servers`);
    const selectedHost = pushHosts.find((host) => host.id === selectedPushHostID);
    if (selectedHost) selectedHost.bindings = pushBindings;
    renderPushBindings();
    renderPushHosts();
  });
  box.querySelectorAll("[data-edit-push-binding]").forEach((btn) => btn.onclick = () => { const b = pushBindings.find((x) => x.id === Number(btn.dataset.editPushBinding)); if (!b) return; editingPushBindingID = b.id; document.getElementById("pushBindingProject").value = b.project_name || ""; document.getElementById("pushBindingServerID").value = b.server_id; document.getElementById("pushBindingLabels").value = b.labels; document.getElementById("pushBindingContentTags").value = b.content_tags || ""; document.getElementById("pushBindingRoot").value = b.remote_root; updatePushBindingTargetTag(); });
}

async function loadPushLogs() {
  if (!state.projectId) return;
  const box = document.getElementById("pushLogList");
  if (!box) return;
  const deployments = await api(`/api/projects/${state.projectId}/push-deployments`);
  const statusLabel = { queued: "等待串行执行", running: "发布中", success: "成功", failed: "失败" };
  box.innerHTML = `<table class="data-table"><thead><tr><th>时间</th><th>任务快照</th><th>版本</th><th>筛选</th><th>状态</th><th>服务器日志</th></tr></thead><tbody>${deployments.map((d) => { const selector = parsePushSelector(d.selector); return `<tr><td>${escapeHtml(d.created_at)}</td><td><strong>${escapeHtml(d.task_name || "临时发布")}</strong><small>${d.task_id ? `任务 #${d.task_id}` : `执行 #${d.id}`}</small>${d.idempotency_key ? `<small>请求 ${escapeHtml(d.idempotency_key.slice(-12))}</small>` : ""}</td><td><code>${escapeHtml(d.version)}</code></td><td><small>server_id：${escapeHtml((selector.server_ids || []).join(", ") || "全部")}</small><small>标签：${escapeHtml((selector.tags || []).join(", ") || "test")}</small></td><td><span class="badge badge-${d.status === "success" ? "ok" : d.status === "failed" ? "warn" : "draft"}">${escapeHtml(statusLabel[d.status] || d.status)}</span></td><td><button type="button" class="btn btn-ghost btn-sm" data-push-log="${d.id}">查看逐服输出</button></td></tr>`; }).join("")}</tbody></table>`;
  box.querySelectorAll("[data-push-log]").forEach((btn) => btn.onclick = async () => { const d = await api(`/api/projects/${state.projectId}/push-deployments/${btn.dataset.pushLog}`); const lines = (d.targets || []).map((t) => `# ${t.host_name} / ${t.server_id} [${t.status}]\n${t.output || "等待执行"}`).join("\n\n"); showModal({ title: `${d.task_name || "临时发布"} · 执行 #${d.id}`, message: lines || "没有目标", confirmText: "关闭", cancelText: "关闭", mode: "confirm" }); });
  if (pushLogRefreshTimer) window.clearTimeout(pushLogRefreshTimer);
  if (state.globalView === "release" && deployments.some((item) => item.status === "queued" || item.status === "running")) {
    pushLogRefreshTimer = window.setTimeout(loadPushLogs, 3000);
  }
}

function parsePushSelector(value) {
  try { return JSON.parse(value || "{}"); } catch (_) { return {}; }
}

function filterFileRows(rows, query) {
  const q = String(query || "").trim().toLowerCase();
  if (!q) return rows || [];
  return (rows || []).filter((row) => {
    const tags = displayTags(row.tags).join(" ");
    return `${row.path || ""} ${tags}`.toLowerCase().includes(q);
  });
}

function versionStatusMeta(status) {
  const labels = {
    published: ["已发布", "is-published"],
    draft: ["草稿", "is-draft"],
    pending_review: ["待审批", "is-pending"],
    rejected: ["已驳回", "is-rejected"],
  };
  const [label, className] = labels[status] || [status || "未知", "is-unknown"];
  return { label, className };
}

function buildFileTreeRows(rows, opts = {}) {
  const files = (rows || []).map((row, index) => ({ row, index, path: row.path || row }));
  const folders = new Set();
  files.forEach(({ path }) => {
    const parts = String(path).split("/");
    for (let i = 1; i < parts.length; i += 1) folders.add(parts.slice(0, i).join("/"));
  });
  const folderRows = [...folders].sort().map((path) => ({ path, type: "folder" }));
  const fileRows = files.map((x) => ({ ...x, type: "file" }));
  const treeId = opts.treeId || "version";
  const collapsed = opts.forceExpand ? new Set() : getCollapsedTreeSet(treeId);
  return [...folderRows, ...fileRows]
    .sort((a, b) => a.path.localeCompare(b.path))
    .filter((item) => !hasCollapsedParent(item.path, collapsed))
    .map((item) => {
      const depth = Math.max(0, item.path.split("/").length - 1);
      if (item.type === "folder") {
        const isCollapsed = collapsed.has(item.path);
        return `<button type="button" class="file-row tree-folder" style="--depth:${depth}" data-tree-folder="${escapeAttr(item.path)}" aria-expanded="${isCollapsed ? "false" : "true"}">
          <span class="tree-caret" aria-hidden="true">${isCollapsed ? "▸" : "▾"}</span>
          <span class="file-path">${escapeHtml(item.path.split("/").pop())}</span>
        </button>`;
      }
      const tags = (opts.tags ? opts.tags(item.row, item.index) : ["all"]).map(
        (tag) => `<span class="file-tag">${escapeHtml(tag)}</span>`
      ).join("");
      return `<div class="file-row tree-file" style="--depth:${depth}" ${opts.rowAttr ? opts.rowAttr(item.row, item.index) : ""}>
        <span class="file-path">${escapeHtml(item.path.split("/").pop())}</span>
        <span class="file-actions">${opts.actions ? opts.actions(item.row, item.index) : ""}</span>
        <span class="file-tags">${tags}</span>
      </div>`;
    })
    .join("");
}

function getCollapsedTreeSet(treeId) {
  if (!collapsedFileTreeFolders[treeId]) collapsedFileTreeFolders[treeId] = new Set();
  return collapsedFileTreeFolders[treeId];
}

function toggleTreeFolder(treeId, path) {
  if (!path) return;
  const collapsed = getCollapsedTreeSet(treeId);
  if (collapsed.has(path)) collapsed.delete(path);
  else collapsed.add(path);
}

function hasCollapsedParent(path, collapsed) {
  const parts = String(path || "").split("/");
  for (let i = 1; i < parts.length; i += 1) {
    if (collapsed.has(parts.slice(0, i).join("/"))) return true;
  }
  return false;
}

function parseTagsInput(value) {
  return String(value || "")
    .split(/[\s,;]+/)
    .map((x) => x.trim().toLowerCase())
    .filter(Boolean);
}

function displayTags(tags) {
  const values = Array.isArray(tags) && tags.length ? tags : ["*"];
  return values.map((tag) => (tag === "*" ? "all" : tag));
}

async function loadFileTreeModule() {
  if (!fileTreeModulePromise) {
    fileTreeModulePromise = import("/vendor/file-tree/file-tree.js");
  }
  return fileTreeModulePromise;
}

async function renderVersionFileBrowser(files) {
  const container = document.getElementById("fileList");
  if (!container) return;
  if (fileTree) {
    fileTree.destroy();
    fileTree = null;
  }
  container.innerHTML = "";
  showFilePreviewMessage(files.length ? "点击左侧文件查看内容" : "当前版本无文件");
  if (!files.length) {
    container.innerHTML = `<p class="hint">暂无文件</p>`;
    return;
  }
  try {
    const { FileTree } = await loadFileTreeModule();
    fileTree = new FileTree(container, {
      data: files.map((path) => ({ path, type: "file" })),
      theme: "dark",
      dragAndDrop: false,
      toolbar: {
        createFile: false,
        createFolder: false,
        expandAll: true,
        collapseAll: true,
        custom: [],
      },
      contextMenu: false,
    });
    expandInitialFileTreeFolders(files);
    fileTree.on("select", (e) => {
      if (e.node?.type === "file") previewVersionFile(e.path);
    });
  } catch (e) {
    container.innerHTML = files.map((f) => `<button type="button" class="file-row" data-path="${escapeAttr(f)}">${escapeHtml(f)}</button>`).join("");
    container.querySelectorAll(".file-row").forEach((btn) => {
      btn.onclick = () => previewVersionFile(btn.dataset.path || "");
    });
  }
}

function expandInitialFileTreeFolders(files) {
  if (!fileTree) return;
  const roots = new Set();
  for (const file of files) {
    const root = String(file || "").split("/")[0];
    if (root && root !== file) roots.add(root);
  }
  roots.forEach((root) => fileTree.expand(root));
}

async function previewVersionFile(path) {
  if (!state.projectId || !state.version || !path) return;
  const requestID = ++filePreviewRequestID;
  setFilePreviewHeader(path, "加载中...");
  setHighlightedFileContent(path, "加载中...");
  try {
    const q = new URLSearchParams({ path });
    const d = await api(`/api/projects/${state.projectId}/versions/${encodeURIComponent(state.version)}/files/content?${q}`);
    if (requestID !== filePreviewRequestID) return;
    setFilePreviewHeader(d.path, formatBytes(d.size));
    setHighlightedFileContent(d.path, d.content || "");
  } catch (e) {
    if (requestID !== filePreviewRequestID) return;
    setFilePreviewHeader(path, "");
    setHighlightedFileContent(path, e.message || "无法预览");
  }
}

function showFilePreviewMessage(message) {
  setFilePreviewHeader("选择文件", "");
  setHighlightedFileContent("", message);
}

function setFilePreviewHeader(path, meta) {
  const title = document.getElementById("filePreviewPath");
  const hint = document.getElementById("filePreviewMeta");
  if (title) title.textContent = path || "选择文件";
  if (hint) hint.textContent = meta || "";
}

function setHighlightedFileContent(path, content) {
  const pre = document.getElementById("filePreviewBody");
  if (!pre) return;
  const lang = languageForPath(path);
  pre.className = `file-preview-body language-${lang}`;
  pre.innerHTML = `<code class="language-${lang}"></code>`;
  const code = pre.querySelector("code");
  code.textContent = content;
  if (window.Prism) Prism.highlightElement(code);
}

function languageForPath(path) {
  const lower = String(path || "").toLowerCase();
  const ext = lower.split(".").pop() || "";
  if (["yaml", "yml"].includes(ext)) return "yaml";
  if (ext === "json") return "json";
  if (["js", "mjs", "cjs"].includes(ext)) return "javascript";
  if (["html", "xml", "svg"].includes(ext)) return "markup";
  if (ext === "css") return "css";
  if (["sh", "bash", "cmd", "bat", "ps1"].includes(ext)) return "bash";
  if (ext === "go") return "go";
  if (["properties", "conf", "ini", "env"].includes(ext)) return "properties";
  return "none";
}

function formatBytes(size) {
  const n = Number(size) || 0;
  if (n < 1024) return `${n} B`;
  if (n < 1024 * 1024) return `${(n / 1024).toFixed(1)} KB`;
  if (n < 1024 * 1024 * 1024) return `${(n / 1024 / 1024).toFixed(1)} MB`;
  if (n < 1024 ** 4) return `${(n / 1024 ** 3).toFixed(1)} GB`;
  return `${(n / 1024 ** 4).toFixed(1)} TB`;
}

let dashboardRequestID = 0;
let dashboardActivityChart = null;
let dashboardFailureChart = null;
let dashboardChartObserver = null;

function populateDashboardProjects() {
  const select = document.getElementById("dashboardProject");
  if (!select) return;
  const current = select.value;
  select.innerHTML = `<option value="">全部可访问项目</option>${state.projects
    .map((project) => `<option value="${project.id}">${escapeHtml(project.name)}</option>`)
    .join("")}`;
  if (state.projects.some((project) => String(project.id) === current)) select.value = current;
}

function formatDashboardRate(value, total) {
  if (!total) return "—";
  return `${Number(value || 0).toFixed(1)}%`;
}

function formatDashboardDuration(millis) {
  const value = Number(millis) || 0;
  if (!value) return "—";
  if (value < 1000) return `${value} ms`;
  if (value < 60000) return `${(value / 1000).toFixed(1)} s`;
  return `${(value / 60000).toFixed(1)} min`;
}

function renderDashboardKpis(summary = {}) {
  const host = document.getElementById("dashboardKpis");
  if (!host) return;
  const items = [
    { label: "上传请求", value: summary.uploads || 0, meta: `${summary.upload_failures || 0} 次失败`, tone: summary.upload_failures ? "warn" : "ok" },
    { label: "上传数据", value: formatBytes(summary.upload_bytes || 0), meta: `${summary.uploaded_files || 0} 个文件`, tone: "info" },
    { label: "发布版本", value: summary.publishes || 0, meta: "已发布版本事件", tone: "primary" },
    { label: "节点拉取", value: summary.pulls || 0, meta: `成功率 ${formatDashboardRate(summary.pull_success_rate, summary.pulls)}`, tone: summary.pull_failures ? "warn" : "ok" },
    { label: "SSH 发布", value: summary.deployments || 0, meta: `成功率 ${formatDashboardRate(summary.deployment_success_rate, (summary.deployment_successes || 0) + (summary.deployment_failures || 0))}`, tone: summary.deployment_failures ? "warn" : "ok" },
    { label: "发布目标", value: summary.targets || 0, meta: `平均耗时 ${formatDashboardDuration(summary.average_deployment_millis)}`, tone: summary.target_failures ? "warn" : "info" },
  ];
  host.innerHTML = items.map((item) => `<article class="dashboard-kpi ${item.tone}">
    <span class="dashboard-kpi-label">${escapeHtml(item.label)}</span>
    <strong class="dashboard-kpi-value">${escapeHtml(item.value)}</strong>
    <span class="dashboard-kpi-meta">${escapeHtml(item.meta)}</span>
  </article>`).join("");
}

function renderDashboardChartFallback(host, series = []) {
  if (!host) return;
  const width = 960, height = 260, left = 46, right = 18, top = 16, bottom = 34;
  const plotWidth = width - left - right, plotHeight = height - top - bottom;
  const max = Math.max(1, ...series.flatMap((day) => [day.uploads || 0, day.pulls || 0, day.deployments || 0]));
  const x = (index) => left + (series.length > 1 ? (index / (series.length - 1)) * plotWidth : plotWidth / 2);
  const y = (value) => top + plotHeight - (Number(value || 0) / max) * plotHeight;
  const configs = [
    ["uploads", "upload", "上传"],
    ["pulls", "pull", "拉取"],
    ["deployments", "deploy", "SSH 发布"],
  ];
  const grid = Array.from({ length: 5 }, (_, index) => {
    const value = Math.round((max * (4 - index)) / 4);
    const gy = top + (plotHeight * index) / 4;
    return `<line class="chart-grid-line" x1="${left}" y1="${gy}" x2="${width - right}" y2="${gy}"/><text class="chart-axis-text" x="${left - 8}" y="${gy + 4}" text-anchor="end">${value}</text>`;
  }).join("");
  const labelStep = Math.max(1, Math.ceil(series.length / 6));
  const labels = series.map((day, index) => (index % labelStep === 0 || index === series.length - 1)
    ? `<text class="chart-axis-text" x="${x(index)}" y="${height - 9}" text-anchor="middle">${escapeHtml(day.date.slice(5))}</text>` : "").join("");
  const lines = configs.map(([key, cls, label]) => {
    const points = series.map((day, index) => `${x(index).toFixed(1)},${y(day[key]).toFixed(1)}`).join(" ");
    const dots = series.length <= 30 ? series.map((day, index) => `<circle class="chart-dot ${cls}" cx="${x(index).toFixed(1)}" cy="${y(day[key]).toFixed(1)}" r="2.8"><title>${escapeHtml(day.date)} · ${label} ${day[key] || 0}</title></circle>`).join("") : "";
    return `<polyline class="chart-series ${cls}" points="${points}"/>${dots}`;
  }).join("");
  const total = series.reduce((sum, day) => sum + (day.uploads || 0) + (day.pulls || 0) + (day.deployments || 0), 0);
  host.innerHTML = `<svg viewBox="0 0 ${width} ${height}" role="presentation">${grid}${labels}${lines}</svg>${total ? "" : '<div class="dashboard-chart-empty">当前筛选范围暂无发布活动</div>'}`;
}

function initDashboardCharts() {
  if (!window.echarts || dashboardChartObserver) return;
  const activity = document.getElementById("dashboardChart");
  const failure = document.getElementById("dashboardQualityChart");
  if (!activity || !failure) return;
  dashboardActivityChart = window.echarts.init(activity, null, { renderer: "svg" });
  dashboardFailureChart = window.echarts.init(failure, null, { renderer: "svg" });
  dashboardChartObserver = new ResizeObserver(() => {
    dashboardActivityChart?.resize();
    dashboardFailureChart?.resize();
  });
  dashboardChartObserver.observe(activity);
  dashboardChartObserver.observe(failure);
}

function dashboardChartOption(series, failureMode) {
  const dates = series.map((day) => String(day.date || "").slice(5));
  const lines = failureMode
    ? [
      { name: "拉取失败", key: "pull_failures", color: "#ef4444" },
      { name: "SSH 发布失败", key: "deployment_failures", color: "#f59e0b" },
      { name: "目标失败", key: "target_failures", color: "#a855f7" },
    ]
    : [
      { name: "上传", key: "uploads", color: "#10b981" },
      { name: "拉取", key: "pulls", color: "#3b82f6" },
      { name: "SSH 发布", key: "deployments", color: "#f59e0b" },
    ];
  return {
    animationDuration: 240,
    color: lines.map((line) => line.color),
    tooltip: { trigger: "axis", backgroundColor: "#171b22", borderColor: "#303844", textStyle: { color: "#e5e7eb" } },
    legend: { top: 4, right: 4, textStyle: { color: "#9ca3af", fontSize: 11 }, itemWidth: 10, itemHeight: 10 },
    grid: { top: 42, right: 18, bottom: 30, left: 42, containLabel: false },
    xAxis: { type: "category", boundaryGap: failureMode, data: dates, axisLine: { lineStyle: { color: "#303844" } }, axisLabel: { color: "#8792a2", fontSize: 10, interval: Math.max(0, Math.ceil(dates.length / 7) - 1) } },
    yAxis: { type: "value", minInterval: 1, axisLine: { show: false }, splitLine: { lineStyle: { color: "#262d37" } }, axisLabel: { color: "#8792a2", fontSize: 10 } },
    series: lines.map((line) => ({ name: line.name, type: failureMode ? "bar" : "line", stack: failureMode ? "failures" : undefined, smooth: !failureMode, symbol: "circle", symbolSize: 5, emphasis: { focus: "series" }, data: series.map((day) => Number(day[line.key] || 0)) })),
  };
}

function renderDashboardChart(series = []) {
  const activityHost = document.getElementById("dashboardChart");
  const failureHost = document.getElementById("dashboardQualityChart");
  if (!activityHost || !failureHost) return;
  initDashboardCharts();
  if (!dashboardActivityChart || !dashboardFailureChart) {
    renderDashboardChartFallback(activityHost, series);
    renderDashboardChartFallback(failureHost, series.map((day) => ({ ...day, uploads: day.pull_failures || 0, pulls: day.deployment_failures || 0, deployments: day.target_failures || 0 })));
    return;
  }
  dashboardActivityChart.setOption(dashboardChartOption(series, false), true);
  dashboardFailureChart.setOption(dashboardChartOption(series, true), true);
}

function renderDashboardHealth(health = {}, generatedAt = "") {
  const host = document.getElementById("dashboardHealthGrid");
  const freshness = document.getElementById("dashboardFreshness");
  if (!host) return;
  const pullNodes = Number(health.pull_nodes || 0);
  const pullOnline = Number(health.pull_online || 0);
  const sshHosts = Number(health.ssh_hosts || 0);
  const sshFailing = Number(health.ssh_failing || 0);
  const items = [
    { label: "Pull 在线", value: `${pullOnline}/${pullNodes}`, meta: Number(health.pull_drift || 0) ? `${health.pull_drift} 个节点待收敛` : "无配置漂移", tone: health.pull_drift ? "warn" : "ok" },
    { label: "SSH 存活（租户）", value: `${health.ssh_healthy || 0}/${sshHosts}`, meta: sshFailing ? `${sshFailing} 台检测失败` : `${health.ssh_unknown || 0} 台待首次检测`, tone: sshFailing ? "warn" : "info" },
    { label: "自动 Hook", value: health.hooks_enabled || 0, meta: Number(health.hooks_pending || 0) ? `${health.hooks_pending} 个等待防抖` : "无等待任务", tone: health.hooks_pending ? "warn" : "primary" },
    { label: "Hook 失败", value: health.hook_failures || 0, meta: health.latest_event_at ? `最近事件 ${health.latest_event_at}` : "当前筛选暂无事件", tone: health.hook_failures ? "warn" : "ok" },
  ];
  host.innerHTML = items.map((item) => `<article class="dashboard-health ${item.tone}"><span>${escapeHtml(item.label)}</span><strong>${escapeHtml(item.value)}</strong><small>${escapeHtml(item.meta)}</small></article>`).join("");
  if (freshness) freshness.textContent = generatedAt ? `刷新于 ${new Date(generatedAt).toLocaleString("zh-CN", { hour12: false })}` : "等待数据";
}

function dashboardStatusBadge(status) {
  const normalized = String(status || "").toLowerCase();
  const cls = ["success", "ok"].includes(normalized) ? "ok" : ["failed", "error"].includes(normalized) ? "warn" : "draft";
  const label = { success: "成功", ok: "成功", failed: "失败", error: "失败", running: "执行中", queued: "排队中" }[normalized] || normalized || "—";
  return `<span class="badge badge-${cls}">${escapeHtml(label)}</span>`;
}

function renderDashboardDaily(series = []) {
  const tbody = document.querySelector("#dashboardDailyTable tbody");
  if (!tbody) return;
  tbody.innerHTML = [...series].reverse().map((day) => `<tr>
    <td><code>${escapeHtml(day.date)}</code></td>
    <td>${day.uploads || 0}${day.upload_failures ? ` <span class="metric-failure">-${day.upload_failures}</span>` : ""}</td>
    <td>${formatBytes(day.upload_bytes || 0)}</td>
    <td>${day.publishes || 0}</td>
    <td>${day.pulls || 0}</td>
    <td class="${day.pull_failures ? "metric-failure" : ""}">${day.pull_failures || 0}</td>
    <td>${day.deployments || 0}</td>
    <td>${day.targets || 0}</td>
    <td class="${day.deployment_failures || day.target_failures ? "metric-failure" : ""}">${(day.deployment_failures || 0) + (day.target_failures || 0)}</td>
  </tr>`).join("");
}

function renderDashboardRecords(records = []) {
  const tbody = document.querySelector("#dashboardRecordsTable tbody");
  if (!tbody) return;
  const labels = { upload: "上传", publish: "发布", pull: "拉取", deploy: "SSH 发布" };
  tbody.innerHTML = records.length ? records.map((record) => {
    const amount = record.kind === "upload" ? `${formatBytes(record.bytes || 0)} · ${record.files || 0} 文件` : record.kind === "deploy" ? `${record.files || 0} 目标` : "—";
    const actor = [record.actor, record.server_id].filter(Boolean).join(" / ") || "—";
    return `<tr>
      <td class="dashboard-record-time">${escapeHtml(record.at)}</td>
      <td><span class="record-kind ${escapeAttr(record.kind)}">${escapeHtml(labels[record.kind] || record.kind)}</span></td>
      <td><strong>${escapeHtml(record.project)}</strong><span class="dashboard-record-sub">${escapeHtml(record.version || "—")}</span></td>
      <td>${escapeHtml(actor)}</td>
      <td>${escapeHtml(amount)}</td>
      <td>${dashboardStatusBadge(record.status)}</td>
      <td class="dashboard-record-detail" title="${escapeAttr(record.detail || "")}">${escapeHtml(record.detail || "—")}</td>
    </tr>`;
  }).join("") : `<tr><td colspan="7" class="table-empty">当前筛选范围暂无记录</td></tr>`;
}

async function loadDashboard() {
  const requestID = ++dashboardRequestID;
  const days = document.getElementById("dashboardDays")?.value || "30";
  const projectID = document.getElementById("dashboardProject")?.value || "";
  const query = new URLSearchParams({ days });
  if (projectID) query.set("project_id", projectID);
  document.getElementById("dashboardKpis")?.classList.add("loading");
  try {
    const dashboard = await api("/api/dashboard?" + query.toString());
    if (requestID !== dashboardRequestID) return;
    renderDashboardKpis(dashboard.summary);
    renderDashboardChart(dashboard.series || []);
    renderDashboardHealth(dashboard.health || {}, dashboard.generated_at);
    renderDashboardDaily(dashboard.series || []);
    renderDashboardRecords(dashboard.recent || []);
    const updated = document.getElementById("dashboardUpdatedAt");
    if (updated) updated.textContent = `统计周期 ${dashboard.days} 天 · 更新于 ${new Date(dashboard.generated_at).toLocaleString("zh-CN", { hour12: false })}`;
  } catch (error) {
    if (requestID === dashboardRequestID) showToast("加载数据大盘失败: " + error.message, "error");
  } finally {
    if (requestID === dashboardRequestID) document.getElementById("dashboardKpis")?.classList.remove("loading");
  }
}

document.getElementById("btnDashboardReload")?.addEventListener("click", loadDashboard);
document.getElementById("dashboardDays")?.addEventListener("change", loadDashboard);
document.getElementById("dashboardProject")?.addEventListener("change", loadDashboard);
window.setInterval(() => {
  if (state.globalView === "dashboard" && !document.hidden) loadDashboard();
}, 60000);

let guideDirectory = null;
let selectedGuideTopic = "";

async function loadGuideDirectory() {
  if (guideDirectory) {
    renderGuideDirectory();
    return;
  }
  const content = document.getElementById("guideContent");
  try {
    guideDirectory = await api("/api/agent/guide");
    selectedGuideTopic = guideDirectory.topics?.[0]?.id || "";
    renderGuideDirectory();
    if (selectedGuideTopic) await loadGuideTopic(selectedGuideTopic);
  } catch (error) {
    if (content) content.textContent = "官方指南加载失败: " + error.message;
  }
}

function renderGuideDirectory() {
  const host = document.getElementById("guideTopicList");
  if (!host || !guideDirectory) return;
  host.innerHTML = `<div class="guide-topic-head"><strong>${escapeHtml(guideDirectory.title || "官方接入指南")}</strong><small>${escapeHtml(guideDirectory.notice || "")}</small></div>${(guideDirectory.topics || []).map((topic) => `<button type="button" class="guide-topic ${topic.id === selectedGuideTopic ? "active" : ""}" data-guide-topic="${escapeAttr(topic.id)}"><strong>${escapeHtml(topic.title)}</strong><small>${escapeHtml(topic.summary)}</small></button>`).join("")}`;
  host.querySelectorAll("[data-guide-topic]").forEach((button) => {
    button.onclick = () => loadGuideTopic(button.dataset.guideTopic);
  });
}

async function loadGuideTopic(topicID) {
  if (!topicID) return;
  const content = document.getElementById("guideContent");
  if (!content) return;
  selectedGuideTopic = topicID;
  renderGuideDirectory();
  content.textContent = "正在加载官方指南…";
  try {
    const topic = await api("/api/agent/guide/" + encodeURIComponent(topicID));
    content.innerHTML = `<div class="guide-content-head"><h2>${escapeHtml(topic.title)}</h2><p>${escapeHtml(topic.summary || "")}</p></div><pre>${escapeHtml(topic.content || "")}</pre>`;
  } catch (error) {
    content.textContent = "官方指南加载失败: " + error.message;
  }
}

let previewDebounceTimer = null;
let renderedPreviewIndex = 0;

document.getElementById("verPreviewServerId")?.addEventListener("input", () => {
  const previewSid = document.getElementById("verPreviewServerId")?.value.trim() || "";
  state.selectedServerID = previewSid || null;
  state.previewConfirmed = false;
  const confirmButton = document.getElementById("btnConfirmPreview");
  if (confirmButton) confirmButton.disabled = true;
  updateWorkflowUI();
  const deploySid = document.getElementById("deployServerId");
  if (deploySid && !deploySid.value.trim()) deploySid.value = previewSid;
  generateDeployScript();
  scheduleDeployPreview();
});

function scheduleDeployPreview() {
  clearTimeout(previewDebounceTimer);
  previewDebounceTimer = setTimeout(runDeployPreviewAuto, 450);
}

async function runDeployPreviewAuto() {
  const sid = document.getElementById("verPreviewServerId")?.value.trim();
  if (!state.projectName || !state.version || !sid) return;
  try {
    const d = await fetchDeployPreview(state.projectName, state.version, sid);
    renderPreviewReport(d, document.getElementById("verPreviewTable"));
    const confirmButton = document.getElementById("btnConfirmPreview");
    if (confirmButton) confirmButton.disabled = false;
  } catch (_) {}
}

function renderPreviewReport(report, container) {
  const rendered = (report.rendered_files || []).map((f) => ({
    path: f.path || f.basename,
    basename: f.basename,
    from: f.before || "",
    to: f.after || "",
    change: f.before === f.after ? "unchanged" : "modified",
  })).filter((f) => f.change !== "unchanged");
  const meta = [];
  meta.push(`<strong>${escapeHtml(report.project)}</strong>`);
  meta.push(escapeHtml(report.version));
  meta.push(`<code>${escapeHtml(report.server_id)}</code>`);
  if (report.post_hook) meta.push(`post_hook <code>${escapeHtml(report.post_hook)}</code>`);
  const warnings = (report.warnings || []).map((w) => `<span class="diff-count warn">${escapeHtml(w)}</span>`).join("");
  renderFileDiffWorkspace(rendered, container, {
    empty: "无 replacements 或未匹配配置文件",
    summary: `${meta.join(" / ")} ${warnings}`,
    beforeTitle: "原版",
    afterTitle: "替换后",
    preservePreviewIds: container?.id === "verPreviewTable",
  });
}

function renderFileDiffWorkspace(files, container, opts = {}) {
  if (!container) return;
  const list = files || [];
  const oldSearch = container.querySelector("[data-diff-search]");
  const keepSearchFocus = oldSearch === document.activeElement;
  const oldCaret = keepSearchFocus ? oldSearch.selectionStart : null;
  const filter = String(container.__diffFilter || "").trim().toLowerCase();
  const visible = filterDiffFiles(list, filter);
  let selected = Math.min(container.__diffIndex || 0, Math.max(0, list.length - 1));
  if (visible.length && !visible.some((f) => f.__index === selected)) selected = visible[0].__index;
  container.__diffIndex = selected;
  const beforeId = opts.preservePreviewIds ? "verPreviewOriginalBody" : "";
  const afterId = opts.preservePreviewIds ? "verPreviewRenderedBody" : "";
  const testAttr = opts.preservePreviewIds ? 'data-testid="preview-rendered-body"' : "";
  container.classList.add("diff-workspace");
  container.innerHTML = `${opts.summary ? `<div class="diff-summary">${opts.summary}</div>` : ""}
    <aside class="diff-tree-panel">
      <div class="panel-label">文件树</div>
      <div class="diff-search-wrap">
        <input type="search" class="search-input diff-search-input" placeholder="搜索路径 / 变更…" value="${escapeAttr(filter)}" data-diff-search />
      </div>
      ${visible.length ? buildDiffTree(visible, selected, container, !!filter) : `<div class="empty-diff">${escapeHtml(list.length ? "没有匹配文件" : (opts.empty || "无差异"))}</div>`}
    </aside>
    <section class="diff-main">
      <div class="diff-pane">
        <div class="diff-pane-head">${escapeHtml(opts.beforeTitle || "旧版本")}</div>
        <pre ${beforeId ? `id="${beforeId}"` : ""} class="diff-code language-none"><code></code></pre>
      </div>
      <div class="diff-pane">
        <div class="diff-pane-head">${escapeHtml(opts.afterTitle || "新版本")}</div>
        <pre ${afterId ? `id="${afterId}"` : ""} class="diff-code language-none" ${testAttr}><code></code></pre>
      </div>
    </section>`;
  container.querySelectorAll("[data-diff-index]").forEach((btn) => {
    btn.onclick = () => {
      container.__diffIndex = Number(btn.dataset.diffIndex);
      renderFileDiffWorkspace(list, container, opts);
    };
  });
  const search = container.querySelector("[data-diff-search]");
  if (search) {
    search.oninput = () => {
      container.__diffFilter = search.value;
      renderFileDiffWorkspace(list, container, opts);
    };
    if (keepSearchFocus) {
      search.focus();
      const caret = oldCaret == null ? search.value.length : oldCaret;
      search.setSelectionRange(caret, caret);
    }
  }
  container.querySelectorAll("[data-diff-folder]").forEach((btn) => {
    btn.onclick = () => {
      toggleDiffFolder(container, btn.dataset.diffFolder);
      renderFileDiffWorkspace(list, container, opts);
    };
  });
  if (!visible.length) {
    const msg = list.length ? "没有匹配文件" : (opts.empty || "无差异");
    const emptyRows = [{ no: "", text: msg, cls: "no" }];
    renderCodeLines(container.querySelector(".diff-pane:first-child .diff-code code"), emptyRows);
    renderCodeLines(container.querySelector(".diff-pane:last-child .diff-code code"), emptyRows);
    return;
  }
  const current = list[selected];
  const [left, right] = buildSideBySideDiff(current.from || "", current.to || "");
  renderCodeLines(container.querySelector(".diff-pane:first-child .diff-code code"), left);
  renderCodeLines(container.querySelector(".diff-pane:last-child .diff-code code"), right);
}

function filterDiffFiles(files, query) {
  const q = String(query || "").trim().toLowerCase();
  return (files || [])
    .map((f, index) => ({ ...f, __index: index }))
    .filter((f) => !q || `${f.path || ""} ${f.basename || ""} ${f.change || ""}`.toLowerCase().includes(q));
}

function buildDiffTree(files, selected, container, forceExpand = false) {
  const folders = new Set();
  files.forEach((f) => {
    const parts = String(f.path).split("/");
    for (let i = 1; i < parts.length; i += 1) folders.add(parts.slice(0, i).join("/"));
  });
  const rows = [
    ...[...folders].map((path) => ({ path, type: "folder" })),
    ...files.map((f) => ({ ...f, index: f.__index, type: "file" })),
  ].sort((a, b) => a.path.localeCompare(b.path));
  const collapsed = forceExpand ? new Set() : getDiffCollapsedSet(container);
  return `<div class="diff-tree">${rows.map((item) => {
    if (hasCollapsedParent(item.path, collapsed)) return "";
    const depth = Math.max(0, String(item.path).split("/").length - 1);
    if (item.type === "folder") {
      const isCollapsed = collapsed.has(item.path);
      return `<button type="button" class="diff-tree-row folder" style="padding-left:${0.45 + depth * 1.1}rem" data-diff-folder="${escapeAttr(item.path)}" aria-expanded="${isCollapsed ? "false" : "true"}">
        <span class="diff-tree-name"><span class="tree-caret" aria-hidden="true">${isCollapsed ? "▸" : "▾"}</span>${escapeHtml(item.path.split("/").pop())}</span>
      </button>`;
    }
    return `<button type="button" class="diff-tree-row ${item.index === selected ? "active" : ""}" style="padding-left:${0.45 + depth * 1.1}rem" data-diff-index="${item.index}">
      <span class="diff-tree-name">${escapeHtml(item.path.split("/").pop())}</span>
      <span class="diff-tree-badges"><span class="file-tag">${escapeHtml(item.change || "modified")}</span></span>
    </button>`;
  }).join("")}</div>`;
}

function getDiffCollapsedSet(container) {
  if (!container) return new Set();
  let set = collapsedFileTreeFolders.diff.get(container);
  if (!set) {
    set = new Set();
    collapsedFileTreeFolders.diff.set(container, set);
  }
  return set;
}

function toggleDiffFolder(container, path) {
  if (!path) return;
  const collapsed = getDiffCollapsedSet(container);
  if (collapsed.has(path)) collapsed.delete(path);
  else collapsed.add(path);
}

function buildSideBySideDiff(before, after) {
  const a = splitLines(before);
  const b = splitLines(after);
  if (a.length * b.length > 120000) return buildSimpleDiff(a, b);
  const dp = Array.from({ length: a.length + 1 }, () => Array(b.length + 1).fill(0));
  for (let i = a.length - 1; i >= 0; i -= 1) {
    for (let j = b.length - 1; j >= 0; j -= 1) {
      dp[i][j] = a[i] === b[j] ? dp[i + 1][j + 1] + 1 : Math.max(dp[i + 1][j], dp[i][j + 1]);
    }
  }
  const left = [];
  const right = [];
  let i = 0;
  let j = 0;
  while (i < a.length || j < b.length) {
    if (i < a.length && j < b.length && a[i] === b[j]) {
      left.push({ no: i + 1, text: a[i], cls: "" });
      right.push({ no: j + 1, text: b[j], cls: "" });
      i += 1;
      j += 1;
    } else if (j < b.length && (i === a.length || dp[i][j + 1] >= dp[i + 1][j])) {
      left.push({ no: "", text: "", cls: "no" });
      right.push({ no: j + 1, text: b[j], cls: "added" });
      j += 1;
    } else {
      left.push({ no: i + 1, text: a[i], cls: "removed" });
      right.push({ no: "", text: "", cls: "no" });
      i += 1;
    }
  }
  return [left, right];
}

function buildSimpleDiff(a, b) {
  const max = Math.max(a.length, b.length);
  const left = [];
  const right = [];
  for (let i = 0; i < max; i += 1) {
    const same = a[i] === b[i];
    left.push({ no: i < a.length ? i + 1 : "", text: a[i] || "", cls: same ? "" : (i < a.length ? "changed" : "no") });
    right.push({ no: i < b.length ? i + 1 : "", text: b[i] || "", cls: same ? "" : (i < b.length ? "changed" : "no") });
  }
  return [left, right];
}

function splitLines(text) {
  const lines = String(text || "").replace(/\r\n/g, "\n").split("\n");
  if (lines.length && lines[lines.length - 1] === "") lines.pop();
  return lines.length ? lines : [""];
}

function renderCodeLines(code, rows) {
  if (!code) return;
  code.innerHTML = rows.map((row) => `<span class="diff-line ${row.cls || ""}">
    <span class="diff-line-num">${escapeHtml(row.no)}</span>
    <span class="diff-line-text">${escapeHtml(row.text || "")}</span>
  </span>`).join("");
}

function escapeHtml(s) {
  return String(s).replace(/&/g, "&amp;").replace(/</g, "&lt;").replace(/>/g, "&gt;");
}

function escapeAttr(s) {
  return escapeHtml(s).replace(/"/g, "&quot;");
}

async function fetchDeployPreview(project, version, serverId) {
  const q = new URLSearchParams({ project, version, server_id: serverId });
  return api("/api/deploy/preview?" + q);
}

document.getElementById("btnVerPreview").onclick = async () => {
  const sid = document.getElementById("verPreviewServerId").value.trim();
  if (!state.projectName || !state.version || !sid) {
    showToast("请选择版本并填写 server_id", "warn");
    return;
  }
  try {
    const d = await fetchDeployPreview(state.projectName, state.version, sid);
    renderPreviewReport(d, document.getElementById("verPreviewTable"));
    state.selectedServerID = sid;
    state.previewConfirmed = false;
    document.getElementById("btnConfirmPreview").disabled = false;
    updateWorkflowUI();
  } catch (e) {
    showToast(e.message, "error");
  }
};

document.getElementById("btnConfirmPreview")?.addEventListener("click", () => {
  state.previewConfirmed = true;
  updateWorkflowUI();
  navigateProjectTab("publish");
  showToast("合成预览已确认");
});

document.getElementById("btnFlowValidate")?.addEventListener("click", () => {
  if (!state.version) return navigateProjectTab("versions");
  document.getElementById("btnValidate")?.click();
});

document.getElementById("btnFlowPublish")?.addEventListener("click", () => {
  document.getElementById("btnPublish")?.click();
});

function tryAutoPreviewOnVersionSelect() {
  const sid = document.getElementById("verPreviewServerId")?.value.trim();
  if (sid) scheduleDeployPreview();
}

document.getElementById("btnDelProject").onclick = async () => {
  if (!state.projectId) return;
  if (!(await showConfirm({ title: "删除项目", message: "删除项目及所有版本？", confirmText: "删除", danger: true }))) return;
  await api(`/api/projects/${state.projectId}`, { method: "DELETE" });
  state.projectId = null;
  setGlobalView("workspace");
};

document.getElementById("btnAddProject").onclick = async () => {
  const name = document.getElementById("newProject").value.trim();
  if (!name) return;
  if (name.includes("|")) {
    showToast("项目名称不能包含 |，该字符用于 project|serverId 目标标签", "warn");
    return;
  }
  await api("/api/projects", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ name }),
  });
  document.getElementById("newProject").value = "";
  loadProjects();
};

document.getElementById("btnAddVersion").onclick = async () => {
  const button = document.getElementById("btnAddVersion");
  const input = document.getElementById("newVersion");
  const name = input.value.trim();
  if (!name || !state.projectId) return;
  if (!/^\d+\.\d+\.\d+$/.test(name)) {
    showToast("正式版本推荐使用 X.Y.Z（例如 0.0.1）；server_id 差异写 server.yaml", "warn");
  }
  button.disabled = true;
  try {
    await api(`/api/projects/${state.projectId}/versions`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ name }),
    });
    if (input.value.trim() === name) input.value = "";
    await selectProject({ id: state.projectId, name: state.projectName });
  } finally {
    button.disabled = !canWriteProject();
  }
};

document.getElementById("btnValidate").onclick = async () => {
  if (!state.projectId || !state.version) return;
  try {
    const r = await api(`/api/projects/${state.projectId}/versions/${encodeURIComponent(state.version)}/validate`);
    const el = document.getElementById("validateResult");
    let html = r.ok ? "<p class='hint'>✓ 可以发布</p>" : "<p class='warn'>✗ 不可发布</p>";
    (r.errors || []).forEach((e) => (html += `<p class="warn">${escapeHtml(e)}</p>`));
    (r.warnings || []).forEach((w) => (html += `<p class="hint">⚠ ${escapeHtml(w)}</p>`));
    el.innerHTML = html;
  } catch (e) {
    showToast(e.message, "error");
  }
};

document.getElementById("btnSubmitReview")?.addEventListener("click", async () => {
  if (!state.projectId || !state.version) return;
  await api(`/api/projects/${state.projectId}/versions/${encodeURIComponent(state.version)}/submit-review`, {
    method: "POST",
  });
  selectProject({ id: state.projectId, name: state.projectName });
});

document.getElementById("btnRejectReview")?.addEventListener("click", async () => {
  if (!state.projectId || !state.version) return;
  if (!(await showConfirm({ title: "驳回版本", message: "确认驳回当前版本？", confirmText: "驳回" }))) return;
  await api(`/api/projects/${state.projectId}/versions/${encodeURIComponent(state.version)}/reject`, { method: "POST" });
  selectProject({ id: state.projectId, name: state.projectName });
});

document.getElementById("btnPublish").onclick = async () => {
  if (!(await showConfirm({ title: "正式发布", message: "发布后不可修改。确认发布当前版本？", confirmText: "发布" }))) return;
  const ver = state.version;
  await api(`/api/projects/${state.projectId}/versions/${encodeURIComponent(ver)}/publish`, { method: "POST" });
  await selectProject({ id: state.projectId, name: state.projectName });
  const versions = (await api(`/api/projects/${state.projectId}/versions`)) || [];
  const published = versions.find((v) => v.version === ver);
  if (published) await selectVersion(published);
};

document.getElementById("btnVersionDiff")?.addEventListener("click", async () => {
  const from = document.getElementById("diffFromVer")?.value;
  const to = document.getElementById("diffToVer")?.value;
  const sid = document.getElementById("diffServerId")?.value.trim();
  if (!state.projectName || !from || !to || !sid) {
    showToast("选择 from/to 版本并填写 server_id", "warn");
    return;
  }
  if (from === to) {
    showToast("from 和 to 不能相同", "warn");
    return;
  }
  try {
    const q = new URLSearchParams({ project: state.projectName, from, to, server_id: sid });
    const d = await api("/api/deploy/diff?" + q);
    renderVersionDiff(d, document.getElementById("versionDiffOut"));
  } catch (e) {
    showToast(e.message, "error");
  }
});

function renderVersionDiff(report, container) {
  if (!container) return;
  const files = (report.file_diffs || []).map((f) => ({
    path: f.path || f.basename,
    basename: f.basename,
    from: f.from || "",
    to: f.to || "",
    change: f.change || "modified",
  }));
  const keyChanges = (report.files || []).reduce((n, f) => n + (f.keys || []).length, 0);
  renderFileDiffWorkspace(files, container, {
    empty: "无文件差异",
    summary: `<strong>${escapeHtml(report.from_version)}</strong> → <strong>${escapeHtml(report.to_version)}</strong> <code>${escapeHtml(report.server_id)}</code> <span class="diff-count">${files.length} 个文件</span> <span class="diff-count">${keyChanges} 个配置键</span>`,
    beforeTitle: `旧版本 ${report.from_version}`,
    afterTitle: `新版本 ${report.to_version}`,
  });
}

document.getElementById("btnDeleteVersion").onclick = async () => {
  if (!(await showConfirm({ title: "删除版本", message: `删除版本 ${state.version}？`, confirmText: "继续", danger: true }))) return;
  if (!(await showConfirm({ title: "再次确认", message: "该操作会删除版本文件，确认继续？", confirmText: "删除", danger: true }))) return;
  await api(`/api/projects/${state.projectId}/versions/${state.version}?confirm=yes`, { method: "DELETE" });
  selectProject({ id: state.projectId, name: state.projectName });
};

async function uploadFiles(files) {
  const tags = parseTagsInput(document.getElementById("uploadTags")?.value || "");
  for (const file of files) {
    await uploadOneFileWithRetry(file, tags, 3);
  }
  selectVersion({ version: state.version, status: state.versionStatus, created_at: "", published_at: "" });
}

async function uploadOneFileWithRetry(file, tags, retries) {
  let lastErr = null;
  for (let attempt = 1; attempt <= retries; attempt += 1) {
    try {
      await uploadOneFile(file, tags, attempt);
      return;
    } catch (err) {
      lastErr = err;
      if (attempt < retries) {
        showToast(`${file.name} 上传失败，正在重试 ${attempt + 1}/${retries}`, "warn", 2200);
        await sleep(650 * attempt);
      }
    }
  }
  throw lastErr;
}

function uploadOneFile(file, tags, attempt) {
  return new Promise((resolve, reject) => {
    const fd = new FormData();
    fd.append("file", file);
    fd.append("path", file.name);
    tags.forEach((tag) => fd.append("tags", tag));
    const xhr = new XMLHttpRequest();
    xhr.open("POST", `/api/projects/${state.projectId}/versions/${state.version}/files`);
    xhr.withCredentials = true;
    xhr.timeout = 5 * 60 * 1000;
    const t = getToken();
    if (t) xhr.setRequestHeader("Authorization", `Bearer ${t}`);
    xhr.onload = () => {
      if (xhr.status >= 200 && xhr.status < 300) {
        if (attempt > 1) showToast(`${file.name} 上传成功`, "ok");
        resolve();
        return;
      }
      reject(new Error(xhr.responseText || `upload failed ${xhr.status}`));
    };
    xhr.onerror = () => reject(new Error(`${file.name} 网络错误`));
    xhr.ontimeout = () => reject(new Error(`${file.name} 上传超时`));
    xhr.send(fd);
  });
}

function sleep(ms) {
  return new Promise((resolve) => window.setTimeout(resolve, ms));
}

document.getElementById("fileList")?.addEventListener("click", async (e) => {
  const btn = e.target.closest("[data-action]");
  if (!state.projectId || !state.version) return;
  if (!btn) {
    const item = e.target.closest("[data-preview-index]");
    const row = item ? state.fileRows[Number(item.dataset.previewIndex)] : null;
    if (row) previewVersionFile(row.path);
    return;
  }
  const row = state.fileRows[Number(btn.dataset.index)];
  if (!row) return;
  try {
    if (btn.dataset.action === "edit-tags") {
      const next = await showPrompt({
        title: "设置文件标签",
        message: `${row.path}（逗号分隔，空=all）`,
        value: displayTags(row.tags).join(","),
      });
      if (next === null) return;
      await api(`/api/projects/${state.projectId}/versions/${encodeURIComponent(state.version)}/file-tags`, {
        method: "PUT",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ path: row.path, tags: parseTagsInput(next) }),
      });
      await loadVersionFileTags();
    }
    if (btn.dataset.action === "clear-tags") {
      await api(`/api/projects/${state.projectId}/versions/${encodeURIComponent(state.version)}/file-tags?path=${encodeURIComponent(row.path)}`, {
        method: "DELETE",
      });
      await loadVersionFileTags();
    }
  } catch (err) {
    showToast(err.message, "error");
  }
});

document.getElementById("btnApplyFileTags")?.addEventListener("click", async () => {
  if (!state.projectId || !state.version) return;
  const raw = document.getElementById("tagBatchPaths")?.value || "";
  const items = raw.split(/\r?\n/).map((x) => x.trim()).filter(Boolean);
  const patterns = items.filter((x) => /[*?[\]]/.test(x) || x.endsWith("/**"));
  const paths = items.filter((x) => !patterns.includes(x));
  const tags = parseTagsInput(document.getElementById("tagBatchTags")?.value || "");
  const mode = document.getElementById("tagBatchMode")?.value || "set";
  try {
    await api(`/api/projects/${state.projectId}/versions/${encodeURIComponent(state.version)}/file-tags/batch`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ paths, patterns, tags, mode }),
    });
    await loadVersionFileTags();
  } catch (err) {
    showToast(err.message, "error");
  }
});

document.getElementById("fileInput").onchange = async (e) => {
  const uploadActions = [document.getElementById("btnValidate"), document.getElementById("btnPublish")].filter(Boolean);
  const previousDisabled = uploadActions.map((button) => button.disabled);
  uploadActions.forEach((button) => { button.disabled = true; });
  try {
    await uploadFiles(e.target.files);
  } catch (err) {
    showToast(err.message, "error");
  } finally {
    uploadActions.forEach((button, index) => { button.disabled = previousDisabled[index]; });
    // Selecting the same local file for another version must fire change again.
    // File inputs cannot be assigned programmatically except to clear them.
    e.target.value = "";
  }
};

const dropZone = document.getElementById("versionDetail");
if (dropZone) {
  dropZone.addEventListener("dragover", (e) => {
    e.preventDefault();
    dropZone.classList.add("drag");
  });
  dropZone.addEventListener("dragleave", () => dropZone.classList.remove("drag"));
  dropZone.addEventListener("drop", async (e) => {
    e.preventDefault();
    dropZone.classList.remove("drag");
    if (!state.version) return;
    try {
      await uploadFiles(e.dataTransfer.files);
    } catch (err) {
      showToast(err.message, "error");
    }
  });
}

async function loadServerYaml() {
  const d = await api("/api/server-yaml");
  document.getElementById("serverYaml").value = d.content;
}

document.getElementById("btnSaveYaml").onclick = async () => {
  await api("/api/server-yaml", {
    method: "PUT",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ content: document.getElementById("serverYaml").value }),
  });
  showToast("已保存", "ok");
  loadServerIDs();
};

document.getElementById("btnPreview").onclick = async () => {
  const project = document.getElementById("previewProject").value.trim();
  const version = document.getElementById("previewVersion").value.trim();
  const serverId = document.getElementById("previewServerId").value.trim();
  if (!project || !version || !serverId) {
    showToast("填写 project / version / server_id", "warn");
    return;
  }
  try {
    const d = await fetchDeployPreview(project, version, serverId);
    renderPreviewReport(d, document.getElementById("previewOut"));
  } catch (e) {
    showToast(e.message, "error");
  }
};

async function loadUsers() {
  const users = await api("/api/users");
  const tbody = document.querySelector("#userTable tbody");
  tbody.innerHTML = "";
  users.forEach((u) => {
    const tr = document.createElement("tr");
    const hasNewToken = recentUserTokens.has(Number(u.id));
    tr.innerHTML = `<td>${u.id}</td><td>${escapeHtml(u.username)}</td><td><span class="secret-masked">${hasNewToken ? "新 Token 已生成" : "已隐藏"}</span></td>
      <td>${hasNewToken ? `<button data-id="${u.id}" class="btn btn-primary btn-sm copy-token">复制新 Token</button>` : ""}
      <button data-id="${u.id}" data-username="${escapeAttr(u.username)}" class="btn btn-secondary btn-sm refresh">刷新 Token</button>
      <button data-id="${u.id}" class="btn btn-danger btn-sm del">删除</button></td>`;
    tbody.appendChild(tr);
  });
  tbody.querySelectorAll(".copy-token").forEach((button) => {
    button.onclick = async () => {
      const id = Number(button.dataset.id);
      const token = recentUserTokens.get(id);
      if (!token) return;
      try {
        await navigator.clipboard.writeText(token);
        recentUserTokens.delete(id);
        showToast("新 Token 已复制；离开后不可再次查看", "ok", 4800);
        loadUsers();
      } catch (_) {
        showToast("浏览器拒绝剪贴板访问，请允许权限后重试", "warn", 4800);
      }
    };
  });
  tbody.querySelectorAll(".refresh").forEach((b) => {
    b.onclick = async () => {
      const confirmed = await showConfirm({
        title: "刷新拉取 Token",
        message: `刷新 ${b.dataset.username || "该账号"} 的 Token 后，使用旧 Token 的节点会立即失效。确认继续？`,
        confirmText: "刷新 Token",
        danger: true,
      });
      if (!confirmed) return;
      const result = await api(`/api/users/${b.dataset.id}/refresh-token`, { method: "POST" });
      recentUserTokens.set(Number(b.dataset.id), result.token);
      loadUsers();
    };
  });
  tbody.querySelectorAll(".del").forEach((b) => {
    b.onclick = async () => {
      if (!(await showConfirm({ title: "删除用户", message: "确认删除该账号？", confirmText: "删除", danger: true }))) return;
      await api(`/api/users/${b.dataset.id}`, { method: "DELETE" });
      loadUsers();
    };
  });
}

document.getElementById("btnAddUser").onclick = async () => {
  const created = await api("/api/users", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({
      username: document.getElementById("newUser").value,
      password: document.getElementById("newUserPass").value,
      role: document.getElementById("newUserRole").value,
      is_admin: document.getElementById("newUserAdmin").checked,
    }),
  });
  recentUserTokens.set(Number(created.id), created.token);
  document.getElementById("newUser").value = "";
  document.getElementById("newUserPass").value = "";
  loadUsers();
  showToast("账号已创建；请复制一次性显示的新 Token", "ok", 4800);
};

async function loadAuditLogs() {
  if (!state.isAdmin) return;
  const rows = (await api("/api/audit-logs")) || [];
  const tbody = document.querySelector("#auditTable tbody");
  tbody.innerHTML = rows
    .map(
      (r) =>
        `<tr><td>${escapeHtml(r.at)}</td><td>${escapeHtml(r.username)}</td><td>${escapeHtml(r.action)}</td><td class="audit-detail" title="${escapeHtml(r.detail)}">${escapeHtml(r.detail)}</td><td>${escapeHtml(r.ip || "")}</td></tr>`
    )
    .join("");
}

document.getElementById("btnReloadAudit")?.addEventListener("click", loadAuditLogs);

function formatLoginAttemptTimes(times) {
  if (!Array.isArray(times) || times.length === 0) return "-";
  return times.map((time) => `<small>${escapeHtml(time)}</small>`).join("");
}

async function loadLoginProtection() {
  if (!state.isAdmin) return;
  try {
    const [settings, attacks] = await Promise.all([
      api("/api/security/login-protection"),
      api("/api/security/login-ip-bans"),
    ]);
    const enabled = document.getElementById("loginProtectionEnabled");
    if (enabled) enabled.checked = !!settings.enabled;
    const summary = document.getElementById("loginProtectionSummary");
    if (summary) summary.textContent = settings.enabled
      ? `已启用：同一 IP 连续失败 ${settings.failure_limit} 次后暂停 ${settings.ban_seconds} 秒。`
      : "仅记录失败来源，不阻止登录。";
    const tbody = document.querySelector("#loginAttackTable tbody");
    if (!tbody) return;
    tbody.innerHTML = (attacks || []).map((attack) => {
      const active = attack.banned_until && Date.parse(attack.banned_until) > Date.now();
      return `<tr>
        <td><code>${escapeHtml(attack.ip)}</code></td>
        <td>${escapeHtml(attack.username || "-")}</td>
        <td>${Number(attack.attempt_count || 0)}</td>
        <td>${Number(attack.failures || 0)}</td>
        <td>${active ? `<span class="badge badge-warn">暂停至 ${escapeHtml(attack.banned_until)}</span>` : "记录中"}</td>
        <td>${formatLoginAttemptTimes(attack.last_attempt_times)}</td>
        <td><button type="button" class="btn btn-ghost btn-sm" data-login-ip="${escapeAttr(attack.ip)}">清除</button></td>
      </tr>`;
    }).join("") || "<tr><td colspan=\"7\" class=\"empty\">暂无失败来源</td></tr>";
    tbody.querySelectorAll("[data-login-ip]").forEach((button) => {
      button.onclick = async () => {
        const ip = button.dataset.loginIp;
        if (!(await showConfirm({ title: "清除登录记录", message: `清除 ${ip} 的登录失败记录？`, confirmText: "清除", danger: true }))) return;
        await api(`/api/security/login-ip-bans/${encodeURIComponent(ip)}`, { method: "DELETE" });
        loadLoginProtection();
      };
    });
  } catch (error) {
    showToast(error.message, "error");
  }
}

document.getElementById("loginProtectionEnabled")?.addEventListener("change", async (event) => {
  const enabled = event.target;
  try {
    const settings = await api("/api/security/login-protection", {
      method: "PUT",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ enabled: enabled.checked }),
    });
    enabled.checked = !!settings.enabled;
    await loadLoginProtection();
  } catch (error) {
    enabled.checked = !enabled.checked;
    showToast(error.message, "error");
  }
});

document.getElementById("btnReloadLoginProtection")?.addEventListener("click", loadLoginProtection);

async function loadSystemUpdateStatus() {
  if (!state.isRoot) return;
  const panel = document.getElementById("systemMaintenance");
  if (panel) panel.classList.remove("hidden");
  try {
    const st = await api("/api/system/update");
    renderSystemUpdateStatus(st);
  } catch (e) {
    const out = document.getElementById("systemUpdateStatus");
    if (out) out.textContent = e.message;
  }
}

function renderSystemUpdateStatus(st) {
  const versionEl = document.getElementById("currentServerVersion");
  if (versionEl) versionEl.textContent = `${st.current_version || "dev"} ${st.current_commit || ""}`.trim();
  const btn = document.getElementById("btnSystemUpdate");
  if (btn) btn.disabled = !!st.running;
  const lines = [];
  lines.push(st.running ? "状态: 更新中" : st.finished_at ? (st.ok ? "状态: 完成" : "状态: 失败") : "状态: 空闲");
  if (st.target_version) lines.push(`目标: ${st.target_version}`);
  if (st.started_at) lines.push(`开始: ${st.started_at}`);
  if (st.finished_at) lines.push(`结束: ${st.finished_at}`);
  if (st.error) lines.push(`错误: ${st.error}`);
  if (st.output) lines.push("", st.output);
  const out = document.getElementById("systemUpdateStatus");
  if (out) out.textContent = lines.join("\n");
  if (st.running) setTimeout(loadSystemUpdateStatus, 2000);
}

document.getElementById("btnSystemUpdateStatus")?.addEventListener("click", loadSystemUpdateStatus);

document.getElementById("btnSystemUpdate")?.addEventListener("click", async () => {
  if (!state.isRoot) return;
  if (!(await showConfirm({
    title: "更新中央服",
    message: "更新到最新 Release？更新过程中服务会短暂重启。",
    confirmText: "开始更新",
  }))) return;
  const btn = document.getElementById("btnSystemUpdate");
  if (btn) btn.disabled = true;
  try {
    const st = await api("/api/system/update", { method: "POST" });
    renderSystemUpdateStatus(st);
  } catch (e) {
    showToast(e.message, "error");
    if (btn) btn.disabled = false;
  }
});

document.getElementById("btnChangeMyPass")?.addEventListener("click", async () => {
  await api("/api/me/password", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({
      old_password: document.getElementById("myOldPass").value,
      new_password: document.getElementById("myNewPass").value,
    }),
  });
  showToast("密码已更新", "ok");
});

async function parseInviteHash() {
  const m = (location.hash || "").match(/invite\?token=([^&]+)/);
  if (!m) return;
  const token = decodeURIComponent(m[1]);
  state.pendingInviteToken = token;
  try {
    const info = await fetch("/api/project-invites/" + encodeURIComponent(token)).then((r) =>
      r.ok ? r.json() : r.json().then((j) => Promise.reject(new Error(j.error || r.statusText)))
    );
    const banner = document.getElementById("inviteBanner");
    const text = document.getElementById("inviteBannerText");
    if (!banner || !text) return;
    banner.classList.remove("hidden");
    text.textContent = `邀请加入「${info.project_name}」· ${info.role === "admin" ? "管理员" : "只读"}${info.expired ? "（已过期）" : ""}`;
    document.getElementById("btnAcceptInvite").disabled = !!info.expired;
  } catch (e) {
    showToast("邀请无效: " + e.message, "error");
  }
}

document.getElementById("btnAcceptInvite")?.addEventListener("click", async () => {
  if (!state.pendingInviteToken) return;
  try {
    const p = await api("/api/project-invites/" + encodeURIComponent(state.pendingInviteToken) + "/accept", {
      method: "POST",
    });
    state.pendingInviteToken = null;
    location.hash = "";
    document.getElementById("inviteBanner")?.classList.add("hidden");
    await loadProjects();
    const found = state.projects.find((x) => x.name === p.name);
    if (found) selectProject(found);
  } catch (e) {
    showToast(e.message, "error");
    if (e.message.includes("登录")) showLogin();
  }
});

// ═══════════ Onboarding / Demo Project ═══════════
const ONBOARDING_VERSION = "v1";
const DEMO_PROJECT = "demo-game";
const DEMO_SERVER_ID = "game-logic-01";

function onboardingKey() {
  const tenant = state.tenantSlug || "default";
  const username = state.username || "user";
  return `express233_onboarding_${ONBOARDING_VERSION}_${tenant}_${username}`;
}

function hasSeenOnboarding() {
  return !!localStorage.getItem(onboardingKey());
}

function markOnboardingSeen(status = "done") {
  localStorage.setItem(onboardingKey(), JSON.stringify({ status, at: new Date().toISOString() }));
}

function scheduleOnboarding() {
  if (navigator.webdriver || hasSeenOnboarding()) return;
  window.setTimeout(() => startOnboarding({ force: false }), 1200);
}

function demoServerYaml() {
  return `servers:
  game-logic-01:
    replacements:
      game.properties:
        server.id: game-logic-01
        server.port: "9001"
        db.host: db-ecs-a.internal
      application.yaml:
        mysql.url: jdbc:mysql://db-ecs-a.internal:3306/game
        game.serverId: game-logic-01
        game.listenPort: 9001
        game.featureFlags.hotfixReward: true
      settings.json:
        shard.id: game-logic-01
        network.publicHost: logic-01.example.internal
        tuning.maxPlayers: 500
    post_hook: scripts/restart.sh
    post_hook_env:
      SERVER_ID: game-logic-01

  game-logic-02:
    replacements:
      game.properties:
        server.id: game-logic-02
        server.port: "9002"
        db.host: db-ecs-b.internal
      application.yaml:
        mysql.url: jdbc:mysql://db-ecs-b.internal:3306/game
        game.serverId: game-logic-02
        game.listenPort: 9002
      settings.json:
        shard.id: game-logic-02
        network.publicHost: logic-02.example.internal
        tuning.maxPlayers: 650
    post_hook: scripts/restart.sh
    post_hook_env:
      SERVER_ID: game-logic-02

  game-logic-03:
    replacements:
      game.properties:
        server.id: game-logic-03
        server.port: "9003"
        db.host: db-ecs-c.internal
      application.yaml:
        mysql.url: jdbc:mysql://db-ecs-c.internal:3306/game
        game.serverId: game-logic-03
        game.listenPort: 9003
      settings.json:
        shard.id: game-logic-03
        network.publicHost: logic-03.example.internal
        tuning.maxPlayers: 800
    post_hook: scripts/restart.sh
    post_hook_env:
      SERVER_ID: game-logic-03`;
}

function demoFiles(version) {
  const maxPlayers = version === "2.0.0" ? 600 : version === "1.1.0" ? 520 : 500;
  return [
    [
      "game.properties",
      `# Game Server Configuration
server.id=template
server.port=8000
db.host=db-template.internal
db.port=3306
log.level=info
max.players=${maxPlayers}
`,
    ],
    [
      "application.yaml",
      `mysql:
  url: jdbc:mysql://template-db.internal:3306/game_tpl
  username: game_rw
  password: changeme

game:
  serverId: template
  listenPort: 8000
  tickRate: 20
  worldSize: ${version === "2.0.0" ? 8192 : 4096}
  featureFlags:
    hotfixReward: false
`,
    ],
    [
      "settings.json",
      JSON.stringify(
        {
          shard: { id: "template" },
          network: { publicHost: "logic-template.example.internal" },
          tuning: { maxPlayers, tickRate: 20 },
          versionLearning: {
            version,
            note: "server.yaml replacements 会按 basename 合并 JSON/YAML/properties 配置。",
          },
        },
        null,
        2
      ) + "\n",
    ],
    [
      "bin/game-server.sh",
      `#!/bin/bash
echo "Starting game server v${version}..."
DIR="$(cd "$(dirname "$0")/.." && pwd)"
exec java -Xmx2G -cp "$DIR/lib/*" com.neko233.game.Main
`,
    ],
    [
      "scripts/restart.sh",
      `#!/bin/bash
set -euo pipefail
DIR="$(cd "$(dirname "$0")/.." && pwd)"
PID_FILE="$DIR/run/server.pid"
echo "[restart] stopping server \${SERVER_ID}..."
[ -f "$PID_FILE" ] && kill "$(cat "$PID_FILE")" 2>/dev/null || true
sleep 2
echo "[restart] starting server \${SERVER_ID}..."
mkdir -p "$DIR/run" "$DIR/logs"
nohup "$DIR/bin/game-server.sh" > "$DIR/logs/\${SERVER_ID}.log" 2>&1 &
echo $! > "$PID_FILE"
echo "[restart] PID=$!"
`,
    ],
  ];
}

async function uploadDemoFile(projectID, version, name, content) {
  const fd = new FormData();
  fd.append("file", new Blob([content], { type: "text/plain" }), name);
  fd.append("path", name);
  const headers = {};
  const t = getToken();
  if (t) headers.Authorization = `Bearer ${t}`;
  const r = await fetch(`/api/projects/${projectID}/versions/${encodeURIComponent(version)}/files`, {
    method: "POST",
    credentials: "include",
    headers,
    body: fd,
  });
  if (!r.ok) throw new Error(await readErrorMessage(r));
}

async function ensureDemoProject({ select = true } = {}) {
  await api("/api/server-yaml", {
    method: "PUT",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ content: demoServerYaml() }),
  });
  await loadServerIDs();
  await loadProjects();

  let proj = state.projects.find((x) => x.name === DEMO_PROJECT);
  if (!proj) {
    proj = await api("/api/projects", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ name: DEMO_PROJECT }),
    });
  }

  let versions = (await api(`/api/projects/${proj.id}/versions`)) || [];
  for (const version of ["1.0.0", "1.1.0", "2.0.0"]) {
    let row = versions.find((v) => v.version === version);
    if (!row) {
      row = await api(`/api/projects/${proj.id}/versions`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ name: version }),
      });
      versions = (await api(`/api/projects/${proj.id}/versions`)) || [];
    }
    if (row.status !== "published") {
      for (const [name, content] of demoFiles(version)) {
        await uploadDemoFile(proj.id, version, name, content);
      }
    }
    if (version !== "2.0.0" && row.status !== "published") {
      await api(`/api/projects/${proj.id}/versions/${encodeURIComponent(version)}/publish`, { method: "POST" });
      versions = (await api(`/api/projects/${proj.id}/versions`)) || [];
    }
  }

  await loadProjects();
  const found = state.projects.find((x) => x.name === DEMO_PROJECT) || proj;
  if (select) {
    await selectProject(found);
    const fresh = (await api(`/api/projects/${found.id}/versions`)) || [];
    const draft = fresh.find((v) => v.version === "2.0.0") || fresh[0];
    if (draft) await selectVersion(draft);
  }
  return found;
}

async function prepareOnboardingWorkspace() {
  if (canWriteProject()) {
    await ensureDemoProject({ select: true });
  } else {
    await loadProjects();
    if (!state.projectId && state.projects[0]) await selectProject(state.projects[0]);
    if (!state.version && state.versions[0]) await selectVersion(state.versions[0]);
  }
  setGlobalView("workspace");
  setProjectTab("versions");
}

async function prepareOnboardingPreview() {
  setGlobalView("workspace");
  setProjectTab("preview");
  const input = document.getElementById("verPreviewServerId");
  if (input) input.value = DEMO_SERVER_ID;
  const deploySid = document.getElementById("deployServerId");
  if (deploySid && !deploySid.value.trim()) deploySid.value = DEMO_SERVER_ID;
  if (state.projectName && state.version) {
    const d = await fetchDeployPreview(state.projectName, state.version, DEMO_SERVER_ID);
    renderPreviewReport(d, document.getElementById("verPreviewTable"));
  }
}

function driverFactory() {
  return window.driver?.js?.driver || window.driver?.driver;
}

async function startOnboarding({ force = true } = {}) {
  if (!force && hasSeenOnboarding()) return;
  const makeDriver = driverFactory();
  if (!makeDriver) {
    showToast("新手引导组件加载失败，请稍后重试。", "error");
    return;
  }
  let preparing = false;
  const tour = makeDriver({
    popoverClass: "express-tour",
    showProgress: true,
    progressText: "{{current}} / {{total}}",
    nextBtnText: "下一步",
    prevBtnText: "上一步",
    doneBtnText: "完成",
    showButtons: ["next", "previous", "close"],
    allowClose: true,
    onCloseClick: (_, __, { driver }) => {
      markOnboardingSeen("skipped");
      driver.destroy();
    },
    onDestroyStarted: (_, __, { driver }) => {
      markOnboardingSeen("skipped");
      driver.destroy();
    },
    onDestroyed: () => markOnboardingSeen("closed"),
    steps: [
      {
        element: () => document.querySelector("#emptyProject:not(.hidden)") || document.querySelector(".main"),
        popover: {
          title: "从演示项目开始",
          description:
            "我会准备一个自带 1.0.0、1.1.0、2.0.0 的 demo-game，演示按 server_id 替换 JSON、YAML、properties 配置。右上角关闭即可跳过，系统会记住。",
          side: "bottom",
          align: "center",
          onNextClick: async (_, __, { driver }) => {
            if (preparing) return;
            preparing = true;
            await prepareOnboardingWorkspace();
            preparing = false;
            driver.moveNext();
          },
        },
      },
      {
        element: "#projectList",
        popover: {
          title: "项目列表",
          description: "demo-game 是给新手学习的安全项目。真实工作时，一个项目通常对应一组游戏逻辑服版本包。",
          side: "right",
          align: "start",
        },
      },
      {
        element: "#versionList",
        popover: {
          title: "多版本学习",
          description: "版本列表支持搜索。已发布版本用于节点拉取，draft 版本适合上传文件、调整配置并做发布前检查。",
          side: "right",
          align: "start",
        },
      },
      {
        element: "#versionDetail",
        popover: {
          title: "版本包文件",
          description:
            "demo 版本包里有 game.properties、application.yaml、settings.json。替换规则不看目录，只按配置文件 basename 匹配。",
          side: "left",
          align: "start",
          onNextClick: (_, __, { driver }) => {
            setGlobalView("server");
            driver.moveNext();
          },
        },
      },
      {
        element: "#serverYaml",
        popover: {
          title: "server.yaml 替换规则",
          description:
            "每个 server_id 都有 replacements。JSON/YAML 文件可以是嵌套结构，替换时推荐用 dotted key 精确写到叶子字段；properties 按扁平 key 替换。",
          side: "left",
          align: "start",
          onNextClick: async (_, __, { driver }) => {
            await prepareOnboardingPreview();
            driver.moveNext();
          },
        },
      },
      {
        element: "#verPreviewTable",
        popover: {
          title: "键级预览",
          description: "这里能看到每个配置键替换前后的值。发布前先看这里，可以避免把错误 server_id 配到节点上。",
          side: "right",
          align: "start",
        },
      },
      {
        element: "#verPreviewRenderedBody",
        popover: {
          title: "替换后完整配置",
          description: "右侧是替换后的全文。切换上方文件标签，可以查看 JSON、YAML、properties 三类配置最终会变成什么。",
          side: "left",
          align: "start",
          onNextClick: (_, __, { driver }) => {
            setProjectTab("diff");
            driver.moveNext();
          },
        },
      },
      {
        element: "#ptab-diff",
        popover: {
          title: "版本差异",
          description: "版本学习的核心是比较差异：选择 from/to 版本和 server_id，就能看到升级会改变哪些配置键。",
          side: "top",
          align: "start",
          onNextClick: (_, __, { driver }) => {
            setProjectTab("deploy");
            generateDeployScript();
            driver.moveNext();
          },
        },
      },
      {
        element: "#deployCmd",
        popover: {
          title: "部署脚本",
          description: "部署脚本会按安全流程拉取到临时目录，再 stop、swap、start。学完后可以直接从这里复制或下载。",
          side: "left",
          align: "start",
        },
      },
    ],
  });
  tour.drive();
}

document.getElementById("btnDemoProject")?.addEventListener("click", async () => {
  const btn = document.getElementById("btnDemoProject");
  btn.disabled = true;
  btn.textContent = "创建中...";
  try {
    await ensureDemoProject({ select: true });
  } catch (e) {
    showToast("创建演示项目失败: " + e.message, "error");
  } finally {
    btn.disabled = false;
    btn.textContent = "添加演示项目";
  }
});

document.getElementById("btnStartOnboarding")?.addEventListener("click", () => startOnboarding({ force: true }));
document.getElementById("btnEmptyStartOnboarding")?.addEventListener("click", () => startOnboarding({ force: true }));
document.getElementById("btnSettingsStartOnboarding")?.addEventListener("click", () => startOnboarding({ force: true }));
document.getElementById("btnSettingsDemoProject")?.addEventListener("click", () => document.getElementById("btnDemoProject")?.click());

document.getElementById("btnCreateInvite")?.addEventListener("click", async () => {
  if (!state.projectId) return;
  const role = document.getElementById("inviteRole")?.value || "viewer";
  const d = await api(`/api/projects/${state.projectId}/invites`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ role, valid_hours: 168 }),
  });
  const el = document.getElementById("inviteUrl");
  if (el) {
    el.value = d.url;
    navigator.clipboard.writeText(d.url);
    showToast("已复制邀请链接", "ok");
  }
});

let storageSelectedPath = null;

function formatStorageKind(kind) {
  const map = { project: "项目", version: "版本", file: "文件", folder: "目录", tenant: "租户", blob: "Blob", orphan_blob: "孤立 Blob" };
  return map[kind] || kind;
}

async function loadStorageOverview() {
  try {
    const ov = await api("/api/storage/overview");
    renderStorageStats(ov);
    renderStorageBars(ov);
    const tree = await api("/api/storage/tree");
    renderStorageTree(tree);
    storageSelectedPath = null;
    updateStorageDetail(null);
  } catch (e) {
    showToast("加载存储信息失败: " + e.message, "error");
  }
}

function renderStorageStats(ov) {
  const el = document.getElementById("storageStats");
  if (!el) return;
  const avail = ov.available_bytes ? ` · 可用 ${formatBytes(ov.available_bytes)}` : "";
  el.innerHTML = `<div class="storage-stat-row">
    <span>数据目录 <code>${escapeHtml(ov.data_dir || "")}</code></span>
    <span>已用 ${formatBytes(ov.total_bytes)}${avail}</span>
    <span>${ov.project_count || 0} 项目 · ${ov.version_count || 0} 版本</span>
    <span>索引 ${ov.index_entry_count || 0} 条${ov.index_updated_at ? " · " + escapeHtml(ov.index_updated_at) : ""}</span>
    <span>Blob ${ov.blob_stats?.blob_count || 0} 个 · 去重 ${formatBytes(ov.blob_stats?.total_bytes || 0)}</span>
  </div>`;
}

function renderStorageBars(ov) {
  const el = document.getElementById("storageBars");
  if (!el) return;
  const cats = ov.categories || [];
  const total = cats.reduce((s, c) => s + (c.bytes || 0), 0) || ov.total_bytes || 1;
  el.innerHTML = cats.map((c) => {
    const pct = Math.max(2, Math.round((c.bytes / total) * 100));
    return `<div class="storage-bar-row">
      <div class="storage-bar-label"><span>${escapeHtml(c.label || c.name)}</span><span class="text-muted">${formatBytes(c.bytes)}</span></div>
      <div class="storage-bar-track"><div class="storage-bar-fill" style="width:${pct}%"></div></div>
    </div>`;
  }).join("");
}

function renderStorageTree(node, container) {
  const root = container || document.getElementById("storageTree");
  if (!root || !node) return;
  const renderNode = (n, depth = 0) => {
    const hasKids = n.children && n.children.length;
    const isCollapsed = hasKids && collapsedFileTreeFolders.storage.has(n.path);
    const kids = hasKids && !isCollapsed ? `<div class="storage-tree-children">${n.children.map((c) => renderNode(c, depth + 1)).join("")}</div>` : "";
    const meta = n.meta?.status ? ` <span class="badge badge-${n.meta.status === "published" ? "ok" : "draft"}">${escapeHtml(n.meta.status)}</span>` : "";
    return `<button type="button" class="storage-tree-row${storageSelectedPath === n.path ? " active" : ""}" data-path="${escapeAttr(n.path)}" ${hasKids ? `data-storage-folder="${escapeAttr(n.path)}" aria-expanded="${isCollapsed ? "false" : "true"}"` : ""} style="padding-left:${8 + depth * 12}px">
      <span class="tree-caret" aria-hidden="true">${hasKids ? (isCollapsed ? "▸" : "▾") : ""}</span>
      <span class="storage-tree-kind">${escapeHtml(formatStorageKind(n.kind))}</span>
      <span class="storage-tree-name">${escapeHtml(n.name)}</span>${meta}
      <span class="storage-tree-size text-muted">${formatBytes(n.size_bytes)}</span>
    </button>${kids}`;
  };
  root.innerHTML = renderNode(node);
  root.querySelectorAll(".storage-tree-row").forEach((btn) => {
    btn.onclick = () => {
      if (btn.dataset.storageFolder) {
        const p = btn.dataset.storageFolder;
        if (collapsedFileTreeFolders.storage.has(p)) collapsedFileTreeFolders.storage.delete(p);
        else collapsedFileTreeFolders.storage.add(p);
        renderStorageTree(node, container);
      }
      selectStoragePath(btn.dataset.path);
    };
  });
}

async function selectStoragePath(path) {
  storageSelectedPath = path;
  document.getElementById("storageSearchHits")?.classList.add("hidden");
  document.querySelectorAll(".storage-tree-row").forEach((b) => b.classList.toggle("active", b.dataset.path === path));
  try {
    const plan = await api("/api/storage/delete-plan?path=" + encodeURIComponent(path));
    updateStorageDetail(plan);
  } catch (e) {
    updateStorageDetail({ path, deny_reason: e.message, allowed: false });
  }
}

function updateStorageDetail(plan) {
  const el = document.getElementById("storageDetail");
  const delBtn = document.getElementById("btnStorageDelete");
  if (!el) return;
  if (!plan) {
    el.textContent = "选择左侧节点或搜索命中项";
    delBtn?.classList.add("hidden");
    return;
  }
  const warnings = (plan.warnings || []).map((w) => `<li>${escapeHtml(w)}</li>`).join("");
  const related = (plan.related || []).map((r) => `<li>${escapeHtml(r)}</li>`).join("");
  el.innerHTML = `<dl class="storage-detail-dl">
    <dt>路径</dt><dd><code>${escapeHtml(plan.path || "")}</code></dd>
    <dt>类型</dt><dd>${escapeHtml(formatStorageKind(plan.kind || ""))}</dd>
    <dt>大小</dt><dd>${formatBytes(plan.size_bytes || 0)}</dd>
    ${plan.deny_reason ? `<dt>不可删</dt><dd class="err">${escapeHtml(plan.deny_reason)}</dd>` : ""}
    ${warnings ? `<dt>提示</dt><dd><ul>${warnings}</ul></dd>` : ""}
    ${related ? `<dt>关联</dt><dd><ul>${related}</ul></dd>` : ""}
  </dl>`;
  if (plan.allowed) delBtn?.classList.remove("hidden");
  else delBtn?.classList.add("hidden");
}

async function runStorageSearch() {
  const q = document.getElementById("storageSearch")?.value.trim();
  const hitsEl = document.getElementById("storageSearchHits");
  if (!q) {
    hitsEl?.classList.add("hidden");
    return;
  }
  const data = await api("/api/storage/search?q=" + encodeURIComponent(q));
  const hits = data.hits || [];
  if (!hitsEl) return;
  hitsEl.classList.remove("hidden");
  hitsEl.innerHTML = hits.length
    ? hits.map((h) => `<button type="button" class="storage-hit-row" data-path="${escapeAttr(h.path)}">
        <span>${escapeHtml(h.name)}</span>
        <span class="text-muted">${escapeHtml(h.project_name || "")} ${escapeHtml(h.version || "")}</span>
        <span class="text-muted">${formatBytes(h.size_bytes)}</span>
      </button>`).join("")
    : `<p class="hint">无匹配结果，可尝试重建索引</p>`;
  hitsEl.querySelectorAll(".storage-hit-row").forEach((btn) => {
    btn.onclick = () => selectStoragePath(btn.dataset.path);
  });
}

document.getElementById("btnStorageReindex")?.addEventListener("click", async () => {
  try {
    const d = await api("/api/storage/reindex", { method: "POST" });
    showToast(`索引已重建（${d.entries} 条）`, "ok");
    await loadStorageOverview();
  } catch (e) {
    showToast("重建索引失败: " + e.message, "error");
  }
});

document.getElementById("storageSearch")?.addEventListener("input", () => {
  clearTimeout(window._storageSearchTimer);
  window._storageSearchTimer = setTimeout(() => runStorageSearch().catch((e) => showToast(e.message, "error")), 300);
});

document.getElementById("btnStorageDelete")?.addEventListener("click", async () => {
  if (!storageSelectedPath) return;
  const plan = await api("/api/storage/delete-plan?path=" + encodeURIComponent(storageSelectedPath));
  const ok = await showConfirm({
    title: "确认删除",
    message: `将删除 ${storageSelectedPath}（${formatBytes(plan.size_bytes || 0)}）。此操作不可撤销。`,
    confirmText: "删除",
    danger: true,
  });
  if (!ok) return;
  try {
    await api("/api/storage/items", {
      method: "DELETE",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ path: storageSelectedPath }),
    });
    showToast("已删除", "ok");
    await loadStorageOverview();
  } catch (e) {
    showToast("删除失败: " + e.message, "error");
  }
});

init();
