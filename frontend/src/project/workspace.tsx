import { Activity, createContext, lazy, Suspense, useContext, useState } from "react";
import { useNavigate, useParams, useSearchParams } from "react-router";
import { Info, Settings2 } from "lucide-react";
import { ApiError, errorMessage, request, uploadFile, useResource } from "../api";
import { NotFound, useConsole } from "../app";
import {
  type Project,
  type Validation,
  type Version,
  type VersionFile,
  versionPath,
} from "../types";
import { Badge, Button, ResourceNotice, useAction, useFeedback } from "../ui";
import { Publish } from "./publish";
import { Versions } from "./versions";

const ServerConfigs = lazy(() =>
  import("./config").then((module) => ({ default: module.ServerConfigs })),
);
const Preview = lazy(() => import("./preview").then((module) => ({ default: module.Preview })));
const Diff = lazy(() => import("./preview").then((module) => ({ default: module.VersionDiff })));
const Delivery = lazy(() => import("./delivery").then((module) => ({ default: module.Delivery })));
const Cluster = lazy(() => import("./cluster").then((module) => ({ default: module.Cluster })));
const Team = lazy(() => import("./team").then((module) => ({ default: module.Team })));
const DeployScript = lazy(() =>
  import("./script").then((module) => ({ default: module.DeployScript })),
);

const panels = [
  { key: "versions", label: "版本包", name: "版本", View: Versions },
  { key: "config", label: "分服配置", View: ServerConfigs },
  { key: "preview", label: "配置预览", name: "拉取预览", View: Preview },
  { key: "publish", label: "发布版本", View: Publish },
  { key: "delivery", label: "部署上线", View: Delivery },
  { key: "diff", label: "差异", View: Diff },
  { key: "cluster", label: "集群节点", View: Cluster },
  { key: "team", label: "团队", View: Team },
  { key: "deploy", label: "部署脚本", View: DeployScript },
] as const;

type Panel = (typeof panels)[number]["key"];
type VersionStatus = { version: string; result: Validation };
type ConfirmedPreview = { version: string; target: string };
interface Workspace {
  project: Project;
  projectPath: string;
  canWrite: boolean;
  versions: Version[];
  version?: Version;
  latest?: Version;
  activePath?: string;
  files: VersionFile[];
  filesLoading: boolean;
  filesError?: string;
  refreshFiles: () => void;
  target: string;
  setTarget: (value: string) => void;
  confirmed: boolean;
  clearPreview: () => void;
  confirmPreview: () => void;
  revision: number;
  invalidateChecks: () => void;
  serverIDs: string[];
  refreshServerIDs: () => void;
  open: (tab: Panel) => void;
  selectVersion: (version: string) => void;
  createVersion: (version: string) => Promise<void>;
  reloadVersions: () => void;
  validation?: Validation;
  validate: () => Promise<void>;
  publish: () => Promise<void>;
  busy: boolean;
  busyKind: string;
  upload: (files: File[], tags: string[]) => Promise<void>;
  progress: string;
}
const WorkspaceContext = createContext<Workspace | null>(null);
const noVersions: Version[] = [];
const noFiles: VersionFile[] = [];
const noServerIDs: string[] = [];

export function ProjectWorkspace() {
  const { projectId, tab = "versions" } = useParams();
  const { projects, projectsLoading } = useConsole();
  const project = projects.find((p) => String(p.id) === projectId);
  const panel = panels.find((entry) => entry.key === tab);
  if (!project && projectsLoading) return <ResourceNotice loading />;
  if (!project || !panel) return <NotFound />;
  return <ProjectScreen key={project.id} project={project} panel={panel} />;
}

