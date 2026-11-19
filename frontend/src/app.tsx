import { createContext, useContext, useEffect, useRef, useState, type FormEvent } from "react";
import { Link, NavLink, Outlet, useLocation, useMatch, useNavigate } from "react-router";
import {
  ArrowLeft,
  BookOpen,
  Box,
  ChartNoAxesColumnIncreasing,
  Database,
  FileCode2,
  Folder,
  LogOut,
  Plus,
  Search,
  Server,
  Settings2,
} from "lucide-react";
import {
  ApiError,
  errorMessage,
  request,
  saveToken,
  uploadFile,
  useResource,
  watchSessionExpiry,
  type Session,
} from "./api";
import { type Project, type Version, formText, versionPath } from "./types";
import {
  Button,
  Empty,
  FeedbackProvider,
  Field,
  PageHeader,
  ResourceNotice,
  useAction,
  useFeedback,
  Wordmark,
} from "./ui";

export function ConsoleApp() {
  return (
    <FeedbackProvider>
      <AuthGate />
    </FeedbackProvider>
  );
}

function AuthGate() {
  const [session, setSession] = useState<Session | null>();
  const [error, setError] = useState("");
  const [attempt, retry] = useState(0);
  useEffect(() => watchSessionExpiry(() => setSession(null)), []);
  useEffect(() => {
    const controller = new AbortController();
    request<Session>("/api/me", { signal: controller.signal })
      .then((me) => {
        if (controller.signal.aborted) return;
        saveToken(me.token);
        setSession(me);
      })
      .catch((failure: unknown) => {
        if (controller.signal.aborted) return;
        if (!(failure instanceof ApiError && failure.status === 401))
          setError(errorMessage(failure));
        setSession(null);
      });
    return () => controller.abort();
  }, [attempt]);

  if (session === undefined)
    return (
      <main className="center-screen">
        <ResourceNotice loading />
      </main>
    );
  if (!session)
    return (
      <Login
        error={error}
        retry={() => {
          setError("");
          setSession(undefined);
          retry(attempt + 1);
        }}
        onLogin={(me) => {
          saveToken(me.token);
          setSession(me);
          setError("");
        }}
      />
    );
  return (
    <Shell
      key={session.tenant_id + ":" + session.username}
      session={session}
      onLogout={() => {
        saveToken();
        setSession(null);
      }}
    />
  );
}

function Login({
  onLogin,
  error,
  retry,
}: {
  onLogin: (me: Session) => void;
  error: string;
  retry: () => void;
}) {
  const [loginError, setLoginError] = useState("");
  const { busy, run } = useAction();
  function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    const data = new FormData(event.currentTarget);
    const username = formText(data, "username");
    const password = String(data.get("password") ?? "");
    if (!username || !password) {
      setLoginError("请输入账号和密码");
      return;
    }
    setLoginError("");
    void run(async () => {
      try {
        onLogin(
          await request<Session>("/api/login", {
            method: "POST",
            body: { username, password },
            auth: false,
          }),
        );
      } catch (failure) {
        setLoginError(errorMessage(failure));
      }
    });
  }
  return (
    <main className="login-screen" data-testid="login-panel">
      <div className="login-card">
        <Wordmark />
        <h1>登录控制台</h1>
        <form onSubmit={submit} className="form-stack">
          <Field label="账号">
            <input className="input" name="username" id="user" autoComplete="username" autoFocus />
          </Field>
          <Field label="密码">
            <input
              className="input"
              name="password"
              id="pass"
              type="password"
              autoComplete="current-password"
            />
          </Field>
          {loginError && (
            <p id="loginErr" className="error" role="alert">
              {loginError}
            </p>
          )}
          {error && <ResourceNotice error={error} retry={retry} />}
          <Button type="submit" tone="primary" data-testid="login-submit" disabled={busy}>
            {busy ? "正在登录…" : "登录"}
          </Button>
        </form>
        <a className="muted-link" href="/docs/" target="_blank" rel="noreferrer">
          API 文档
        </a>
      </div>
    </main>
  );
}

interface ConsoleState {
  session: Session;
  projects: Project[];
  projectsLoading: boolean;
  updateProject: (value: Project) => void;
  refreshProjects: () => void;
  canCreateProject: boolean;
}
const ConsoleContext = createContext<ConsoleState | null>(null);
const emptyProjects: Project[] = [];
const nav = [
  {
    label: "交付总览",
    path: "/dashboard",
    key: "dashboard",
    icon: ChartNoAxesColumnIncreasing,
    testId: "nav-dashboard",
  },
  {
    label: "机器与目标",
    path: "/agents",
    key: "agent",
    icon: Server,
    testId: "nav-agent",
    admin: true,
  },
  { label: "高级配置", path: "/server", key: "server", icon: FileCode2 },
  { label: "存储空间", path: "/storage", key: "storage", icon: Database, testId: "nav-storage" },
  { label: "系统设置", path: "/settings", key: "settings", icon: Settings2 },
  { label: "文档目录", path: "/guide", key: "guide", icon: BookOpen, testId: "nav-guide" },
];

