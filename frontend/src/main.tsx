import { FormEvent, useEffect, useMemo, useState } from "react";
import { createRoot } from "react-dom/client";
import {
  api,
  Project,
  PushDeployment,
  PushDeploymentTask,
  LoginAttack,
  values,
  Version,
} from "./api";
import { Dashboard } from "./Dashboard";
import { AgentPanel } from "./AgentPanel";
import { HookTab } from "./HookTab";
import { ClusterTab } from "./ClusterTab";
import { GuidePanel } from "./GuidePanel";
import "../../internal/api/web/style.css";

type Tab =
  | "versions"
  | "cluster"
  | "push"
  | "hooks"
  | "logs"
  | "preview"
  | "team"
  | "deploy"
  | "diff";
type GlobalView = "workspace" | "dashboard" | "guide" | "agent" | "security";
const projectTabs: readonly [Tab, string][] = [
  ["versions", "版本管理"],
  ["cluster", "集群节点"],
  ["push", "发布任务"],
  ["hooks", "自动 Hook"],
  ["logs", "发布日志"],
  ["preview", "拉取预览"],
  ["team", "团队"],
  ["deploy", "部署"],
  ["diff", "差异"],
];

function App() {
  const [me, setMe] = useState<{ username: string; is_admin: boolean } | null>(
    null,
  );
  const [projects, setProjects] = useState<Project[]>([]);
  const [project, setProject] = useState<Project | null>(null);
  const [versions, setVersions] = useState<Version[]>([]);
  const [tab, setTab] = useState<Tab>("versions");
  const [message, setMessage] = useState("");
  const [view, setView] = useState<GlobalView>("workspace");
  const loadProjects = async () => {
    const list = (await api<Project[] | null>("/projects")) ?? [];
    setProjects(list);
    setProject(
      (now) => list.find((item) => item.id === now?.id) ?? list[0] ?? null,
    );
  };
  const loadVersions = async () => {
    if (project)
      setVersions(
        (await api<Version[] | null>(`/projects/${project.id}/versions`)) ?? [],
      );
  };
  useEffect(() => {
    api<{ username: string; is_admin: boolean }>("/me")
      .then((user) => {
        setMe(user);
        return loadProjects();
      })
      .catch(() => setMe(null));
  }, []);
  useEffect(() => {
    void loadVersions();
  }, [project?.id]);
  useEffect(() => {
    document.title = me ? "express233 · 发布控制台" : "express233 · 登录";
  }, [me]);
  if (!me)
    return (
      <Login
        onLogin={async (user) => {
          setMe(user);
          await loadProjects();
        }}
      />
    );
  const selectProject = (next: Project) => {
    setProject(next);
    setView("workspace");
  };
  return (
    <div className="app-shell" data-testid="app-shell">
      <Sidebar
        projects={projects}
        selected={project}
        onSelect={selectProject}
        onCreated={loadProjects}
        username={me.username}
        isAdmin={me.is_admin}
        view={view}
        onView={setView}
      />
      <main className="main">
        {view === "dashboard" ? (
          <Dashboard projects={projects} />
        ) : view === "guide" ? (
          <GuidePanel />
        ) : view === "agent" ? (
          <AgentPanel />
        ) : view === "security" ? (
          <LoginSecurity />
        ) : (
          <>
            <header className="workspace-header">
              <div className="workspace-title">
                <h1 data-testid="cur-project">{project?.name ?? "选择项目"}</h1>
              </div>
              {project && (
                <nav className="project-tabs" role="tablist">
                  {projectTabs.map(([id, label]) => (
                    <button
                      type="button"
                      key={id}
                      className={`project-tab ${tab === id ? "active" : ""}`}
                      onClick={() => setTab(id)}
                    >
                      {label}
                    </button>
                  ))}
                </nav>
              )}
            </header>
            {message && <div className="toast ok">{message}</div>}
            {project ? (
              <Workspace
                project={project}
                versions={versions}
                tab={tab}
                onMessage={setMessage}
                reloadVersions={loadVersions}
              />
            ) : (
              <div className="empty-state">
                <h2>选择项目</h2>
                <p>在左侧选择或创建项目，管理版本包与 server_id 配置替换。</p>
              </div>
            )}
          </>
        )}
      </main>
    </div>
  );
}

