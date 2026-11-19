import { useEffect, useEffectEvent, useState, type FormEvent } from "react";
import { diffLines } from "diff";
import { FileDiff, RefreshCw } from "lucide-react";
import { query, request, useResource } from "../api";
import { Code } from "../code";
import { formText } from "../types";
import { Badge, Button, Card, Empty, ResourceNotice, useAction, useFeedback } from "../ui";
import { useWorkspace } from "./workspace";

interface PreviewReport {
  rendered_files?: { path: string; basename: string; before: string; after: string }[];
  warnings?: string[];
  post_hook?: string;
  post_hook_plan?: string[];
}
interface DiffFile {
  path: string;
  basename: string;
  from: string;
  to: string;
  change: string;
}
interface DiffReport {
  from_version: string;
  to_version: string;
  server_id: string;
  file_diffs?: DiffFile[];
}

export function Preview() {
  const {
    project,
    version,
    target,
    setTarget,
    serverIDs,
    revision,
    confirmPreview,
    clearPreview,
    open,
  } = useWorkspace();
  const [requested, setRequested] = useState(target.trim());
  const [refreshID, refresh] = useState(0);
  const { toast } = useFeedback();
  const sid = target.trim();
  const path =
    version && sid && sid === requested
      ? query("/api/deploy/preview", {
          project: project.name,
          version: version.version,
          server_id: sid,
        })
      : undefined;
  return (
    <Card
      title="配置预览"
      description="对照原始配置与替换后的内容。"
      actions={
        sid && (
          <code>
            {project.name}|{sid}
          </code>
        )
      }
    >
      <form
        className="toolbar spaced"
        onSubmit={(event) => {
          event.preventDefault();
          if (!version || !sid) {
            toast("请选择版本并填写 server_id", "warn");
            return;
          }
          setTarget(sid);
          clearPreview();
          setRequested(sid);
          refresh((value) => value + 1);
        }}
      >
        <ServerID
          value={target}
          setValue={(value) => {
            setRequested("");
            setTarget(value);
          }}
          ids={serverIDs}
          id="verPreviewServerId"
          testId="preview-server-id"
        />
        <Button type="submit" tone="primary" data-testid="preview-submit">
          <RefreshCw size={14} aria-hidden="true" />
          刷新预览
        </Button>
      </form>
      <div data-testid="preview-table">
        <PreviewResult
          key={`${path}:${revision}:${refreshID}`}
          path={path}
          onConfirm={() => {
            confirmPreview();
            open("publish");
            toast("配置预览已确认");
          }}
        />
      </div>
    </Card>
  );
}

function PreviewResult({ path, onConfirm }: { path?: string; onConfirm: () => void }) {
  const response = useResource<PreviewReport>(path);
  const { toast } = useFeedback();
  const data = response.data;
  const files =
    data?.rendered_files
      ?.filter((file) => file.before !== file.after)
      .map((file) => ({
        path: file.path,
        basename: file.basename,
        from: file.before,
        to: file.after,
        change: "modified",
      })) ?? [];
  return (
    <>
      <ResourceNotice loading={response.loading} />
      {response.error && <PreviewFailure error={response.error} notify={toast} />}
      {data && (
        <>
          {data.warnings?.map((warning) => (
            <p className="notice" key={warning}>
              {warning}
            </p>
          ))}
          {data.post_hook && (
            <p className="hint">
              启动钩子 <code>{data.post_hook}</code>
            </p>
          )}
          {data.post_hook_plan?.map((line, index) => (
            <p className="hint" key={index}>
              {line}
            </p>
          ))}
          <DiffBrowser
            files={files}
            before="原始配置"
            after="替换后配置"
            preview
            empty="没有匹配的覆盖配置"
          />
        </>
      )}
      {!path && <Empty>选择版本和 server_id 后生成预览。</Empty>}
      <div className="toolbar spaced">
        <Button
          small
          id="btnConfirmPreview"
          disabled={!data || response.loading || !!response.error}
          onClick={onConfirm}
        >
          确认预览并继续
        </Button>
      </div>
    </>
  );
}

function PreviewFailure({
  error,
  notify,
}: {
  error: string;
  notify: (message: string, kind: "error") => void;
}) {
  const announce = useEffectEvent(() => notify(error, "error"));
  useEffect(() => {
    announce();
  }, [error]);
  return <ResourceNotice error={`预览失败：${error}`} />;
}

