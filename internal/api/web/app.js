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
  fileRows: [],
  version: null,
  versionStatus: null,
  isAdmin: false,
  isRoot: false,
  role: "viewer",
  tenantSlug: null,
  projectRole: null,
  pendingInviteToken: null,
  globalView: "workspace",
  projectTab: "versions",
};

let fileTree = null;
let fileTreeModulePromise = null;
let filePreviewRequestID = 0;

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
  document.getElementById("login").classList.remove("hidden");
  document.getElementById("app").classList.add("hidden");
}

function showApp(username) {
  try {
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
  setGlobalView("workspace");
  loadProjects();
  loadServerYaml();
  loadServerIDs();
  parseInviteHash();
  scheduleOnboarding();
  } catch (showAppErr) {
    document.title = "showApp_ERR:" + String(showAppErr);
    throw showAppErr;
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
  const inProject = view === "workspace" && state.projectId;
  document.getElementById("projectWorkspace").classList.toggle("hidden", !inProject);
  document.getElementById("emptyProject").classList.toggle("hidden", inProject || view !== "workspace");
  if (view === "settings" && state.isAdmin) {
    loadUsers();
    loadAuditLogs();
    if (state.isRoot) loadSystemUpdateStatus();
  }
  if (view === "storage") loadStorageOverview();
}

function setProjectTab(tab) {
  state.projectTab = tab;
  document.querySelectorAll(".project-tab").forEach((b) => {
    b.classList.toggle("active", b.dataset.ptab === tab);
  });
  ["versions", "preview", "team", "deploy", "diff"].forEach((t) => {
    const el = document.getElementById("ptab-" + t);
    if (el) el.classList.toggle("hidden", t !== tab);
  });
  if (tab === "deploy") generateDeployScript();
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
    console.error("init failed:", initErr);
    document.title = "ERR:" + String(initErr);
    if (saved) setToken(null);
    showLogin();
  }
  await parseInviteHash();
}

async function loadServerIDs() {
  try {
    const d = await api("/api/server-ids");
    const dl = document.getElementById("serverIdList");
    if (dl) dl.innerHTML = (d.server_ids || []).map((id) => `<option value="${escapeHtml(id)}">`).join("");
  } catch (_) {}
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
    if (btn.dataset.global === "workspace") setGlobalView("workspace");
    else if (btn.dataset.global === "server") setGlobalView("server");
    else if (btn.dataset.global === "storage") setGlobalView("storage");
    else if (btn.dataset.global === "settings") setGlobalView("settings");
  };
});

document.querySelectorAll(".project-tab").forEach((btn) => {
  btn.onclick = () => setProjectTab(btn.dataset.ptab);
});