function ProjectScreen({ project, panel }: { project: Project; panel: (typeof panels)[number] }) {
  const { session, refreshProjects } = useConsole();
  const canWrite = session.is_admin || project.my_role === "admin";
  const projectPath = `/api/projects/${project.id}`;
  const versionsResource = useResource<Version[] | null>(projectPath + "/versions");
  const serverIDs = useResource<{ server_ids: string[] }>("/api/server-ids");
  const navigate = useNavigate();
  const [search] = useSearchParams();
  const [created, setCreated] = useState<Version>();
  const [target, setServer] = useState("");
  const [confirmation, setConfirmation] = useState<ConfirmedPreview>();
  const [check, setCheck] = useState<VersionStatus>();
  const [revision, setRevision] = useState(0);
  const [busyKind, setBusyKind] = useState("");
  const [progress, setProgress] = useState("");
  const action = useAction();
  const deleting = useAction();
  const { confirm, toast } = useFeedback();

  const fetched = versionsResource.data ?? noVersions;
  const versions =
    created && !fetched.some((row) => row.version === created.version)
      ? [created, ...fetched]
      : fetched;
  const version = versions.find((row) => row.version === search.get("version")) ?? versions[0];
  const latest = versions
    .filter((row) => row.status === "published")
    .toSorted(
      (a, b) => (b.published_at || "").localeCompare(a.published_at || "") || b.id - a.id,
    )[0];
  const activePath = version && versionPath(project.id, version.version);
  const filesResource = useResource<VersionFile[] | null>(activePath && activePath + "/file-tags");
  const files = filesResource.data ?? noFiles;
  const validation = check?.version === version?.version ? check?.result : undefined;
  const confirmed = Boolean(
    confirmation && confirmation.version === version?.version && confirmation.target === target,
  );

  function invalidateChecks() {
    setCheck(undefined);
    setConfirmation(undefined);
    setRevision((value) => value + 1);
  }

  function setTarget(value: string) {
    if (target !== value) setConfirmation(undefined);
    setServer(value);
  }

  function open(tab: Panel) {
    if (tab === panel.key) return;
    void navigate(
      `/projects/${project.id}/${tab}` +
        (version ? "?version=" + encodeURIComponent(version.version) : ""),
    );
  }

  function selectVersion(name: string) {
    if (name === version?.version && search.get("version") === name) return;
    invalidateChecks();
    void navigate(`/projects/${project.id}/${panel.key}?version=${encodeURIComponent(name)}`, {
      replace: true,
    });
  }

  async function createVersion(name: string) {
    const row = await request<Version>(projectPath + "/versions", {
      method: "POST",
      body: { name },
    });
    setCreated(row);
    versionsResource.refresh();
    selectVersion(row.version);
  }

  function reloadVersions() {
    setCreated(undefined);
    versionsResource.refresh();
  }

  async function validate() {
    if (!activePath || !version) return;
    await action.run(async () => {
      setBusyKind("validate");
      setCheck(undefined);
      try {
        const result = await request<Validation>(activePath + "/validate");
        setCheck({ version: version.version, result });
      } catch (error) {
        setCheck({
          version: version.version,
          result: { ok: false, errors: [errorMessage(error)] },
        });
      } finally {
        setBusyKind("");
      }
    });
  }

  async function publish() {
    if (
      !activePath ||
      !version ||
      !canWrite ||
      !["draft", "pending_review"].includes(version.status)
    )
      return;
    await action.run(async () => {
      if (
        !(await confirm({
          title: "正式发布",
          confirmText: "发布",
          message: `项目：${project.name}\n版本：${version.version}\n发布会锁定版本内容，并触发已开启的自动发布规则。`,
        }))
      )
        return;
      setBusyKind("publish");
      try {
        await request(activePath + "/publish", { method: "POST" });
        reloadVersions();
        toast(`版本 ${version.version} 已发布`);
      } finally {
        setBusyKind("");
      }
    });
  }

  async function upload(selection: File[], tags: string[]) {
    if (!activePath || !version || !selection.length || !canWrite || version.status === "published")
      return;
    await action.run(async () => {
      setBusyKind("upload");
      invalidateChecks();
      let completed = 0;
      try {
        for (const file of selection) {
          const label = `${version.version} · ${completed + 1}/${selection.length} · ${file.name}`;
          setProgress(label);
          for (let attempt = 1; ; attempt++) {
            try {
              await uploadFile(activePath + "/files", file, tags, (percent) =>
                setProgress(label + ` · ${percent}%`),
              );
              break;
            } catch (error) {
              if (
                attempt >= 3 ||
                !(error instanceof ApiError) ||
                (error.status !== 0 && error.status < 500)
              )
                throw error;
              await new Promise((resolve) => setTimeout(resolve, 600 * attempt));
            }
          }
          completed++;
        }
        setProgress(`${version.version} · ${completed} 个文件上传完成`);
        toast(`${completed} 个文件上传完成`);
      } catch (error) {
        setProgress(`上传停止，已完成 ${completed}/${selection.length}。${errorMessage(error)}`);
        throw error;
      } finally {
        filesResource.refresh();
        setBusyKind("");
      }
    });
  }

  const context: Workspace = {
    project,
    projectPath,
    canWrite,
    versions,
    version,
    latest,
    activePath,
    files,
    filesLoading: filesResource.loading,
    filesError: filesResource.error,
    refreshFiles: filesResource.refresh,
    target,
    setTarget,
    confirmed,
    clearPreview: () => setConfirmation(undefined),
    confirmPreview: () => {
      if (version && target) setConfirmation({ version: version.version, target });
    },
    revision,
    invalidateChecks,
    serverIDs: serverIDs.data?.server_ids ?? noServerIDs,
    refreshServerIDs: serverIDs.refresh,
    open,
    selectVersion,
    createVersion,
    reloadVersions,
    validation,
    validate,
    publish,
    busy: action.busy,
    busyKind,
    upload,
    progress,
  };

  return (
    <WorkspaceContext value={context}>
      <div id="projectWorkspace">
        <header className="workspace-heading">
          <div>
            <h1 id="curProject" data-testid="cur-project">
              {project.name}
            </h1>
            <div className="workspace-meta">
              <span>项目权限 · {canWrite ? "管理员" : "只读"}</span>
              {latest && (
                <span>
                  最新发布 <code>{latest.version}</code>
                </span>
              )}
            </div>
          </div>
          {canWrite && (
            <details className="project-options">
              <summary title="项目管理">
                <Settings2 size={16} aria-hidden="true" />
                项目管理
              </summary>
              <div>
                <Button
                  tone="danger"
                  small
                  disabled={deleting.busy || action.busy}
                  onClick={() =>
                    void deleting.run(async () => {
                      if (
                        !(await confirm({
                          title: "删除项目",
                          message: `删除 ${project.name} 及其所有版本和文件。`,
                          danger: true,
                          confirmText: "删除",
                        }))
                      )
                        return;
                      await request(projectPath, { method: "DELETE" });
                      await navigate("/");
                      refreshProjects();
                      toast("项目已删除");
                    })
                  }
                >
                  删除项目
                </Button>
              </div>
            </details>
          )}
        </header>
        <nav className="project-tabs" aria-label="项目工作区">
          {panels.map((entry) => (
            <button
              type="button"
              key={entry.key}
              className={`project-tab ${entry.key === panel.key ? "active" : ""}`}
              data-ptab={entry.key}
              aria-label={"name" in entry ? entry.name : entry.label}
              aria-current={entry.key === panel.key ? "page" : undefined}
              onClick={() => open(entry.key)}
            >
              {entry.label}
            </button>
          ))}
        </nav>
        {versionsResource.error && (
          <ResourceNotice error={versionsResource.error} retry={reloadVersions} />
        )}
        {versionsResource.loading && versionsResource.data === undefined && !created ? (
          <ResourceNotice loading />
        ) : (
          <>
            {panel.key !== "delivery" && <NextStep panel={panel.key} />}
            <Activity mode={panel.key === "config" ? "visible" : "hidden"}>
              <div className="ptab-panel" id="ptab-config">
                <Suspense fallback={<ResourceNotice loading />}>
                  <ServerConfigs />
                </Suspense>
              </div>
            </Activity>
            {panel.key !== "config" && (
              <div className="ptab-panel" id={`ptab-${panel.key}`}>
                <Suspense fallback={<ResourceNotice loading />}>
                  <panel.View />
                </Suspense>
              </div>
            )}
          </>
        )}
      </div>
    </WorkspaceContext>
  );
}

