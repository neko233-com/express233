import { useRef, useState, type FormEvent } from "react";
import { ChevronDown, ChevronRight, File, Folder, Plus, Search, Upload } from "lucide-react";
import { query, request, useResource } from "../api";
import { useConsole } from "../app";
import { Code } from "../code";
import {
  csv,
  dateLabel,
  formText,
  formatBytes,
  suggestedVersion,
  type Project,
  type Version,
  type VersionFile,
} from "../types";
import {
  Badge,
  Button,
  Card,
  Disclosure,
  Empty,
  ResourceNotice,
  VersionBadge,
  useAction,
  useFeedback,
} from "../ui";
import { ValidationResult } from "./publish";
import { useWorkspace } from "./workspace";

export function Versions() {
  const { versions, latest, version, canWrite, createVersion, selectVersion } = useWorkspace();
  const [search, setSearch] = useState("");
  const create = useAction();
  const filtered = versions.filter((row) =>
    row.version.toLowerCase().includes(search.toLowerCase()),
  );
  const suggestion = suggestedVersion(versions);
  return (
    <>
      <div className="split-versions">
        <Card title="版本列表" className="version-sidebar">
          {canWrite && (
            <form
              className="inline-form"
              onSubmit={(event) => {
                event.preventDefault();
                const name = formText(new FormData(event.currentTarget), "version");
                if (name) void create.run(() => createVersion(name));
              }}
            >
              <input
                key={suggestion}
                id="newVersion"
                name="version"
                className="input"
                defaultValue={suggestion}
                aria-label="新版本号"
                data-testid="new-version-input"
                required
              />
              <Button
                type="submit"
                tone="primary"
                small
                disabled={create.busy}
                data-testid="add-version"
              >
                <Plus size={14} aria-hidden="true" />
                新建
              </Button>
            </form>
          )}
          <div className="input-icon">
            <Search size={15} aria-hidden="true" />
            <input
              className="input"
              type="search"
              data-testid="version-search"
              aria-label="搜索版本"
              placeholder="搜索版本…"
              value={search}
              onChange={(event) => setSearch(event.currentTarget.value)}
            />
          </div>
          <ul className="version-list" data-testid="version-list">
            {filtered.map((row) => (
              <li key={row.id}>
                <button
                  type="button"
                  className={`version-item ${row.version === version?.version ? "selected" : ""} ${row === latest ? "latest-published" : ""}`}
                  aria-current={row.version === version?.version ? "true" : undefined}
                  onClick={() => selectVersion(row.version)}
                >
                  <span className="version-item-line">
                    <strong>{row.version}</strong>
                    <VersionBadge status={row.status} />
                  </span>
                  <small>{dateLabel(row.published_at || row.created_at)}</small>
                </button>
              </li>
            ))}
          </ul>
          {!filtered.length && <p className="hint">{search ? "未找到匹配版本" : "暂无版本"}</p>}
        </Card>
        {version ? (
          <VersionDetail key={version.version} version={version} />
        ) : (
          <Card>
            <Empty>创建版本后上传程序、配置文件或归档。</Empty>
          </Card>
        )}
      </div>
      <div className="two-col inspector">
        <Retention />
        <Disclosure title="最近版本活动" badge={<Badge>{versions.length} 个版本</Badge>}>
          {!versions.length && <p className="hint">暂无版本</p>}
          <ol className="activity-list">
            {versions.slice(0, 8).map((row) => (
              <li key={row.id}>
                <code>{row.version}</code>
                <VersionBadge status={row.status} />
                <span>{dateLabel(row.published_at || row.created_at)}</span>
              </li>
            ))}
          </ol>
        </Disclosure>
      </div>
    </>
  );
}