function Login({
  onLogin,
}: {
  onLogin: (user: { username: string; is_admin: boolean }) => Promise<void>;
}) {
  const [error, setError] = useState("");
  async function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    const data = new FormData(event.currentTarget);
    try {
      const user = await api<{
        username: string;
        is_admin: boolean;
        token?: string;
      }>("/login", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({
          username: data.get("username"),
          password: data.get("password"),
        }),
      });
      if (user.token) localStorage.setItem("express233_jwt", user.token);
      await onLogin(user);
    } catch (err) {
      setError((err as Error).message);
    }
  }
  return (
    <div className="login-screen" data-testid="login-panel">
      <form className="login-card" onSubmit={submit}>
        <div className="login-brand">
          <h1>express233</h1>
          <p className="login-sub">游戏服务器集群 · 推拉一体交付控制台</p>
        </div>
        <label className="field">
          <span>用户名</span>
          <input
            className="input"
            name="username"
            defaultValue="root"
            autoComplete="username"
          />
        </label>
        <label className="field">
          <span>密码</span>
          <input
            className="input"
            name="password"
            type="password"
            autoComplete="current-password"
          />
        </label>
        <button
          className="btn btn-primary btn-block"
          data-testid="login-submit"
        >
          登录
        </button>
        {error && <p className="err">{error}</p>}
      </form>
    </div>
  );
}

