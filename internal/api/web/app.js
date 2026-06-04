const api = (path, opts = {}) =>
  fetch(path, { credentials: "include", ...opts }).then(async (r) => {
    if (!r.ok) {
      const j = await r.json().catch(() => ({}));
      throw new Error(j.error || r.statusText);
    }
    if (r.status === 204) return null;
    return r.json();
  });

let state = {
  projects: [],
  projectId: null,
  projectName: null,
  version: null,
  versionStatus: null,
  isAdmin: false,
  role: "viewer",
  tenantSlug: null,
  projectRole: null,
  pendingInviteToken: null,
};

async function init() {
  try {
    const me = await api("/api/me");
    state.isAdmin = me.is_admin;
    state.role = me.role || (me.is_admin ? "admin" : "viewer");
    state.tenantSlug = me.tenant_slug || null;
    showApp(me.username);
  } catch {
    document.getElementById("login").classList.remove("hidden");
  }
  await parseInviteHash();
}

function showApp(username) {
  document.getElementById("login").classList.add("hidden");
  document.getElementById("app").classList.remove("hidden");
  const who = document.getElementById("who");
  who.textContent = state.tenantSlug
    ? `${username} @ ${state.tenantSlug}`
    : username;
  if (state.isAdmin) {
    document.querySelectorAll(".admin-only").forEach((el) => el.classList.remove("hidden"));
    loadUsers();
    loadAuditLogs();
  }
  if (state.isAdmin || state.role === "operator") {
    document.querySelectorAll(".operator-only").forEach((el) => el.classList.remove("hidden"));
  }
  loadProjects();
  loadServerYaml();
  loadServerIDs();
  parseInviteHash();
}

async function loadServerIDs() {
  try {
    const d = await api("/api/server-ids");
    const dl = document.getElementById("serverIdList");
    dl.innerHTML = (d.server_ids || []).map((id) => `<option value="${escapeHtml(id)}">`).join("");
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
    state.isAdmin = me.is_admin;
    state.role = me.role || (me.is_admin ? "admin" : "viewer");
    state.tenantSlug = me.tenant_slug || null;
    showApp(me.username);
  } catch (e) {
    document.getElementById("loginErr").textContent = e.message;
  }
};

document.getElementById("btnLogout").onclick = async () => {
  await api("/api/logout", { method: "POST" });
  location.reload();
};

document.querySelectorAll("nav button").forEach((btn) => {
  btn.onclick = () => {
    document.querySelectorAll("nav button").forEach((b) => b.classList.remove("active"));
    btn.classList.add("active");
    ["projects", "server", "users", "audit"].forEach((t) => {
      document.getElementById("tab-" + t).classList.toggle("hidden", t !== btn.dataset.tab);
    });
  };
});

async function loadProjects() {
  state.projects = await api("/api/projects");
  const ul = document.getElementById("projectList");
  ul.innerHTML = "";
  state.projects.forEach((p) => {
    const li = document.createElement("li");
    li.textContent = p.name;
    li.onclick = () => selectProject(p);
    if (p.id === state.projectId) li.classList.add("selected");
    ul.appendChild(li);
  });
}

function canWriteProject() {
  return state.projectRole === "admin" || state.isAdmin;
}

function updateProjectWriteUI() {
  const w = canWriteProject();
  document.querySelectorAll(".project-write").forEach((el) => {
    el.classList.toggle("hidden", !w);
    if (el.tagName === "INPUT" || el.tagName === "BUTTON") {
      el.disabled = !w;
    }
  });
  const fileInput = document.getElementById("fileInput");
  if (fileInput) fileInput.disabled = !w;
}

async function loadProjectTeam() {
  const box = document.getElementById("projectTeam");
  if (!box || !state.projectId) return;
  box.classList.remove("hidden");
  try {
    const members = await api(`/api/projects/${state.projectId}/members`);
    const ul = document.getElementById("memberList");
    ul.innerHTML = members
      .map(
        (m) =>
          `<li>${escapeHtml(m.username)} <code>${escapeHtml(m.role)}</code>${
            canWriteProject() && m.role !== "admin"
              ? ""
              : ""
          }</li>`
      )
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
  await loadProjects();
  const versions = await api(`/api/projects/${p.id}/versions`);
  const ul = document.getElementById("versionList");
  ul.innerHTML = "";
  versions.forEach((v) => {
    const li = document.createElement("li");
    li.textContent = `${v.version} [${v.status}]`;
    li.onclick = () => selectVersion(v);
    ul.appendChild(li);
  });
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
  document.getElementById("verStatus").textContent = v.status;
  document.getElementById("verCreated").textContent = v.created_at;
  document.getElementById("verPublished").textContent = v.published_at || "-";
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
  fl.innerHTML = files.map((f) => `<li>${f}</li>`).join("");
  try {
    const cfg = await api(`/api/projects/${state.projectId}/versions/${v.version}/config-files`);
    const dup = Object.entries(cfg.duplicates || {}).filter(([, n]) => n > 1);
    if (dup.length) {
      fl.innerHTML += `<li class="warn">重复配置名: ${dup.map(([b]) => b).join(", ")}</li>`;
    }
    if (cfg.files?.length) {
      fl.innerHTML += `<li class="hint">配置文件: ${cfg.files.map((x) => x.basename).join(", ")}</li>`;
    }
  } catch (_) {}
  document.getElementById("verPreviewTable").innerHTML = "";
  document.getElementById("verPreviewRenderedTabs").innerHTML = "";
  document.getElementById("verPreviewRenderedBody").textContent = "选择 server_id 后显示替换后的配置文件全文";
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
  } catch (_) {
    /* 输入过程中 server_id 可能尚未存在 */
  }
}