function Retention() {
  const { project, projectPath, canWrite, versions, reloadVersions } = useWorkspace();
  const { updateProject } = useConsole();
  const { confirm, toast } = useFeedback();
  const action = useAction();
  const [limited, setLimited] = useState(project.max_published_versions > 0);
  const [count, setCount] = useState(String(project.max_published_versions || 10));
  function save(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    void action.run(async () => {
      const max = limited ? Number(count) : 0;
      const excess = versions.filter((row) => row.status === "published").length - max;
      if (
        limited &&
        excess > 0 &&
        !(await confirm({
          title: "更新版本保留策略",
          message: `保存后会尝试清理最早的 ${excess} 个已发布版本及其制品文件。`,
          confirmText: "保存并清理",
          danger: true,
        }))
      )
        return;
      const updated = await request<Project>(projectPath + "/version-retention", {
        method: "PUT",
        body: { max_published_versions: max },
      });
      updateProject({ ...project, ...updated });
      reloadVersions();
      toast(max ? `已保留最近 ${max} 个已发布版本` : "已设置为无限保留");
    });
  }
  return (
    <Disclosure
      title="版本保留策略"
      id="versionPolicyDetails"
      testId="version-retention-card"
      badge={
        <span id="versionRetentionBadge">
          <Badge>
            {project.max_published_versions
              ? `最近 ${project.max_published_versions} 个`
              : "无限保留"}
          </Badge>
        </span>
      }
    >
      <form onSubmit={save} className="form-stack">
        <label className="check-field">
          <input
            id="versionRetentionUnlimited"
            type="radio"
            name="retention"
            checked={!limited}
            disabled={!canWrite || action.busy}
            onChange={() => setLimited(false)}
          />
          无限保留
        </label>
        <div className="toolbar">
          <label className="check-field">
            <input
              id="versionRetentionLimited"
              type="radio"
              name="retention"
              checked={limited}
              disabled={!canWrite || action.busy}
              onChange={() => setLimited(true)}
            />
            仅保留最近
          </label>
          <input
            id="versionRetentionCount"
            className="input short"
            type="number"
            min={1}
            max={999}
            required
            value={count}
            aria-label="保留最近多少个已发布版本"
            disabled={!canWrite || !limited || action.busy}
            onChange={(event) => setCount(event.currentTarget.value)}
          />
          <span>个已发布版本</span>
        </div>
        <p className="hint">草稿和仍被部署或交付节点引用的版本会保留。</p>
        {canWrite && (
          <Button
            type="submit"
            tone="primary"
            disabled={action.busy}
            data-testid="save-version-retention"
          >
            保存策略
          </Button>
        )}
      </form>
    </Disclosure>
  );
}