function Sidebar({
  projects,
  selected,
  onSelect,
  onCreated,
  username,
  isAdmin,
  view,
  onView,
}: {
  projects: Project[];
  selected: Project | null;
  onSelect: (project: Project) => void;
  onCreated: () => Promise<void>;
  username: string;
  isAdmin: boolean;
  view: GlobalView;
  onView: (view: GlobalView) => void;
}) {
  const [name, setName] = useState("");
  async function create() {
    if (!name.trim()) return;
    await api("/projects", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ name }),
    });
    setName("");
    await onCreated();
  }
  return (
    <aside className="sidebar">
      <div className="sidebar-brand-row">
        <span className="brand-name">express233</span>
      </div>
      <div className="sidebar-section">
        <div className="section-label">项目</div>
        <div className="new-project-row">
          <input
            className="input input-sm"
            value={name}
            onChange={(event) => setName(event.target.value)}
            placeholder="新项目名称"
            data-testid="new-project-input"
          />
          <button
            className="btn btn-primary btn-sm"
            onClick={create}
            data-testid="add-project"
            aria-label="新建项目"
          >
            +
          </button>
        </div>
      </div>
      <ul className="project-list" data-testid="project-list">
        {projects.map((item) => (
          <li
            key={item.id}
            className={`project-item ${item.id === selected?.id && view === "workspace" ? "selected" : ""}`}
            onClick={() => onSelect(item)}
            title={item.name}
          >
            <svg
              className="project-item-icon"
              aria-hidden="true"
              viewBox="0 0 24 24"
            >
              <path d="M3 7a2 2 0 0 1 2-2h5l2 2h7a2 2 0 0 1 2 2v8a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2V7Z" />
            </svg>
            <span className="project-item-name">{item.name}</span>
          </li>
        ))}
      </ul>
      <nav className="sidebar-nav">
        <div className="section-label">导航</div>
        <button
          type="button"
          className={`sidebar-nav-item ${view === "workspace" ? "active" : ""}`}
          onClick={() => onView("workspace")}
        >
          <svg className="nav-svg" aria-hidden="true" viewBox="0 0 24 24">
            <rect x="3" y="3" width="7" height="7" rx="1" />
            <rect x="14" y="3" width="7" height="7" rx="1" />
            <rect x="3" y="14" width="7" height="7" rx="1" />
            <rect x="14" y="14" width="7" height="7" rx="1" />
          </svg>
          <span>工作区</span>
        </button>
        <button
          type="button"
          className={`sidebar-nav-item ${view === "dashboard" ? "active" : ""}`}
          onClick={() => onView("dashboard")}
          data-testid="nav-dashboard"
        >
          <svg className="nav-svg" aria-hidden="true" viewBox="0 0 24 24">
            <path d="M4 19V9M10 19V5M16 19v-7M22 19V2" />
            <path d="M2 19h22" />
          </svg>
          <span>数据大盘</span>
        </button>
        <button
          type="button"
          className={`sidebar-nav-item ${view === "guide" ? "active" : ""}`}
          onClick={() => onView("guide")}
          data-testid="nav-guide"
        >
          <svg className="nav-svg" aria-hidden="true" viewBox="0 0 24 24">
            <path d="M4 19.5v-15A2.5 2.5 0 0 1 6.5 2H20v20H6.5a2.5 2.5 0 0 1 0-5H20" />
            <path d="M8 7h8M8 11h8M8 15h5" />
          </svg>
          <span>文档目录</span>
        </button>
        {isAdmin ? (
          <button
            type="button"
            className={`sidebar-nav-item ${view === "agent" ? "active" : ""}`}
            onClick={() => onView("agent")}
            data-testid="nav-agent"
          >
            <svg className="nav-svg" aria-hidden="true" viewBox="0 0 24 24">
              <rect x="3" y="11" width="18" height="10" rx="2" />
              <circle cx="12" cy="5" r="2" />
              <path d="M12 7v4M8 15h.01M16 15h.01M8 18h8" />
            </svg>
            <span>Agent 与节点</span>
          </button>
        ) : null}
        {isAdmin ? (
          <button type="button" className={`sidebar-nav-item ${view === "security" ? "active" : ""}`} onClick={() => onView("security")} data-testid="nav-login-security">
            <svg className="nav-svg" aria-hidden="true" viewBox="0 0 24 24"><path d="M12 3 4 6v5c0 5.1 3.4 9.8 8 11 4.6-1.2 8-5.9 8-11V6l-8-3Z"/><path d="M9 12l2 2 4-4"/></svg><span>登录安全</span>
          </button>
        ) : null}
      </nav>
      <div className="sidebar-footer">
        <span className="user-label" data-testid="whoami">
          {username}
        </span>
      </div>
    </aside>
  );
}

function LoginSecurity() {
  const [attacks, setAttacks] = useState<LoginAttack[]>([]);
  const [error, setError] = useState("");
  const load = async () => {
    try { setAttacks(await api<LoginAttack[]>("/security/login-ip-bans")); setError(""); }
    catch (reason) { setError((reason as Error).message); }
  };
  useEffect(() => { void load(); }, []);
  const clear = async (ip: string) => { await api(`/security/login-ip-bans/${encodeURIComponent(ip)}`, { method: "DELETE" }); await load(); };
  return <section className="global-panel login-security-panel" data-testid="login-security-panel">
    <header className="page-header"><div><h1>登录安全</h1><p className="subtitle">近 30 天攻击来源。每个 IP 仅保留累计次数与最后 3 次失败时间。</p></div><button className="btn btn-secondary btn-sm" onClick={() => void load()}>刷新</button></header>
    {error ? <p className="err">{error}</p> : null}
    <div className="card"><div className="table-wrap"><table className="data-table login-attack-table"><thead><tr><th>攻击来源</th><th>目标账号</th><th>累计失败</th><th>封禁</th><th>最后 3 次尝试</th><th>操作</th></tr></thead><tbody>{attacks.length ? attacks.map((attack) => <tr key={attack.ip}><td><code>{attack.ip}</code></td><td>{attack.username || "未知"}</td><td><strong>{attack.attempt_count}</strong><small>当前窗口 {attack.failures}</small></td><td>{attack.banned_until ? <span className="badge badge-warn">至 {attack.banned_until}</span> : <span className="badge badge-ok">未封禁</span>}</td><td>{attack.last_attempt_times.map((time) => <small key={time}>{time}</small>)}</td><td><button className="btn btn-danger btn-sm" onClick={() => void clear(attack.ip)}>清除</button></td></tr>) : <tr><td colSpan={6} className="table-empty">暂无失败尝试</td></tr>}</tbody></table></div></div>
  </section>;
}

