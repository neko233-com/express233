import { Activity, createContext, lazy, Suspense, useContext, useState } from "react";
import { Link, useNavigate, useParams, useSearchParams } from "react-router";
import { Folder, Settings2 } from "lucide-react";
import { ApiError, errorMessage, query, request, uploadFile, useResource } from "../api";
import { NotFound, useConsole } from "../app";
import {
  type Project,
  type Validation,
  type Version,
  type VersionFile,
  versionPath,
} from "../types";
import { Button, ResourceNotice, useAction, useFeedback } from "../ui";
import { Publish } from "./publish";
import { Versions } from "./versions";

const ServerConfigs = lazy(() =>
  import("./config").then((module) => ({ default: module.ServerConfigs })),
);
const Preview = lazy(() => import("./preview").then((module) => ({ default: module.Preview })));
const Diff = lazy(() => import("./preview").then((module) => ({ default: module.VersionDiff })));
const Delivery = lazy(() => import("./delivery").then((module) => ({ default: module.Delivery })));
const Automation = lazy(() =>
  import("./automation").then((module) => ({ default: module.Automation })),
);
const Cluster = lazy(() => import("./cluster").then((module) => ({ default: module.Cluster })));
const Team = lazy(() => import("./team").then((module) => ({ default: module.Team })));
const DeployScript = lazy(() =>
  import("./script").then((module) => ({ default: module.DeployScript })),
);

const panels = [
  { key: "versions", label: "版本包", name: "版本", View: Versions, needsVersions: true },
  { key: "config", label: "分服配置", View: ServerConfigs },
  { key: "preview", label: "配置预览", name: "拉取预览", View: Preview, needsVersions: true },
  { key: "publish", label: "发布版本", View: Publish, needsVersions: true },
  { key: "delivery", label: "部署上线", View: Delivery },
  { key: "automation", label: "高级发布", View: Automation },
  { key: "diff", label: "版本对比", name: "差异", View: Diff, needsVersions: true },
  { key: "cluster", label: "集群节点", View: Cluster },
  { key: "deploy", label: "部署脚本", View: DeployScript, needsVersions: true },
  { key: "team", label: "团队", View: Team },
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
  missingVersion?: string;
  versionsLoading: boolean;
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
  panelLink: (tab: Panel, params?: Record<string, string | number | undefined>) => string;
  open: (tab: Panel) => void;
  selectVersion: (version: string) => void;
  createVersion: (version: string) => Promise<void>;
  reloadVersions: () => void;
  validation?: Validation;
  validate: () => Promise<void>;
  publish: () => Promise<void>;
  review: (endpoint: "submit-review" | "reject") => Promise<void>;
  canRetryPublish: boolean;
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
  const { projects, projectsLoading, projectsError, refreshProjects } = useConsole();
  const project = projects.find((p) => String(p.id) === projectId);
  const panel = panels.find((entry) => entry.key === tab);
  if (!project && projectsLoading) return <ResourceNotice loading />;
  if (!project && projectsError)
    return <ResourceNotice error={projectsError} retry={refreshProjects} />;
  if (!project || !panel) return <NotFound />;
  return <ProjectScreen key={project.id} project={project} panel={panel} />;
}

