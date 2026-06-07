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
  version: null,
  versionStatus: null,
  isAdmin: false,
  role: "viewer",
  tenantSlug: null,
  projectRole: null,
  pendingInviteToken: null,
  globalView: "workspace",
  projectTab: "versions",
};

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
  document.getElementById("globalSettings").classList.toggle("hidden", view !== "settings");
  const inProject = view === "workspace" && state.projectId;
  document.getElementById("projectWorkspace").classList.toggle("hidden", !inProject);
  document.getElementById("emptyProject").classList.toggle("hidden", inProject || view !== "workspace");
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
  const files = (await api(`/api/projects/${state.projectId}/versions/${v.version}/files`)) || [];
  const fl = document.getElementById("fileList");
  fl.innerHTML = files.map((f) => `<li>${escapeHtml(f)}</li>`).join("");
  try {
    const cfg = await api(`/api/projects/${state.projectId}/versions/${v.version}/config-files`);
    const dup = Object.entries(cfg.duplicates || {}).filter(([, n]) => n > 1);
    if (dup.length) {
      fl.innerHTML += `<li class="warn">重复: ${dup.map(([b]) => b).join(", ")}</li>`;
    }
  } catch (_) {}
  document.getElementById("verPreviewTable").innerHTML = "";
  document.getElementById("verPreviewRenderedTabs").innerHTML = "";
  document.getElementById("verPreviewRenderedBody").textContent = "选择 server_id";
  updateDeployCmd();
  tryAutoPreviewOnVersionSelect();
  return files;
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
  let html = `<p class="hint"><strong>${escapeHtml(report.project)}</strong> / ${escapeHtml(report.version)} / <code>${escapeHtml(report.server_id)}</code></p>`;
  (report.warnings || []).forEach((w) => {
    html += `<p class="warn">⚠ ${escapeHtml(w)}</p>`;
  });
  if (!report.files?.length) {
    html += "<p>无 replacements 或未匹配配置文件</p>";
  } else {
    html += `<table class="preview-table"><thead><tr><th>文件</th><th>路径</th><th>键</th><th>前</th><th>后</th><th></th></tr></thead><tbody>`;
    for (const f of report.files) {
      const path = (f.paths && f.paths[0]) || "—";
      for (const c of f.changes || []) {
        const cls =
          c.action === "add" ? "action-add" : c.action === "unchanged" ? "action-unchanged" : "action-replace";
        html += `<tr><td>${escapeHtml(f.basename)}</td><td><code>${escapeHtml(path)}</code></td><td>${escapeHtml(c.key)}</td><td>${escapeHtml(c.before || "")}</td><td>${escapeHtml(c.after || "")}</td><td class="${cls}">${c.action}</td></tr>`;
      }
    }
    html += "</tbody></table>";
  }
  if (report.post_hook) html += `<p class="hint">post_hook: <code>${escapeHtml(report.post_hook)}</code></p>`;
  if (report.post_hook_plan?.length) {
    html += `<p class="hint">计划: ${report.post_hook_plan.map((x) => `<code>${escapeHtml(x)}</code>`).join(" ")}</p>`;
  }
  container.innerHTML = html;
  renderRenderedFiles(report.rendered_files || []);
}

function renderRenderedFiles(files) {
  const tabs = document.getElementById("verPreviewRenderedTabs");
  const body = document.getElementById("verPreviewRenderedBody");
  if (!tabs || !body) return;
  if (!files.length) {
    tabs.innerHTML = "";
    body.textContent = "无 replacements 或未匹配配置文件";
    return;
  }
  if (renderedPreviewIndex >= files.length) renderedPreviewIndex = 0;
  tabs.innerHTML = files
    .map(
      (f, i) =>
        `<button type="button" class="${i === renderedPreviewIndex ? "active" : ""}" data-idx="${i}">${escapeHtml(f.basename)}</button>`
    )
    .join("");
  tabs.querySelectorAll("button").forEach((btn) => {
    btn.onclick = () => {
      renderedPreviewIndex = Number(btn.dataset.idx);
      renderRenderedFiles(files);
    };
  });
  const cur = files[renderedPreviewIndex];
  body.textContent = `# ${cur.path}\n\n${cur.after}`;
}

function escapeHtml(s) {
  return String(s).replace(/&/g, "&amp;").replace(/</g, "&lt;").replace(/>/g, "&gt;");
}

async function fetchDeployPreview(project, version, serverId) {
  const q = new URLSearchParams({ project, version, server_id: serverId });
  return api("/api/deploy/preview?" + q);
}