document.querySelectorAll("#settingsTabs .seg-tab").forEach((btn) => {
  btn.onclick = () => {
    document.querySelectorAll("#settingsTabs .seg-tab").forEach((b) => b.classList.remove("active"));
    btn.classList.add("active");
    document.getElementById("stab-users").classList.toggle("hidden", btn.dataset.stab !== "users");
    document.getElementById("stab-audit").classList.toggle("hidden", btn.dataset.stab !== "audit");
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

function renderProjectList() {
  const ul = document.getElementById("projectList");
  if (!ul) return;
  ul.innerHTML = "";
  const q = state.projectFilter;
  state.projects
    .filter((p) => !q || p.name.toLowerCase().includes(q))
    .forEach((p) => {
      const li = document.createElement("li");
      li.textContent = p.name;
      li.onclick = () => selectProject(p);
      if (p.id === state.projectId) li.classList.add("selected");
      ul.appendChild(li);
    });
}

async function loadProjects() {
  state.projects = (await api("/api/projects")) || [];
  renderProjectList();
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

async function selectProject(p) {
  state.projectId = p.id;
  state.projectName = p.name;
  state.projectRole = p.my_role || (state.isAdmin ? "admin" : "viewer");
  state.version = null;
  document.getElementById("curProject").textContent = p.name;
  document.getElementById("btnDelProject")?.classList.toggle("hidden", !canWriteProject());
  updateProjectWriteUI();
  loadProjectTeam();
  setGlobalView("workspace");
  setProjectTab(state.projectTab || "versions");
  await loadProjects();
  state.versions = (await api(`/api/projects/${p.id}/versions`)) || [];
  renderVersionList();
  // Populate diff dropdowns
  populateDiffDropdowns(state.versions);
  // Generate deploy script when switching to deploy tab
  generateDeployScript();
  document.getElementById("versionDetail").classList.add("hidden");
  return state.versions;
}

function renderVersionList() {
  const ul = document.getElementById("versionList");
  if (!ul) return;
  ul.innerHTML = "";
  const q = state.versionFilter;
  (state.versions || [])
    .filter((v) => !q || v.version.toLowerCase().includes(q) || v.status.toLowerCase().includes(q))
    .forEach((v) => {
    const li = document.createElement("li");
    li.textContent = `${v.version} · ${v.status}`;
    li.onclick = () => selectVersion(v);
    if (v.version === state.version) li.classList.add("selected");
    ul.appendChild(li);
  });
}

function populateDiffDropdowns(versions) {
  const fromSel = document.getElementById("diffFromVer");
  const toSel = document.getElementById("diffToVer");
  if (!fromSel || !toSel) return;
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
  document.getElementById("versionDetail").classList.remove("hidden");
  setVersionStatusBadge(v.status);
  document.getElementById("verCreated").textContent = v.created_at;
  document.getElementById("verPublished").textContent = v.published_at || "—";
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
  fl.innerHTML = buildFileTreeRows(state.fileRows, {
    canEdit,
    rowAttr: (row, index) => `data-preview-index="${index}"`,
    tags: (row) => row.tags || ["*"],
    actions: (row, index) =>
      canEdit
        ? `<button type="button" class="file-tag-action" data-action="edit-tags" data-index="${index}">编辑</button>
           <button type="button" class="file-tag-action" data-action="clear-tags" data-index="${index}">清空</button>`
        : "",
  });
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
  return [...folderRows, ...fileRows]
    .sort((a, b) => a.path.localeCompare(b.path))
    .map((item) => {
      const depth = Math.max(0, item.path.split("/").length - 1);
      if (item.type === "folder") {
        return `<div class="file-row tree-folder" style="--depth:${depth}">
          <span class="file-path">▾ ${escapeHtml(item.path.split("/").pop())}</span>
        </div>`;
      }
      const tags = (opts.tags ? opts.tags(item.row, item.index) : ["*"]).map(
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

function parseTagsInput(value) {
  return String(value || "")
    .split(/[\s,;]+/)
    .map((x) => x.trim().toLowerCase())
    .filter(Boolean);
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
    fileTree.expandAll();
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
  return `${(n / 1024 / 1024).toFixed(1)} MB`;
}

let previewDebounceTimer = null;
let renderedPreviewIndex = 0;

document.getElementById("verPreviewServerId")?.addEventListener("input", () => {
  const previewSid = document.getElementById("verPreviewServerId")?.value.trim() || "";
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
  const selected = Math.min(container.__diffIndex || 0, Math.max(0, list.length - 1));
  container.__diffIndex = selected;
  const beforeId = opts.preservePreviewIds ? "verPreviewOriginalBody" : "";
  const afterId = opts.preservePreviewIds ? "verPreviewRenderedBody" : "";
  const testAttr = opts.preservePreviewIds ? 'data-testid="preview-rendered-body"' : "";
  container.classList.add("diff-workspace");
  container.innerHTML = `${opts.summary ? `<div class="diff-summary">${opts.summary}</div>` : ""}
    <aside class="diff-tree-panel">
      <div class="panel-label">文件树</div>
      ${list.length ? buildDiffTree(list, selected) : `<div class="empty-diff">${escapeHtml(opts.empty || "无差异")}</div>`}
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
  if (!list.length) {
    const msg = opts.empty || "无差异";
    renderCodeLines(container.querySelector(".diff-pane:first-child .diff-code code"), msg);
    renderCodeLines(container.querySelector(".diff-pane:last-child .diff-code code"), msg);
    return;
  }
  const current = list[selected];
  const [left, right] = buildSideBySideDiff(current.from || "", current.to || "");
  renderCodeLines(container.querySelector(".diff-pane:first-child .diff-code code"), left);
  renderCodeLines(container.querySelector(".diff-pane:last-child .diff-code code"), right);
}

function buildDiffTree(files, selected) {
  const folders = new Set();
  files.forEach((f) => {
    const parts = String(f.path).split("/");
    for (let i = 1; i < parts.length; i += 1) folders.add(parts.slice(0, i).join("/"));
  });
  const rows = [
    ...[...folders].map((path) => ({ path, type: "folder" })),
    ...files.map((f, index) => ({ ...f, index, type: "file" })),
  ].sort((a, b) => a.path.localeCompare(b.path));
  return `<div class="diff-tree">${rows.map((item) => {
    const depth = Math.max(0, String(item.path).split("/").length - 1);
    if (item.type === "folder") {
      return `<div class="diff-tree-row folder" style="padding-left:${0.45 + depth * 1.1}rem">
        <span class="diff-tree-name">▾ ${escapeHtml(item.path.split("/").pop())}</span>
      </div>`;
    }
    return `<button type="button" class="diff-tree-row ${item.index === selected ? "active" : ""}" style="padding-left:${0.45 + depth * 1.1}rem" data-diff-index="${item.index}">
      <span class="diff-tree-name">${escapeHtml(item.path.split("/").pop())}</span>
      <span class="diff-tree-badges"><span class="file-tag">${escapeHtml(item.change || "modified")}</span></span>
    </button>`;
  }).join("")}</div>`;
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
  } catch (e) {
    showToast(e.message, "error");
  }
};

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
  await api("/api/projects", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ name }),
  });
  document.getElementById("newProject").value = "";
  loadProjects();
};

document.getElementById("btnAddVersion").onclick = async () => {
  const name = document.getElementById("newVersion").value.trim();
  if (!name || !state.projectId) return;
  await api(`/api/projects/${state.projectId}/versions`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ name }),
  });
  selectProject({ id: state.projectId, name: state.projectName });
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
        message: `${row.path}（逗号分隔，空=*）`,
        value: (row.tags || ["*"]).join(","),
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
  try {
    await uploadFiles(e.target.files);
  } catch (err) {
    showToast(err.message, "error");
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
    tr.innerHTML = `<td>${u.id}</td><td>${escapeHtml(u.username)}</td><td><code>${escapeHtml(u.token)}</code></td>
      <td><button data-id="${u.id}" class="btn btn-secondary btn-sm refresh">刷新 Token</button>
      <button data-id="${u.id}" class="btn btn-danger btn-sm del">删除</button></td>`;
    tbody.appendChild(tr);
  });
  tbody.querySelectorAll(".refresh").forEach((b) => {
    b.onclick = async () => {
      await api(`/api/users/${b.dataset.id}/refresh-token`, { method: "POST" });
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
  await api("/api/users", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({
      username: document.getElementById("newUser").value,
      password: document.getElementById("newUserPass").value,
      role: document.getElementById("newUserRole").value,
      is_admin: document.getElementById("newUserAdmin").checked,
    }),
  });
  loadUsers();
};

async function loadAuditLogs() {
  if (!state.isAdmin) return;
  const rows = await api("/api/audit-logs");
  const tbody = document.querySelector("#auditTable tbody");
  tbody.innerHTML = rows
    .map(
      (r) =>
        `<tr><td>${escapeHtml(r.at)}</td><td>${escapeHtml(r.username)}</td><td>${escapeHtml(r.action)}</td><td class="audit-detail" title="${escapeHtml(r.detail)}">${escapeHtml(r.detail)}</td><td>${escapeHtml(r.ip || "")}</td></tr>`
    )
    .join("");
}

document.getElementById("btnReloadAudit")?.addEventListener("click", loadAuditLogs);

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
    const kids = hasKids ? `<div class="storage-tree-children">${n.children.map((c) => renderNode(c, depth + 1)).join("")}</div>` : "";
    const meta = n.meta?.status ? ` <span class="badge badge-${n.meta.status === "published" ? "ok" : "draft"}">${escapeHtml(n.meta.status)}</span>` : "";
    return `<button type="button" class="storage-tree-row${storageSelectedPath === n.path ? " active" : ""}" data-path="${escapeAttr(n.path)}" style="padding-left:${8 + depth * 12}px">
      <span class="storage-tree-kind">${escapeHtml(formatStorageKind(n.kind))}</span>
      <span class="storage-tree-name">${escapeHtml(n.name)}</span>${meta}
      <span class="storage-tree-size text-muted">${formatBytes(n.size_bytes)}</span>
    </button>${kids}`;
  };
  root.innerHTML = renderNode(node);
  root.querySelectorAll(".storage-tree-row").forEach((btn) => {
    btn.onclick = () => selectStoragePath(btn.dataset.path);
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