function Workspace({
  project,
  versions,
  tab,
  onMessage,
  reloadVersions,
}: {
  project: Project;
  versions: Version[];
  tab: Tab;
  onMessage: (value: string) => void;
  reloadVersions: () => Promise<void>;
}) {
  if (tab === "cluster")
    return (
      <ClusterTab project={project} versions={versions} onMessage={onMessage} />
    );
  if (tab === "push")
    return (
      <PushTab project={project} versions={versions} onMessage={onMessage} />
    );
  if (tab === "hooks")
    return <HookTab project={project} onMessage={onMessage} />;
  if (tab === "logs") return <LogsTab project={project} />;
  if (tab !== "versions")
    return (
      <section className="ptab-panel">
        <div className="card">
          <h3 className="card-title">
            {projectTabs.find(([id]) => id === tab)?.[1]}
          </h3>
          <p className="hint">迁移中：该模块将按现有 HTML 交互逐项接管。</p>
        </div>
      </section>
    );
  return (
    <VersionsTab
      project={project}
      versions={versions}
      onMessage={onMessage}
      reload={reloadVersions}
    />
  );
}

function VersionsTab({
  project,
  versions,
  onMessage,
  reload,
}: {
  project: Project;
  versions: Version[];
  onMessage: (value: string) => void;
  reload: () => Promise<void>;
}) {
  const latest = useMemo(
    () => versions.find((version) => version.status === "published"),
    [versions],
  );
  const [name, setName] = useState("");
  async function create() {
    const version = await api<Version>(`/projects/${project.id}/versions`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ name }),
    });
    setName("");
    onMessage(`已创建版本 ${version.version}`);
    await reload();
  }
  return (
    <section className="ptab-panel">
      <div className="split-versions">
        <div className="card version-sidebar">
          <div className="toolbar compact">
            <input
              className="input"
              value={name}
              onChange={(event) => setName(event.target.value)}
              placeholder="1.0.0"
              data-testid="new-version-input"
            />
            <button
              className="btn btn-primary btn-sm"
              onClick={create}
              data-testid="add-version"
            >
              新建
            </button>
          </div>
          <ul className="version-list" data-testid="version-list">
            {versions.map((version) => (
              <li
                className={`version-item ${version.version === latest?.version ? "latest-published" : ""}`}
                key={version.id}
              >
                <span>
                  {version.version}{" "}
                  {version.version === latest?.version && (
                    <span className="latest-version-label">最新</span>
                  )}
                </span>
                <span className="version-status">{version.status}</span>
              </li>
            ))}
          </ul>
        </div>
        <div className="card version-detail" data-testid="version-detail">
          <h3 className="card-title">版本管理</h3>
          <p className="hint">已发布版本是不可变快照；修复请创建新版本，回滚请选择历史已发布版本。</p>
        </div>
      </div>
    </section>
  );
}