function Shell({ session, onLogout }: { session: Session; onLogout: () => void }) {
  const projectsResource = useResource<Project[] | null>("/api/projects");
  const [recent, setRecent] = useState<Project[]>([]);
  const [search, setSearch] = useState("");
  const navigate = useNavigate();
  const location = useLocation();
  const matchedProject = useMatch("/projects/:projectId/*");
  const [workspace, setWorkspace] = useState("/");
  const main = useRef<HTMLElement>(null);
  const action = useAction();
  const logout = useAction();
  const { toast } = useFeedback();
  const canCreateProject = session.is_admin || session.role === "operator";

  const fetched = projectsResource.data ?? emptyProjects;
  const projects = [
    ...fetched.map((project) => recent.find((local) => local.id === project.id) ?? project),
    ...recent.filter((project) => !fetched.some((server) => server.id === project.id)),
  ].toSorted((a, b) => a.name.localeCompare(b.name));
  const visibleProjects = projects.filter((project) =>
    project.name.toLowerCase().includes(search.trim().toLowerCase()),
  );

  useEffect(() => {
    main.current?.scrollTo({ top: 0 });
    if (location.pathname.startsWith("/projects/"))
      setWorkspace(location.pathname + location.search);
  }, [location.pathname, location.search]);

  function updateProject(value: Project) {
    setRecent((items) => [...items.filter((item) => item.id !== value.id), value]);
  }

  function refreshProjects() {
    setRecent([]);
    projectsResource.refresh();
  }

  function create(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    const form = event.currentTarget;
    const name = formText(new FormData(form), "name");
    if (!name) return;
    void action.run(async () => {
      const project = await request<Project>("/api/projects", { method: "POST", body: { name } });
      updateProject({ ...project, my_role: "admin" });
      form.reset();
      setSearch("");
      await navigate(`/projects/${project.id}/versions`);
      toast("项目已创建");
    });
  }

  return (
    <ConsoleContext
      value={{
        session,
        projects,
        projectsLoading: projectsResource.loading,
        updateProject,
        refreshProjects,
        canCreateProject,
      }}
    >
      <div id="app" className="app-shell" data-testid="app-shell">
        <aside className="sidebar">
          <Link className="brand-link" to="/">
            <Wordmark />
          </Link>
          <div className="sidebar-projects">
            <div className="section-label">项目</div>
            <div className="input-icon">
              <Search size={15} aria-hidden="true" />
              <input
                className="input"
                type="search"
                id="projectSearch"
                value={search}
                placeholder="搜索项目…"
                aria-label="搜索项目"
                onChange={(event) => setSearch(event.currentTarget.value)}
              />
            </div>
            {canCreateProject && (
              <form className="sidebar-create" onSubmit={create}>
                <input
                  id="newProject"
                  name="name"
                  className="input"
                  placeholder="新项目名称"
                  aria-label="新项目名称"
                  data-testid="new-project-input"
                />
                <Button
                  type="submit"
                  tone="primary"
                  data-testid="add-project"
                  disabled={action.busy}
                  aria-label="新建项目"
                  title="新建项目"
                >
                  <Plus size={17} />
                </Button>
              </form>
            )}
            <ResourceNotice
              error={projectsResource.error}
              loading={projectsResource.loading && !projects.length}
              retry={refreshProjects}
            />
            <ul className="project-list" data-testid="project-list">
              {visibleProjects.map((project) => (
                <li key={project.id}>
                  <NavLink
                    to={`/projects/${project.id}/versions`}
                    className={`project-item ${matchedProject?.params.projectId === String(project.id) ? "selected" : ""}`}
                    title={project.name}
                  >
                    <Box size={16} aria-hidden="true" />
                    <span>{project.name}</span>
                  </NavLink>
                </li>
              ))}
            </ul>
            {!visibleProjects.length && !projectsResource.loading && (
              <div className="sidebar-empty">{search ? "未找到匹配项目" : "暂无项目"}</div>
            )}
          </div>
          <nav className="sidebar-nav" aria-label="控制台">
            <Link
              to={workspace}
              data-global="workspace"
              className={`sidebar-nav-item ${location.pathname === "/" || matchedProject ? "active" : ""}`}
            >
              <Folder size={16} aria-hidden="true" />
              项目工作区
            </Link>
            {nav
              .filter((item) => !item.admin || session.is_admin)
              .map(({ key, path, label, testId, icon: Icon }) => (
                <NavLink
                  key={key}
                  to={path}
                  className="sidebar-nav-item"
                  data-global={key}
                  data-testid={testId}
                >
                  <Icon size={16} aria-hidden="true" />
                  <span>{label}</span>
                </NavLink>
              ))}
            <a className="sidebar-nav-item" href="/docs/" target="_blank" rel="noreferrer">
              <BookOpen size={16} aria-hidden="true" />
              API 文档
            </a>
          </nav>
          <div className="sidebar-footer">
            <span className="user-avatar" aria-hidden="true">
              {session.username.slice(0, 1).toUpperCase()}
            </span>
            <NavLink className="user-name" to="/settings" data-testid="whoami" title="账号设置">
              <strong>{session.username}</strong>
              <small>{session.tenant_slug}</small>
            </NavLink>
            <Button
              tone="ghost"
              small
              disabled={logout.busy}
              aria-label="退出"
              title="退出"
              onClick={() =>
                void logout.run(async () => {
                  await request("/api/logout", { method: "POST", auth: false });
                  onLogout();
                })
              }
            >
              <LogOut size={16} />
            </Button>
          </div>
        </aside>
        <main className="main" ref={main} id="main-content">
          <div className="main-inner">
            {location.pathname !== "/" && (
              <Button
                tone="ghost"
                small
                className="back-button"
                data-testid="navigate-back"
                onClick={() => void (history.state?.idx > 0 ? navigate(-1) : navigate("/"))}
              >
                <ArrowLeft size={15} aria-hidden="true" />
                上一步
              </Button>
            )}
            <InviteBanner />
            <Outlet />
          </div>
        </main>
      </div>
    </ConsoleContext>
  );
}