export function VersionDiff() {
  const { project, versions, target, serverIDs } = useWorkspace();
  const [report, setReport] = useState<DiffReport>();
  const action = useAction();
  const { toast } = useFeedback();
  function compare(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    const data = new FormData(event.currentTarget);
    const from = formText(data, "from");
    const to = formText(data, "to");
    const server_id = formText(data, "server");
    if (!from || !to || !server_id || from === to) {
      toast("请选择不同版本并填写 server_id", "warn");
      return;
    }
    setReport(undefined);
    void action.run(async () => {
      setReport(
        await request<DiffReport>(
          query("/api/deploy/diff", { project: project.name, from, to, server_id }),
        ),
      );
    });
  }
  return (
    <Card title="版本对比" description="查看两个版本应用指定分服配置后的差异。">
      <form className="toolbar spaced" onSubmit={compare}>
        <select
          id="diffFromVer"
          className="input"
          name="from"
          aria-label="旧版本"
          defaultValue={versions[1]?.version ?? ""}
        >
          <option value="">旧版本</option>
          {versions.map((row) => (
            <option key={row.id}>{row.version}</option>
          ))}
        </select>
        <select
          id="diffToVer"
          className="input"
          name="to"
          aria-label="新版本"
          defaultValue={versions[0]?.version ?? ""}
        >
          <option value="">新版本</option>
          {versions.map((row) => (
            <option key={row.id}>{row.version}</option>
          ))}
        </select>
        <ServerID ids={serverIDs} defaultValue={target} id="diffServerId" name="server" />
        <Button type="submit" tone="primary" disabled={action.busy}>
          生成 diff
        </Button>
      </form>
      <div id="versionDiffOut">
        <ResourceNotice loading={action.busy} />
        {report && (
          <DiffBrowser
            key={`${report.from_version}:${report.to_version}:${report.server_id}`}
            files={report.file_diffs ?? []}
            before={`旧版本 ${report.from_version}`}
            after={`新版本 ${report.to_version}`}
          />
        )}
      </div>
    </Card>
  );
}

export function ServerID({
  ids,
  id,
  testId,
  value,
  defaultValue,
  setValue,
  name,
}: {
  ids: string[];
  id: string;
  testId?: string;
  value?: string;
  defaultValue?: string;
  setValue?: (value: string) => void;
  name?: string;
}) {
  return (
    <>
      <input
        className="input"
        id={id}
        name={name}
        value={value}
        defaultValue={defaultValue}
        onChange={setValue ? (event) => setValue(event.currentTarget.value) : undefined}
        list={id + "-options"}
        aria-label="server_id"
        data-testid={testId}
        placeholder="选择或输入 server_id"
        autoComplete="off"
      />
      <datalist id={id + "-options"}>
        {ids.map((server) => (
          <option key={server} value={server} />
        ))}
      </datalist>
    </>
  );
}

function DiffBrowser({
  files,
  before,
  after,
  preview,
  empty = "无文件差异",
}: {
  files: DiffFile[];
  before: string;
  after: string;
  preview?: boolean;
  empty?: string;
}) {
  const [needle, setNeedle] = useState("");
  const [selected, setSelected] = useState("");
  const matches = files.filter((file) =>
    (file.path || file.basename).toLowerCase().includes(needle.trim().toLowerCase()),
  );
  const file = matches.find((item) => (item.path || item.basename) === selected) ?? matches[0];
  if (!files.length) return <Empty>{empty}</Empty>;
  return (
    <div className="diff-grid">
      <aside className="diff-files">
        <input
          type="search"
          className="input full"
          value={needle}
          aria-label="搜索差异文件"
          placeholder="搜索路径…"
          onChange={(event) => setNeedle(event.currentTarget.value)}
        />
        <ul className="tree-list">
          {matches.map((row, index) => (
            <li key={row.path || row.basename}>
              <button
                type="button"
                data-diff-index={index}
                className={`tree-row ${row === file ? "selected" : ""}`}
                title={row.path || row.basename}
                onClick={() => setSelected(row.path || row.basename)}
              >
                <FileDiff size={15} aria-hidden="true" />
                <span>{row.path || row.basename}</span>
                <Badge
                  kind={
                    row.change === "removed" ? "danger" : row.change === "added" ? "ok" : "info"
                  }
                >
                  {row.change === "removed" ? "删除" : row.change === "added" ? "新增" : "修改"}
                </Badge>
              </button>
            </li>
          ))}
        </ul>
        {!matches.length && <p className="hint">没有匹配文件</p>}
      </aside>
      {file && <DiffPanes file={file} before={before} after={after} preview={preview} />}
    </div>
  );
}

function DiffPanes({
  file,
  before,
  after,
  preview,
}: {
  file: DiffFile;
  before: string;
  after: string;
  preview?: boolean;
}) {
  const left = file.from || "";
  const right = file.to || "";
  const changes =
    left.length + right.length < 240_000 ? diffLines(left, right, { timeout: 50 }) : undefined;
  const panes = [
    {
      title: before,
      value: left,
      id: preview ? "verPreviewOriginalBody" : undefined,
      exclude: "added" as const,
    },
    {
      title: after,
      value: right,
      id: preview ? "verPreviewRenderedBody" : undefined,
      testId: preview ? "preview-rendered-body" : undefined,
      exclude: "removed" as const,
    },
  ];
  return (
    <div className="diff-panes">
      {panes.map((pane) => (
        <section key={pane.title}>
          <header className="code-head">{pane.title}</header>
          {changes ? (
            <pre className="code" tabIndex={0} id={pane.id} data-testid={pane.testId}>
              <code>
                {changes.map(
                  (change, index) =>
                    !change[pane.exclude] && (
                      <span
                        className={
                          change.added ? "diff-add" : change.removed ? "diff-remove" : undefined
                        }
                        key={index}
                      >
                        {change.value}
                      </span>
                    ),
                )}
              </code>
            </pre>
          ) : (
            <Code text={pane.value} id={pane.id} testId={pane.testId} path={file.path} />
          )}
        </section>
      ))}
    </div>
  );
}