function VersionDetail({ version }: { version: Version }) {
  const {
    activePath,
    busy,
    busyKind,
    canWrite,
    invalidateChecks,
    latest,
    open,
    progress,
    project,
    publish,
    reloadVersions,
    target,
    upload,
    validate,
    validation,
  } = useWorkspace();
  const [tags, setTags] = useState("");
  const input = useRef<HTMLInputElement>(null);
  const action = useAction();
  const { confirm } = useFeedback();
  const published = version.status === "published";
  const editable = canWrite && !published;
  const blocked = busy || action.busy;
  const disabled = !editable || blocked;

  function review(endpoint: "submit-review" | "reject") {
    if (!activePath) return;
    void action.run(async () => {
      if (
        endpoint === "reject" &&
        !(await confirm({
          title: "驳回版本",
          message: `驳回 ${version.version} 并退回修改。`,
          confirmText: "驳回",
        }))
      )
        return;
      await request(activePath + "/" + endpoint, { method: "POST" });
      invalidateChecks();
      reloadVersions();
    });
  }
  return (
    <div
      className="card version-detail"
      data-testid="version-detail"
      onDragOver={
        editable
          ? (event) => {
              event.preventDefault();
            }
          : undefined
      }
      onDrop={
        editable
          ? (event) => {
              event.preventDefault();
              if (!disabled) void upload(Array.from(event.dataTransfer.files), parseTags(tags));
            }
          : undefined
      }
    >
      <header className="version-detail-head">
        <div>
          <div className="version-title">
            <h2 id="verDetailTitle">{version.version}</h2>
            <VersionBadge status={version.status} testId="ver-status" />
          </div>
          <div className="hint timestamp">
            创建 {dateLabel(version.created_at)}
            {version.published_at && <> · 发布 {dateLabel(version.published_at)}</>}
          </div>
        </div>
        <div className="toolbar">
          {editable && (
            <Button tone="primary" small disabled={disabled} onClick={() => input.current?.click()}>
              <Upload size={15} aria-hidden="true" />
              上传文件
            </Button>
          )}
          {published && (
            <a
              href={activePath + "/download"}
              className="btn btn-secondary btn-sm"
              data-testid="download-version"
            >
              下载
            </a>
          )}
          {canWrite && (
            <Button
              tone="danger"
              small
              disabled={blocked}
              onClick={() =>
                void action.run(async () => {
                  if (
                    !(await confirm({
                      title: "删除版本",
                      message: `删除版本 ${version.version} 和其制品文件。`,
                      confirmText: "删除",
                      danger: true,
                    }))
                  )
                    return;
                  await request(activePath + "?confirm=yes", { method: "DELETE" });
                  invalidateChecks();
                  reloadVersions();
                })
              }
            >
              删除版本
            </Button>
          )}
        </div>
      </header>
      <input
        className="visually-hidden"
        tabIndex={-1}
        aria-label="上传版本文件"
        type="file"
        id="fileInput"
        multiple
        data-testid="file-input"
        disabled={disabled}
        ref={input}
        onChange={(event) => {
          const files = Array.from(event.currentTarget.files ?? []);
          event.currentTarget.value = "";
          void upload(files, parseTags(tags));
        }}
      />
      {editable && (
        <div className="upload-zone">
          <span className="hint">拖入文件或 ZIP、TAR 归档</span>
          <input
            className="input"
            value={tags}
            aria-label="上传文件标签"
            placeholder="可选标签，如 linux-amd64"
            disabled={blocked}
            data-testid="upload-tags"
            onChange={(event) => setTags(event.currentTarget.value)}
          />
        </div>
      )}
      {progress && (
        <p className="hint" id="versionUploadStatus" role="status">
          {progress}
        </p>
      )}
      <FileBrowser editable={editable} />
      <Disclosure title="高级：批量文件标签与构建来源" id="versionAdvanced">
        <div className="form-stack">
          <Provenance version={version} />
          {editable && <TagBatch />}
        </div>
      </Disclosure>
      <div className="toolbar version-actions">
        {!published && (
          <Button
            small
            onClick={() => void validate()}
            data-testid="validate-version"
            disabled={blocked}
          >
            {busyKind === "validate" ? "正在检查…" : "发布前检查"}
          </Button>
        )}
        {editable && (version.status === "draft" || version.status === "rejected") && (
          <Button small disabled={blocked} onClick={() => review("submit-review")}>
            提交审批
          </Button>
        )}
        {editable && version.status === "pending_review" && (
          <Button small tone="danger" disabled={blocked} onClick={() => review("reject")}>
            驳回
          </Button>
        )}
        {editable && (
          <Button
            small
            tone="primary"
            data-testid="publish-version"
            disabled={blocked || version.status === "rejected"}
            onClick={() => void publish()}
          >
            {busyKind === "publish" ? "正在发布…" : "正式发布"}
          </Button>
        )}
        {version === latest && (
          <Button small onClick={() => open("delivery")}>
            部署上线
          </Button>
        )}
      </div>
      <ValidationResult
        result={validation}
        checking={busyKind === "validate"}
        testId="validate-result"
      />
      {published && target && (
        <Disclosure title="拉取命令">
          <Code
            text={
              "express233-cli pull " +
              ["--project", "--server-id", "--version"]
                .map(
                  (flag, i) =>
                    flag +
                    " '" +
                    [project.name, target, version.version][i].replaceAll("'", "'\"'\"'") +
                    "'",
                )
                .join(" ")
            }
            testId="deploy-cmd"
          />
          <Button small onClick={() => open("deploy")}>
            生成部署脚本
          </Button>
        </Disclosure>
      )}
    </div>
  );
}

function Provenance({ version }: { version: Version }) {
  return (
    <div className="hint provenance-summary" data-testid="version-provenance">
      <div>
        {version.vcs?.commit ? (
          <>
            <span>
              {version.vcs.provider || "git"} · {version.vcs.ref || "—"} ·{" "}
            </span>
            <code>{version.vcs.commit}</code>
          </>
        ) : (
          "未绑定 VCS 来源"
        )}
      </div>
      <div>
        {version.artifact?.sha256 ? (
          <>
            <span>制品 SHA-256 </span>
            <code>{version.artifact.sha256}</code>
            <span>
              {" "}
              · {version.artifact.file_count} 个文件 · {formatBytes(version.artifact.bytes)}
            </span>
          </>
        ) : (
          "发布时生成制品清单"
        )}
      </div>
    </div>
  );
}