function ProjectScreen({ project, panel }: { project: Project; panel: (typeof panels)[number] }) {
  const { session, refreshProjects } = useConsole();
  const canWrite = session.is_admin || project.my_role === "admin";
  const projectPath = `/api/projects/${project.id}`;
  const versionsResource = useResource<Version[] | null>(projectPath + "/versions");
  const serverIDs = useResource<{ server_ids: string[] }>(
    ["preview", "diff", "deploy"].includes(panel.key) ? "/api/server-ids" : undefined,
  );
  const navigate = useNavigate();
  const [search] = useSearchParams();
  const [created, setCreated] = useState<Version>();
  const [target, setServer] = useState("");
  const [confirmation, setConfirmation] = useState<ConfirmedPreview>();
  const [check, setCheck] = useState<VersionStatus>();
  const [publishRetryVersion, setPublishRetryVersion] = useState<string>();
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
  const requestedVersion = search.get("version");
  const version = requestedVersion
    ? versions.find((row) => row.version === requestedVersion)
    : versions[0];
  const missingVersion =
    !version && requestedVersion && !versionsResource.loading && !versionsResource.error
      ? requestedVersion
      : undefined;
  const latest = versions
    .filter((row) => row.status === "published")
    .toSorted(
      (a, b) => (b.published_at || "").localeCompare(a.published_at || "") || b.id - a.id,
    )[0];
  const activePath = version && versionPath(project.id, version.version);
  const filesResource = useResource<VersionFile[] | null>(
    activePath && (panel.key === "versions" || panel.key === "publish")
      ? activePath + "/file-tags"
      : undefined,
  );
  const files = filesResource.data ?? noFiles;
  const validation = check?.version === version?.version ? check?.result : undefined;
  const canRetryPublish =
    version?.status === "published" && version.version === publishRetryVersion;
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

  function panelLink(tab: Panel, params?: Record<string, string | number | undefined>) {
    return query(`/projects/${project.id}/${tab}`, {
      version: requestedVersion || version?.version,
      ...params,
    });
  }

  function open(tab: Panel) {
    if (tab !== panel.key) void navigate(panelLink(tab));
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
      versionsResource.loading ||
      !canWrite ||
      !(canRetryPublish || ["draft", "pending_review"].includes(version.status))
    )
      return;
    await action.run(async () => {
      if (
        !(await confirm({
          title: canRetryPublish ? "重试发布通知" : "正式发布",
          confirmText: canRetryPublish ? "重试" : "发布",
          message: `项目：${project.name}\n版本：${version.version}\n${canRetryPublish ? "重新通知已启用的 Hook 和自动跟随的 Pull 节点。" : "发布将锁定版本内容，并通知已启用的 Hook 和自动跟随的 Pull 节点。"}`,
        }))
      )
        return;
      setBusyKind("publish");
      try {
        await request(activePath + "/publish", { method: "POST" });
        setPublishRetryVersion((current) => (current === version.version ? undefined : current));
        toast(canRetryPublish ? "发布通知已提交" : `版本 ${version.version} 已发布`);
      } catch (error) {
        if (
          error instanceof ApiError &&
          error.status === 500 &&
          error.message.includes("retry this publish request")
        ) {
          setPublishRetryVersion(version.version);
        }
        throw error;
      } finally {
        reloadVersions();
        setBusyKind("");
      }
    });
  }

  async function review(endpoint: "submit-review" | "reject") {
    if (!activePath || !version || !canWrite || versionsResource.loading) return;
    await action.run(async () => {
      if (
        endpoint === "reject" &&
        !(await confirm({
          title: "驳回版本",
          message: `驳回 ${version.version} 并退回修改。`,
          confirmText: "驳回",
        }))
      )
        return;
      try {
        await request(activePath + "/" + endpoint, { method: "POST" });
        invalidateChecks();
        toast(endpoint === "reject" ? "已驳回版本" : "已提交审批");
      } finally {
        reloadVersions();
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
    missingVersion,
    versionsLoading: versionsResource.loading,
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
    panelLink,
    open,
    selectVersion,
    createVersion,
    reloadVersions,
    validation,
    validate,
    publish,
    review,
    canRetryPublish,
    busy: action.busy,
    busyKind,
    upload,
    progress,
  };

  return (
    <WorkspaceContext value={context}>
      <div id="projectWorkspace">
        <Link className="workspace-home muted-link" to="/" data-testid="navigate-back">
          <Folder size={15} aria-hidden="true" />
          项目工作区
        </Link>
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
            <Link
              key={entry.key}
              to={panelLink(
                entry.key,
                entry.key === panel.key ? Object.fromEntries(search) : undefined,
              )}
              className={`project-tab${entry.key === panel.key ? " active" : ""}${entry.key === "automation" ? " project-tab-secondary" : ""}`}
              data-ptab={entry.key}
              aria-label={"name" in entry ? entry.name : entry.label}
              aria-current={entry.key === panel.key ? "page" : undefined}
            >
              {entry.label}
            </Link>
          ))}
        </nav>
        {versionsResource.error && (
          <ResourceNotice error={versionsResource.error} retry={reloadVersions} />
        )}
        {"needsVersions" in panel &&
        versionsResource.loading &&
        versionsResource.data === undefined &&
        !created ? (
          <ResourceNotice loading />
        ) : (
          <>
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

export function useWorkspace() {
  const workspace = useContext(WorkspaceContext);
  if (!workspace) throw new Error("Project workspace is unavailable");
  return workspace;
}
