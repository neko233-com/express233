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
      const j = await r.json().catch(() => ({}));
      throw new Error(j.error || r.statusText);
    }
    if (r.status === 204) return null;
    return r.json();
  });
};

let state = {
  projects: [],
  projectFilter: "",
  projectId: null,
  projectName: null,
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
  } catch {
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
  const el = document.getElementById("deployCmd");
  if (!el || !state.projectName || !state.version) return;
  const sid = document.getElementById("verPreviewServerId")?.value.trim() || "<server_id>";
  const central = window.location.origin;
  el.textContent =
    `export EXPRESS233_SERVER=${central}\n` +
    `export EXPRESS233_TOKEN=<your_token>\n` +
    `express233 deploy --project ${state.projectName} --version ${state.version} \\\n` +
    `  --server-id ${sid} --dest /opt/game/${sid}`;
}

document.getElementById("btnCopyDeploy")?.addEventListener("click", () => {
  const t = document.getElementById("deployCmd")?.textContent;
  if (t) navigator.clipboard.writeText(t);
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
  state.projects = await api("/api/projects");
  renderProjectList();
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
  const versions = await api(`/api/projects/${p.id}/versions`);
  const ul = document.getElementById("versionList");
  ul.innerHTML = "";
  versions.forEach((v) => {
    const li = document.createElement("li");
    li.textContent = `${v.version} · ${v.status}`;
    li.onclick = () => selectVersion(v);
    ul.appendChild(li);
  });
  document.getElementById("versionDetail").classList.add("hidden");
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
  const files = await api(`/api/projects/${state.projectId}/versions/${v.version}/files`);
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
}

let previewDebounceTimer = null;
let renderedPreviewIndex = 0;

document.getElementById("verPreviewServerId")?.addEventListener("input", () => {
  updateDeployCmd();
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
  const versions = await api(`/api/projects/${state.projectId}/versions`);
  const published = versions.find((v) => v.version === ver);
  if (published) await selectVersion(published);
};

document.getElementById("btnVersionDiff")?.addEventListener("click", async () => {
  const from = document.getElementById("diffFromVer")?.value.trim();
  const sid = document.getElementById("verPreviewServerId")?.value.trim();
  if (!state.projectName || !state.version || !from || !sid) {
    alert("填写 from 与 server_id");
    return;
  }
  const q = new URLSearchParams({ project: state.projectName, from, to: state.version, server_id: sid });
  const d = await api("/api/deploy/diff?" + q);
  renderVersionDiff(d, document.getElementById("versionDiffOut"));
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
    if (!r.ok) throw new Error(await r.text());
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
        `<tr><td>${escapeHtml(r.at)}</td><td>${escapeHtml(r.username)}</td><td>${escapeHtml(r.action)}</td><td>${escapeHtml(r.detail)}</td><td>${escapeHtml(r.ip || "")}</td></tr>`
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

init();