function parseTags(input: string) {
  return csv(input.replaceAll(";", " ").toLowerCase());
}

function TagBatch() {
  const { activePath, busy, refreshFiles, invalidateChecks } = useWorkspace();
  const action = useAction();
  return (
    <form
      className="form-stack"
      onSubmit={(event) => {
        event.preventDefault();
        const data = new FormData(event.currentTarget);
        const items = formText(data, "paths")
          .split(/\r?\n/)
          .map((part) => part.trim())
          .filter(Boolean);
        const patterns = items.filter((part) => /[*?[\]]/.test(part));
        const paths = items.filter((part) => !patterns.includes(part));
        void action.run(async () => {
          await request(activePath + "/file-tags/batch", {
            method: "POST",
            body: {
              paths,
              patterns,
              mode: formText(data, "mode"),
              tags: parseTags(formText(data, "tags")),
            },
          });
          invalidateChecks();
          refreshFiles();
        });
      }}
    >
      <textarea
        className="input"
        name="paths"
        aria-label="批量路径或 glob"
        placeholder="每行一个路径或 glob"
        required
        rows={3}
        data-testid="tag-batch-paths"
      />
      <div className="toolbar">
        <input
          className="input"
          name="tags"
          aria-label="批量标签"
          placeholder="标签，如 linux-amd64"
          data-testid="tag-batch-tags"
        />
        <select className="input" name="mode" aria-label="批量模式" data-testid="tag-batch-mode">
          <option value="set">设置</option>
          <option value="add">追加</option>
          <option value="remove">移除</option>
          <option value="clear">清空为 all</option>
        </select>
        <Button type="submit" small disabled={busy || action.busy} data-testid="tag-batch-apply">
          批量处理
        </Button>
      </div>
    </form>
  );
}

interface FileNode {
  name: string;
  path: string;
  file?: VersionFile;
  children: Map<string, FileNode>;
}

function buildTree(files: VersionFile[]) {
  const root = new Map<string, FileNode>();
  for (const file of files) {
    let children = root;
    let path = "";
    for (const part of file.path.split("/")) {
      path = path ? path + "/" + part : part;
      const node = children.get(part) ?? {
        name: part,
        path,
        children: new Map<string, FileNode>(),
      };
      if (path === file.path) node.file = file;
      children.set(part, node);
      children = node.children;
    }
  }
  return root;
}

function displayTags(tags?: string[]) {
  return tags?.length ? tags.map((tag) => (tag === "*" ? "all" : tag)) : ["all"];
}

function FileBrowser({ editable }: { editable: boolean }) {
  const { activePath, files, filesLoading, filesError, refreshFiles } = useWorkspace();
  const [search, setSearch] = useState("");
  const [selected, setSelected] = useState("");
  const [closed, setClosed] = useState<string[]>([]);
  const file = files.find((row) => row.path === selected);
  const content = useResource<{ path: string; content: string; size: number }>(
    file && query(activePath + "/files/content", { path: file.path }),
  );
  const needle = search.trim().toLowerCase();
  const matching = files.filter(
    (row) =>
      row.path.toLowerCase().includes(needle) ||
      displayTags(row.tags).some((tag) => tag.includes(needle)),
  );
  const tree = buildTree(matching);

  return (
    <section className="version-files">
      <header className="card-head">
        <h3>
          文件 <span className="hint">· {files.length}</span>
        </h3>
        <div className="input-icon">
          <Search size={14} aria-hidden="true" />
          <input
            className="input"
            type="search"
            value={search}
            placeholder="搜索文件路径或标签…"
            aria-label="搜索文件路径或标签"
            data-testid="file-search"
            onChange={(event) => setSearch(event.currentTarget.value)}
          />
        </div>
      </header>
      <ResourceNotice
        loading={filesLoading && !files.length}
        error={filesError}
        retry={refreshFiles}
      />
      <div className="file-browser">
        <div className="file-list" data-testid="file-list">
          {tree.size ? (
            <FileTree
              nodes={tree}
              selected={selected}
              closed={needle ? [] : closed}
              onSelect={setSelected}
              toggle={(path) =>
                setClosed((items) =>
                  items.includes(path) ? items.filter((item) => item !== path) : [...items, path],
                )
              }
            />
          ) : (
            <p className="hint padded">{files.length ? "没有匹配文件" : "暂无文件"}</p>
          )}
        </div>
        <section className="file-preview" aria-label="文件预览">
          <header className="code-head">
            <strong id="filePreviewPath">{file?.path || "文件预览"}</strong>
            {content.data && <span className="hint">{formatBytes(content.data.size)}</span>}
            {file && editable && <FileActions file={file} />}
          </header>
          <ResourceNotice loading={content.loading} error={content.error} retry={content.refresh} />
          {content.data ? (
            <Code text={content.data.content} id="filePreviewBody" path={file?.path} />
          ) : (
            !file && <Empty>选择文件查看内容</Empty>
          )}
        </section>
      </div>
    </section>
  );
}