function renderPreviewReport(report, container) {
  let html = `<p><strong>${report.project}</strong> / ${report.version} / server_id=<code>${report.server_id}</code></p>`;
  (report.warnings || []).forEach((w) => {
    html += `<p class="warn">⚠ ${w}</p>`;
  });
  if (!report.files || report.files.length === 0) {
    html += "<p>无 replacements 或未匹配配置文件</p>";
  } else {
    html += `<table class="preview-table"><thead><tr><th>配置文件</th><th>路径</th><th>键</th><th>变更前</th><th>变更后</th><th>动作</th></tr></thead><tbody>`;
    for (const f of report.files) {
      const path = (f.paths && f.paths[0]) || "—";
      for (const c of f.changes || []) {
        const cls =
          c.action === "add" ? "action-add" : c.action === "unchanged" ? "action-unchanged" : "action-replace";
        html += `<tr><td>${f.basename}</td><td><code>${path}</code></td><td>${c.key}</td><td>${escapeHtml(c.before || "")}</td><td>${escapeHtml(c.after || "")}</td><td class="${cls}">${c.action}</td></tr>`;
      }
    }
    html += "</tbody></table>";
  }
  if (report.post_hook) {
    html += `<p>post_hook: <code>${report.post_hook}</code></p>`;
  }
  if (report.post_hook_plan?.length) {
    html += `<p>post_hook 计划: ${report.post_hook_plan.map((x) => `<code>${escapeHtml(x)}</code>`).join(" ")}</p>`;
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
// 版本选中后若已填 server_id 则自动预览
function tryAutoPreviewOnVersionSelect() {
  const sid = document.getElementById("verPreviewServerId")?.value.trim();
  if (sid) scheduleDeployPreview();
}

document.getElementById("btnDelProject").onclick = async () => {
  if (!state.projectId || !confirm("删除项目及所有版本？")) return;
  await api(`/api/projects/${state.projectId}`, { method: "DELETE" });
  state.projectId = null;
  document.getElementById("versionDetail").classList.add("hidden");
  loadProjects();
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
  try {
    await api(`/api/projects/${state.projectId}/versions/${encodeURIComponent(state.version)}/submit-review`, {
      method: "POST",
    });
    selectProject({ id: state.projectId, name: state.projectName });
  } catch (e) {
    alert(e.message);
  }
});

document.getElementById("btnRejectReview")?.addEventListener("click", async () => {
  if (!state.projectId || !state.version || !confirm("驳回该版本审批？")) return;
  await api(`/api/projects/${state.projectId}/versions/${encodeURIComponent(state.version)}/reject`, {
    method: "POST",
  });
  selectProject({ id: state.projectId, name: state.projectName });
});

document.getElementById("btnPublish").onclick = async () => {
  if (!confirm("发布后不可修改，仅可整版本删除。确认发布？")) return;
  await api(`/api/projects/${state.projectId}/versions/${state.version}/publish`, { method: "POST" });
  selectProject({ id: state.projectId, name: state.projectName });
};

document.getElementById("btnVersionDiff")?.addEventListener("click", async () => {
  const from = document.getElementById("diffFromVer")?.value.trim();
  const sid = document.getElementById("verPreviewServerId")?.value.trim();
  if (!state.projectName || !state.version || !from || !sid) {
    alert("填写对比版本 from 与 server_id");
    return;
  }
  try {
    const q = new URLSearchParams({
      project: state.projectName,
      from,
      to: state.version,
      server_id: sid,
    });
    const d = await api("/api/deploy/diff?" + q);
    renderVersionDiff(d, document.getElementById("versionDiffOut"));
  } catch (e) {
    alert(e.message);
  }
});

function renderVersionDiff(report, container) {
  if (!container) return;
  let html = `<p>${escapeHtml(report.from_version)} → ${escapeHtml(report.to_version)} / server_id=<code>${escapeHtml(report.server_id)}</code></p>`;
  if (!report.files?.length) {
    html += "<p>无差异</p>";
    container.innerHTML = html;
    return;
  }
  for (const f of report.files) {
    html += `<h5>${escapeHtml(f.basename)}</h5><table class="preview-table"><thead><tr><th>键</th><th>变更</th><th>from</th><th>to</th></tr></thead><tbody>`;
    for (const k of f.keys || []) {
      html += `<tr><td>${escapeHtml(k.key)}</td><td>${escapeHtml(k.change)}</td><td>${escapeHtml(k.from || "")}</td><td>${escapeHtml(k.to || "")}</td></tr>`;
    }
    html += "</tbody></table>";
  }
  container.innerHTML = html;
}

document.getElementById("btnDeleteVersion").onclick = async () => {
  if (!confirm("确定删除整个版本？此操作不可恢复。")) return;
  if (!confirm("再次确认：删除版本 " + state.version)) return;
  await api(`/api/projects/${state.projectId}/versions/${state.version}?confirm=yes`, { method: "DELETE" });
  document.getElementById("versionDetail").classList.add("hidden");
  selectProject({ id: state.projectId, name: state.projectName });
};

async function uploadFiles(files) {
  for (const file of files) {
    const fd = new FormData();
    fd.append("file", file);
    fd.append("path", file.name);
    const r = await fetch(`/api/projects/${state.projectId}/versions/${state.version}/files`, {
      method: "POST",
      credentials: "include",
      body: fd,
    });
    if (!r.ok) throw new Error(await r.text());
  }
  selectVersion({ version: state.version });
}

document.getElementById("fileInput").onchange = async (e) => {
  try {
    await uploadFiles(e.target.files);
  } catch (err) {
    alert(err.message);
  }
};

const dropZone = document.getElementById("versionDetail");
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
  try {
    const d = await fetchDeployPreview(project, version, serverId);
    renderPreviewReport(d, document.getElementById("previewOut"));
  } catch (e) {
    alert(e.message);
  }
};