function InviteBanner() {
  const { refreshProjects } = useConsole();
  const { hash } = useLocation();
  const navigate = useNavigate();
  const token = hash.startsWith("#invite?")
    ? new URLSearchParams(hash.slice(8)).get("token")
    : null;
  const endpoint = token ? "/api/project-invites/" + encodeURIComponent(token) : undefined;
  const invite = useResource<{ project_name: string; role: string; expired: boolean }>(endpoint);
  const { run, busy } = useAction();
  if (!endpoint) return null;
  return (
    <div className="notice">
      <ResourceNotice loading={invite.loading} error={invite.error} retry={invite.refresh} />
      {invite.data && (
        <>
          <span>
            邀请加入「{invite.data.project_name}」 ·{" "}
            {invite.data.role === "admin" ? "管理员" : "只读"}
            {invite.data.expired ? " · 已过期" : ""}
          </span>
          <Button
            small
            tone="primary"
            disabled={busy || invite.data.expired}
            onClick={() =>
              void run(async () => {
                const project = await request<Project>(endpoint + "/accept", { method: "POST" });
                refreshProjects();
                await navigate(`/projects/${project.id}/versions`, { replace: true });
              })
            }
          >
            接受邀请
          </Button>
        </>
      )}
    </div>
  );
}

export function useConsole() {
  const context = useContext(ConsoleContext);
  if (!context) throw new Error("Console context is unavailable");
  return context;
}

export function Home() {
  const { canCreateProject, projects, updateProject } = useConsole();
  const { busy, run } = useAction();
  const navigate = useNavigate();
  const { confirm, toast } = useFeedback();
  return (
    <>
      <PageHeader title="项目工作区" description="管理版本包、分服配置与交付目标。" />
      <div className="home-empty card">
        <Folder size={30} aria-hidden="true" />
        <h2>{projects.length ? "选择一个项目" : "从一个服务开始"}</h2>
        <p>从侧栏选择项目，或创建一个项目并上传版本文件。</p>
        <div className="toolbar">
          {canCreateProject && (
            <Button
              tone="primary"
              disabled={busy}
              onClick={() =>
                void run(async () => {
                  if (
                    !(await confirm({
                      title: "添加演示项目",
                      message: "创建 demo-game 项目和一个包含示例配置的草稿版本。",
                      confirmText: "添加",
                    }))
                  )
                    return;
                  const found = projects.find((project) => project.name === "demo-game");
                  const project =
                    found ??
                    (await request<Project>("/api/projects", {
                      method: "POST",
                      body: { name: "demo-game" },
                    }));
                  if (!found) updateProject({ ...project, my_role: "admin" });
                  const base = `/api/projects/${project.id}/versions`;
                  const versions = await request<Version[] | null>(base);
                  if (!versions?.length) {
                    const draft = await request<Version>(base, {
                      method: "POST",
                      body: { name: "1.0.0" },
                    });
                    await uploadFile(
                      versionPath(project.id, draft.version) + "/files",
                      new File(["server.port=8080\nmax.players=100\n"], "game.properties"),
                      [],
                      () => {},
                    );
                  }
                  await navigate(`/projects/${project.id}/versions`);
                  toast("演示项目已就绪");
                })
              }
            >
              添加演示项目
            </Button>
          )}
          <Link className="btn btn-secondary" to="/guide">
            查看使用指南
          </Link>
        </div>
      </div>
    </>
  );
}

export function NotFound() {
  return (
    <Empty>
      <h1>页面不存在</h1>
      <Link to="/">返回项目工作区</Link>
    </Empty>
  );
}