function PushTab({
  project,
  versions,
  onMessage,
}: {
  project: Project;
  versions: Version[];
  onMessage: (value: string) => void;
}) {
  const latest = versions.find((item) => item.status === "published");
  const [tasks, setTasks] = useState<PushDeploymentTask[]>([]);
  const [editing, setEditing] = useState<number | null>(null);
  const [name, setName] = useState("");
  const [version, setVersion] = useState("");
  const [serverIDs, setServerIDs] = useState("");
  const [tags, setTags] = useState("test");
  const [match, setMatch] = useState<"all" | "any">("all");
  const load = async () =>
    setTasks(
      (await api<PushDeploymentTask[] | null>(
        `/projects/${project.id}/push-tasks`,
      )) ?? [],
    );
  useEffect(() => {
    void load();
  }, [project.id]);
  function reset() {
    setEditing(null);
    setName("");
    setVersion("");
    setServerIDs("");
    setTags("test");
    setMatch("all");
  }
  async function save() {
    const path = editing
      ? `/projects/${project.id}/push-tasks/${editing}`
      : `/projects/${project.id}/push-tasks`;
    await api(path, {
      method: editing ? "PUT" : "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({
        name,
        version,
        server_ids: values(serverIDs),
        tags: values(tags),
        tag_match: match,
      }),
    });
    onMessage(editing ? "发布任务已更新" : "发布任务已保存");
    reset();
    await load();
  }
  function edit(task: PushDeploymentTask) {
    setEditing(task.id);
    setName(task.name);
    setVersion(task.version ?? "");
    setServerIDs(task.server_ids.join(", "));
    setTags(task.tags.join(", "));
    setMatch(task.tag_match);
  }
  async function run(task: PushDeploymentTask, dryRun: boolean) {
    await api(`/projects/${project.id}/push-tasks/${task.id}/run`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ dry_run: dryRun }),
    });
    onMessage(dryRun ? "预演完成并已留痕" : "发布任务已排队");
    await load();
  }
  async function remove(task: PushDeploymentTask) {
    if (!window.confirm(`删除任务“${task.name}”？历史执行日志仍保留 30 天。`))
      return;
    await api(`/projects/${project.id}/push-tasks/${task.id}`, {
      method: "DELETE",
    });
    if (editing === task.id) reset();
    await load();
  }
  return (
    <section className="ptab-panel">
      <div className="card">
        <div className="card-title-row">
          <div>
            <h3 className="card-title">可重复发布任务</h3>
            <p className="hint">
              保存版本策略与目标筛选；每次运行形成不可变日志快照。
            </p>
          </div>
          <span className="badge badge-ok">
            最新已发布版本：{latest?.version ?? "—"}
          </span>
        </div>
        <div className="release-task-editor" data-testid="release-task-editor">
          <input
            className="input"
            value={name}
            maxLength={100}
            onChange={(event) => setName(event.target.value)}
            placeholder="任务名称"
          />
          <select
            className="input"
            value={version}
            onChange={(event) => setVersion(event.target.value)}
          >
            <option value="">每次取最新已发布版本</option>
            {versions
              .filter((item) => item.status === "published")
              .map((item) => (
                <option value={item.version} key={item.id}>
                  {item.version}
                </option>
              ))}
          </select>
          <input
            className="input"
            value={serverIDs}
            onChange={(event) => setServerIDs(event.target.value)}
            placeholder="server_id，逗号分隔"
          />
          <input
            className="input"
            value={tags}
            onChange={(event) => setTags(event.target.value)}
            placeholder="标签，默认 test"
          />
          <select
            className="input"
            value={match}
            onChange={(event) => setMatch(event.target.value as "all" | "any")}
          >
            <option value="all">标签同时满足</option>
            <option value="any">任一标签匹配</option>
          </select>
          <button
            className="btn btn-primary btn-sm"
            onClick={() => void save()}
          >
            {editing ? "更新任务" : "保存任务"}
          </button>
          {editing ? (
            <button className="btn btn-ghost btn-sm" onClick={reset}>
              取消
            </button>
          ) : null}
        </div>
        <div className="table-wrap" data-testid="release-task-list">
          {tasks.length ? (
            <table className="data-table release-task-table">
              <thead>
                <tr>
                  <th>任务</th>
                  <th>版本策略</th>
                  <th>目标筛选</th>
                  <th>运行</th>
                  <th>最近</th>
                  <th>操作</th>
                </tr>
              </thead>
              <tbody>
                {tasks.map((task) => (
                  <tr key={task.id}>
                    <td>
                      <strong>{task.name}</strong>
                      <small>#{task.id}</small>
                    </td>
                    <td>
                      {task.version || (
                        <span className="badge badge-ok">最新已发布</span>
                      )}
                    </td>
                    <td>
                      <small>
                        server_id：{task.server_ids.join(", ") || "全部"}
                      </small>
                      <small>标签：{task.tags.join(", ")}</small>
                    </td>
                    <td>
                      <strong>{task.run_count}</strong>
                      <small>次</small>
                    </td>
                    <td>{task.last_run_at || "尚未运行"}</td>
                    <td className="release-task-actions">
                      <button
                        className="btn btn-secondary btn-sm"
                        onClick={() => void run(task, true)}
                      >
                        预演
                      </button>
                      <button
                        className="btn btn-primary btn-sm"
                        onClick={() => void run(task, false)}
                      >
                        执行
                      </button>
                      <button
                        className="btn btn-ghost btn-sm"
                        onClick={() => edit(task)}
                      >
                        编辑
                      </button>
                      <button
                        className="btn btn-danger btn-sm"
                        onClick={() => void remove(task)}
                      >
                        删除
                      </button>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          ) : (
            <div className="agent-empty">
              还没有发布任务。SSH 资源在全局 Agent 页面维护。
            </div>
          )}
        </div>
      </div>
    </section>
  );
}

function LogsTab({ project }: { project: Project }) {
  const [logs, setLogs] = useState<PushDeployment[]>([]);
  const load = async () =>
    setLogs(
      (await api<PushDeployment[] | null>(
        `/projects/${project.id}/push-deployments`,
      )) ?? [],
    );
  useEffect(() => {
    void load();
    const timer = window.setInterval(() => void load(), 3000);
    return () => window.clearInterval(timer);
  }, [project.id]);
  return (
    <section className="ptab-panel">
      <div className="card">
        <div className="card-title-row">
          <div>
            <h3 className="card-title">发布日志</h3>
            <p className="hint">任务、版本与筛选条件按执行时状态快照留痕。</p>
          </div>
          <div className="toolbar compact">
            <span className="badge badge-draft">不可删除 · 保留 30 天</span>
            <button
              className="btn btn-secondary btn-sm"
              onClick={() => void load()}
            >
              刷新
            </button>
          </div>
        </div>
        <table className="data-table">
          <thead>
            <tr>
              <th>时间</th>
              <th>任务快照</th>
              <th>版本</th>
              <th>筛选</th>
              <th>状态</th>
            </tr>
          </thead>
          <tbody>
            {logs.map((log) => {
              let selector: { server_ids?: string[]; tags?: string[] } = {};
              try {
                selector = JSON.parse(log.selector || "{}");
              } catch {
                /* legacy selector */
              }
              return (
                <tr key={log.id}>
                  <td>{log.created_at}</td>
                  <td>
                    <strong>{log.task_name || "临时发布"}</strong>
                    <small>
                      {log.task_id ? `任务 #${log.task_id}` : `执行 #${log.id}`}
                    </small>
                  </td>
                  <td>
                    <code>{log.version}</code>
                  </td>
                  <td>
                    <small>
                      server_id：{selector.server_ids?.join(", ") || "全部"}
                    </small>
                    <small>标签：{selector.tags?.join(", ") || "test"}</small>
                  </td>
                  <td>{log.status}</td>
                </tr>
              );
            })}
          </tbody>
        </table>
      </div>
    </section>
  );
}

createRoot(document.getElementById("root")!).render(<App />);