document.getElementById("btnVerPreview").onclick = async () => {
  const sid = document.getElementById("verPreviewServerId").value.trim();
  if (!state.projectName || !state.version || !sid) {
    alert("请选择版本并填写 server_id");
    return;
  }
  try {
    const d = await fetchDeployPreview(state.projectName, state.version, sid);
    renderPreviewReport(d, document.getElementById("verPreviewTable"));
  } catch (e) {
    alert(e.message);
  }
};

function tryAutoPreviewOnVersionSelect() {
  const sid = document.getElementById("verPreviewServerId")?.value.trim();
  if (sid) scheduleDeployPreview();
}

document.getElementById("btnDelProject").onclick = async () => {
  if (!state.projectId || !confirm("删除项目及所有版本？")) return;
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
    alert(e.message);
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
  if (!state.projectId || !state.version || !confirm("驳回？")) return;
  await api(`/api/projects/${state.projectId}/versions/${encodeURIComponent(state.version)}/reject`, { method: "POST" });
  selectProject({ id: state.projectId, name: state.projectName });
});

document.getElementById("btnPublish").onclick = async () => {
  if (!confirm("发布后不可修改。确认？")) return;
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
    alert("选择 from/to 版本并填写 server_id");
    return;
  }
  if (from === to) {
    alert("from 和 to 不能相同");
    return;
  }
  try {
    const q = new URLSearchParams({ project: state.projectName, from, to, server_id: sid });
    const d = await api("/api/deploy/diff?" + q);
    renderVersionDiff(d, document.getElementById("versionDiffOut"));
  } catch (e) {
    alert(e.message);
  }
});

function renderVersionDiff(report, container) {
  if (!container) return;
  let html = `<p>${escapeHtml(report.from_version)} → ${escapeHtml(report.to_version)}</p>`;
  if (!report.files?.length) {
    container.innerHTML = html + "<p>无差异</p>";
    return;
  }
  for (const f of report.files) {
    html += `<h4>${escapeHtml(f.basename)}</h4><table class="preview-table"><thead><tr><th>键</th><th></th><th>from</th><th>to</th></tr></thead><tbody>`;
    for (const k of f.keys || []) {
      html += `<tr><td>${escapeHtml(k.key)}</td><td>${escapeHtml(k.change)}</td><td>${escapeHtml(k.from || "")}</td><td>${escapeHtml(k.to || "")}</td></tr>`;
    }
    html += "</tbody></table>";
  }
  container.innerHTML = html;
}

document.getElementById("btnDeleteVersion").onclick = async () => {
  if (!confirm("删除版本 " + state.version + "？")) return;
  if (!confirm("再次确认删除")) return;
  await api(`/api/projects/${state.projectId}/versions/${state.version}?confirm=yes`, { method: "DELETE" });
  selectProject({ id: state.projectId, name: state.projectName });
};

async function uploadFiles(files) {
  for (const file of files) {
    const fd = new FormData();
    fd.append("file", file);
    fd.append("path", file.name);
    const headers = {};
    const t = getToken();
    if (t) headers.Authorization = `Bearer ${t}`;
    const r = await fetch(`/api/projects/${state.projectId}/versions/${state.version}/files`, {
      method: "POST",
      credentials: "include",
      headers,
      body: fd,
    });
    if (!r.ok) throw new Error(await readErrorMessage(r));
  }
  selectVersion({ version: state.version, status: state.versionStatus, created_at: "", published_at: "" });
}

document.getElementById("fileInput").onchange = async (e) => {
  try {
    await uploadFiles(e.target.files);
  } catch (err) {
    alert(err.message);
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
      alert(err.message);
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
  alert("已保存");
  loadServerIDs();
};

document.getElementById("btnPreview").onclick = async () => {
  const project = document.getElementById("previewProject").value.trim();
  const version = document.getElementById("previewVersion").value.trim();
  const serverId = document.getElementById("previewServerId").value.trim();
  if (!project || !version || !serverId) {
    alert("填写 project / version / server_id");
    return;
  }
  const d = await fetchDeployPreview(project, version, serverId);
  renderPreviewReport(d, document.getElementById("previewOut"));
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
      if (!confirm("删除用户？")) return;
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

document.getElementById("btnChangeMyPass")?.addEventListener("click", async () => {
  await api("/api/me/password", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({
      old_password: document.getElementById("myOldPass").value,
      new_password: document.getElementById("myNewPass").value,
    }),
  });
  alert("密码已更新");
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
    alert("邀请无效: " + e.message);
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
    alert(e.message);
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
    alert("新手引导组件加载失败，请稍后重试。");
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
    alert("创建演示项目失败: " + e.message);
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
    alert("已复制邀请链接");
  }
});

document.title = "BEFORE_INIT";
init().then(() => { document.title = "AFTER_INIT:" + (document.getElementById('login')?.classList.contains('hidden') ? 'OK' : 'LOGIN'); }).catch(e => { document.title = "INIT_CATCH:" + String(e); });