async function loadUsers() {
  const users = await api("/api/users");
  const tbody = document.querySelector("#userTable tbody");
  tbody.innerHTML = "";
  users.forEach((u) => {
    const tr = document.createElement("tr");
    tr.innerHTML = `<td>${u.id}</td><td>${u.username}</td><td><code>${u.token}</code></td>
      <td><button data-id="${u.id}" class="refresh">刷新 Token</button>
      <button data-id="${u.id}" class="del">删除</button></td>`;
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
  try {
    const rows = await api("/api/audit-logs");
    const tbody = document.querySelector("#auditTable tbody");
    tbody.innerHTML = rows
      .map(
        (r) =>
          `<tr><td>${r.at}</td><td>${escapeHtml(r.username)}</td><td>${escapeHtml(r.action)}</td><td>${escapeHtml(r.detail)}</td><td>${escapeHtml(r.ip || "")}</td></tr>`
      )
      .join("");
  } catch (e) {
    console.warn(e);
  }
}

document.getElementById("btnReloadAudit")?.addEventListener("click", loadAuditLogs);

document.getElementById("btnChangeMyPass")?.addEventListener("click", async () => {
  try {
    await api("/api/me/password", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({
        old_password: document.getElementById("myOldPass").value,
        new_password: document.getElementById("myNewPass").value,
      }),
    });
    alert("密码已更新");
  } catch (e) {
    alert(e.message);
  }
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
    text.textContent = `邀请加入项目「${info.project_name}」，角色：${
      info.role === "admin" ? "项目管理员（读写）" : "只读成员"
    }${info.expired ? "（已过期）" : ""}`;
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
    alert("已加入项目: " + p.name);
    await loadProjects();
  } catch (e) {
    if (e.message.includes("login") || e.message.includes("401")) {
      alert("请先登录后再接受邀请");
      document.getElementById("login").classList.remove("hidden");
    } else {
      alert(e.message);
    }
  }
});

document.getElementById("btnCreateInvite")?.addEventListener("click", async () => {
  if (!state.projectId) return;
  const role = document.getElementById("inviteRole")?.value || "viewer";
  try {
    const d = await api(`/api/projects/${state.projectId}/invites`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ role, valid_hours: 168 }),
    });
    const el = document.getElementById("inviteUrl");
    if (el) {
      el.value = d.url;
      navigator.clipboard.writeText(d.url);
      alert("邀请链接已复制到剪贴板");
    }
  } catch (e) {
    alert(e.message);
  }
});

init();