function FileTree({
  nodes,
  selected,
  closed,
  onSelect,
  toggle,
}: {
  nodes: Map<string, FileNode>;
  selected: string;
  closed: string[];
  onSelect: (path: string) => void;
  toggle: (path: string) => void;
}) {
  const ordered = Array.from(nodes.values()).sort(
    (a, b) => Number(Boolean(a.file)) - Number(Boolean(b.file)) || a.name.localeCompare(b.name),
  );
  return (
    <ul className="tree-list">
      {ordered.map((node) =>
        node.file ? (
          <li key={node.path} className="tree-file">
            <button
              type="button"
              className={`file-path tree-row ${selected === node.path ? "selected" : ""}`}
              title={node.path}
              onClick={() => onSelect(node.path)}
            >
              <File size={15} aria-hidden="true" />
              <span>{node.name}</span>
              {displayTags(node.file.tags).map((tag) => (
                <Badge key={tag}>{tag}</Badge>
              ))}
            </button>
          </li>
        ) : (
          <li key={node.path}>
            <button
              type="button"
              className="tree-row"
              aria-expanded={!closed.includes(node.path)}
              title={node.path}
              onClick={() => toggle(node.path)}
            >
              {closed.includes(node.path) ? <ChevronRight size={13} /> : <ChevronDown size={13} />}
              <Folder size={15} aria-hidden="true" />
              <span>{node.name}</span>
            </button>
            {!closed.includes(node.path) && (
              <FileTree
                nodes={node.children}
                selected={selected}
                closed={closed}
                onSelect={onSelect}
                toggle={toggle}
              />
            )}
          </li>
        ),
      )}
    </ul>
  );
}

function FileActions({ file }: { file: VersionFile }) {
  const { activePath, busy, invalidateChecks, refreshFiles } = useWorkspace();
  const { prompt, confirm } = useFeedback();
  const action = useAction();
  function edit(clear: boolean) {
    void action.run(async () => {
      const endpoint = activePath + "/file-tags";
      if (clear) await request(query(endpoint, { path: file.path }), { method: "DELETE" });
      else {
        const next = await prompt({
          title: "设置文件标签",
          message: file.path + " · 逗号分隔，留空适用全部",
          value: displayTags(file.tags).join(","),
        });
        if (next === null) return;
        await request(endpoint, {
          method: "PUT",
          body: { path: file.path, tags: parseTags(next) },
        });
      }
      invalidateChecks();
      refreshFiles();
    });
  }
  return (
    <div className="toolbar">
      <Button small tone="ghost" disabled={busy || action.busy} onClick={() => edit(false)}>
        编辑标签
      </Button>
      <Button small tone="ghost" disabled={busy || action.busy} onClick={() => edit(true)}>
        清空标签
      </Button>
      <Button
        small
        tone="danger"
        disabled={busy || action.busy}
        onClick={() =>
          void action.run(async () => {
            if (
              !(await confirm({
                title: "删除文件",
                message: file.path,
                confirmText: "删除",
                danger: true,
              }))
            )
              return;
            await request(query(activePath + "/files", { path: file.path }), { method: "DELETE" });
            invalidateChecks();
            refreshFiles();
          })
        }
      >
        删除文件
      </Button>
    </div>
  );
}