function NextStep({ panel }: { panel: Panel }) {
  const { canWrite, version, files, filesLoading, validation, open, busy, busyKind } =
    useWorkspace();
  const published = version?.status === "published";
  const step: { title: string; action: string; tab: Panel; focus?: string } = !version
    ? { title: "创建第一个版本", action: "创建版本", tab: "versions", focus: "newVersion" }
    : published
      ? { title: `${version.version} 已发布`, action: "前往部署", tab: "delivery" }
      : validation?.ok === false
        ? { title: "发布前检查未通过", action: "查看问题", tab: "publish" }
        : !files.length
          ? { title: "上传版本文件", action: "上传文件", tab: "versions", focus: "fileInput" }
          : { title: "版本文件已就绪", action: "检查并发布", tab: "publish" };
  return (
    <section className="workspace-next" aria-label="下一步" data-testid="workspace-next-card">
      <Info size={17} aria-hidden="true" />
      <strong id="workspaceNextTitle">
        {busyKind === "upload" ? "正在上传版本文件" : step.title}
      </strong>
      {!canWrite && !published && <Badge>只读</Badge>}
      <Button
        small
        tone="secondary"
        data-testid="workspace-next"
        disabled={busy || filesLoading || (!canWrite && !published)}
        onClick={() => {
          open(step.tab);
          if (panel === step.tab && step.focus) {
            const input = document.getElementById(step.focus);
            if (step.focus === "fileInput") input?.click();
            else input?.focus();
          }
        }}
      >
        {busyKind === "upload" ? "正在上传…" : step.action}
      </Button>
    </section>
  );
}

export function useWorkspace() {
  const workspace = useContext(WorkspaceContext);
  if (!workspace) throw new Error("Project workspace is unavailable");
  return workspace;
}
