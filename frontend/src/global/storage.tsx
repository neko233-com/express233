import { useEffect, useState } from "react";
import { ChevronDown, ChevronRight, File, Folder, Search } from "lucide-react";
import { query, request, useResource } from "../api";
import { useConsole } from "../app";
import { formatBytes } from "../types";
import {
  Badge,
  Button,
  Card,
  Empty,
  PageHeader,
  ResourceNotice,
  useAction,
  useFeedback,
} from "../ui";

interface Overview {
  data_dir: string;
  total_bytes: number;
  available_bytes?: number;
  project_count: number;
  version_count: number;
  index_entry_count: number;
  index_updated_at?: string;
  blob_stats: { blob_count: number; total_bytes: number; total_refs: number };
  categories?: { name: string; label: string; bytes: number }[];
}
interface TreeNode {
  path: string;
  name: string;
  kind: string;
  size_bytes: number;
  meta?: { status?: string };
  children?: TreeNode[];
}
interface DeletePlan {
  path: string;
  kind: string;
  size_bytes: number;
  allowed: boolean;
  deny_reason?: string;
  warnings?: string[];
  related?: string[];
}
const kindNames: Record<string, string> = {
  project: "项目",
  version: "版本",
  file: "文件",
  folder: "目录",
  tenant: "租户",
  blob: "Blob",
  orphan_blob: "孤立 Blob",
};

export function Storage() {
  const { session, refreshProjects } = useConsole();
  const [scope, setScope] = useState("");
  const [text, setText] = useState("");
  const [search, setSearch] = useState("");
  const [selected, setSelected] = useState("");
  const overview = useResource<Overview>("/api/storage/overview");
  const tree = useResource<TreeNode>(query("/api/storage/tree", { path: scope }));
  const hits = useResource<{ hits?: TreeNode[] }>(
    search ? query("/api/storage/search", { q: search }) : undefined,
  );
  const deletePlan = useResource<DeletePlan>(
    selected ? query("/api/storage/delete-plan", { path: selected }) : undefined,
  );
  const action = useAction();
  const { confirm, toast } = useFeedback();
  const data = overview.data;
  const plan = deletePlan.data;
  useEffect(() => {
    const timer = window.setTimeout(() => setSearch(text.trim()), 250);
    return () => window.clearTimeout(timer);
  }, [text]);

  function refresh() {
    overview.refresh();
    tree.refresh();
    hits.refresh();
    deletePlan.refresh();
  }

  return (
    <div className="section-stack" data-testid="storage-panel">
      <PageHeader title="存储空间" description="查看制品占用、文件索引和关联删除范围。">
        <Button small onClick={refresh}>
          刷新
        </Button>
        <Button
          tone="primary"
          small
          data-testid="storage-reindex"
          disabled={action.busy}
          onClick={() =>
            void action.run(async () => {
              const result = await request<{ entries: number }>("/api/storage/reindex", {
                method: "POST",
              });
              refresh();
              toast(`索引已重建（${result.entries} 条）`);
            })
          }
        >
          重建索引
        </Button>
      </PageHeader>
      <ResourceNotice
        loading={overview.loading && !data}
        error={overview.error}
        retry={overview.refresh}
      />
      <div className="stat-grid" data-testid="storage-stats">
        <div className="card stat">
          <span>统计大小</span>
          <strong>{data ? formatBytes(data.total_bytes) : "—"}</strong>
          <small>
            {data?.available_bytes != null ? `本地可用 ${formatBytes(data.available_bytes)}` : ""}
          </small>
        </div>
        <div className="card stat">
          <span>项目 / 版本</span>
          <strong>{data ? `${data.project_count} / ${data.version_count}` : "—"}</strong>
        </div>
        <div className="card stat">
          <span>Blob 去重</span>
          <strong>{data?.blob_stats.blob_count ?? "—"}</strong>
          <small>{data && formatBytes(data.blob_stats.total_bytes)}</small>
        </div>
        <div className="card stat">
          <span>索引记录</span>
          <strong>{data?.index_entry_count ?? "—"}</strong>
        </div>
      </div>
      <Card title="容量分布" description={data?.data_dir && <code>{data.data_dir}</code>}>
        <div className="capacity" data-testid="storage-bars">
          {data?.categories?.map((category) => (
            <div key={category.name}>
              <div className="card-head">
                <span>{category.label}</span>
                <span className="hint">{formatBytes(category.bytes)}</span>
              </div>
              <progress
                max={Math.max(data.total_bytes, 1)}
                value={category.bytes}
                aria-label={category.label}
              />
            </div>
          ))}
        </div>
      </Card>
      <div className="two-col storage-grid">
        <Card
          title="文件目录"
          actions={
            session.is_admin && (
              <select
                className="input"
                aria-label="存储范围"
                value={scope}
                onChange={(event) => {
                  setScope(event.currentTarget.value);
                  setSelected("");
                }}
              >
                <option value="">项目目录</option>
                <option value="blobs">Blob 对象</option>
              </select>
            )
          }
        >
          <div className="input-icon">
            <Search size={15} aria-hidden="true" />
            <input
              className="input"
              type="search"
              value={text}
              aria-label="索引搜索"
              placeholder="搜索路径或文件名…"
              data-testid="storage-search"
              onChange={(event) => setText(event.currentTarget.value)}
            />
          </div>
          {search ? (
            <div className="option-list" data-testid="storage-search-hits">
              <ResourceNotice
                loading={hits.loading && !hits.data}
                error={hits.error}
                retry={hits.refresh}
              />
              {hits.data?.hits?.map((row) => (
                <button
                  type="button"
                  className="option-row"
                  key={row.path}
                  onClick={() => {
                    setSelected(row.path);
                    setText("");
                    setSearch("");
                  }}
                >
                  <strong>{row.name}</strong>
                  <small>
                    {row.path} · {formatBytes(row.size_bytes)}
                  </small>
                </button>
              ))}
              {hits.data && !hits.data.hits?.length && (
                <p className="hint">没有匹配项，可重建索引后重试。</p>
              )}
            </div>
          ) : (
            <div className="storage-tree" data-testid="storage-tree">
              <ResourceNotice
                loading={tree.loading && !tree.data}
                error={tree.error}
                retry={tree.refresh}
              />
              {tree.data && <Branch node={tree.data} selected={selected} select={setSelected} />}
            </div>
          )}
        </Card>
        <Card title="文件详情" id="storageDetail">
          <div data-testid="storage-detail">
            <ResourceNotice
              loading={deletePlan.loading && !plan}
              error={deletePlan.error}
              retry={deletePlan.refresh}
            />
            {!selected && <Empty>选择目录、版本或文件查看详情。</Empty>}
            {plan && (
              <>
                <dl className="checklist">
                  <div>
                    <dt>路径</dt>
                    <dd>
                      <code>{plan.path}</code>
                    </dd>
                  </div>
                  <div>
                    <dt>类型</dt>
                    <dd>{kindNames[plan.kind] || plan.kind || "—"}</dd>
                  </div>
                  <div>
                    <dt>大小</dt>
                    <dd>{formatBytes(plan.size_bytes)}</dd>
                  </div>
                </dl>
                {plan.deny_reason && <p className="hint">{plan.deny_reason}</p>}
                {plan.warnings?.map((warning) => (
                  <p className="warn-text" key={warning}>
                    {warning}
                  </p>
                ))}
                {plan.related?.length ? (
                  <ul>
                    {plan.related.map((item) => (
                      <li key={item}>{item}</li>
                    ))}
                  </ul>
                ) : null}
                {plan.allowed && (
                  <Button
                    small
                    tone="danger"
                    id="btnStorageDelete"
                    data-testid="storage-delete"
                    disabled={action.busy}
                    onClick={() =>
                      void action.run(async () => {
                        const snapshot = await request<DeletePlan>(
                          query("/api/storage/delete-plan", { path: plan.path }),
                        );
                        if (!snapshot.allowed)
                          throw new Error(snapshot.deny_reason || "当前不可删除");
                        if (
                          !(await confirm({
                            title: "确认删除",
                            message: [
                              `删除 ${snapshot.path}（${formatBytes(snapshot.size_bytes)}）`,
                              ...(snapshot.warnings ?? []),
                              ...(snapshot.related ?? []),
                            ].join("\n"),
                            danger: true,
                            confirmText: "删除",
                          }))
                        )
                          return;
                        await request("/api/storage/items", {
                          method: "DELETE",
                          body: { path: snapshot.path },
                        });
                        setSelected("");
                        refresh();
                        if (snapshot.kind === "project") refreshProjects();
                        toast("已删除");
                      })
                    }
                  >
                    删除
                  </Button>
                )}
              </>
            )}
          </div>
        </Card>
      </div>
    </div>
  );
}

function Branch({
  node,
  selected,
  select,
}: {
  node: TreeNode;
  selected: string;
  select: (path: string) => void;
}) {
  const folder = Boolean(node.children?.length);
  const [expanded, setExpanded] = useState(node.kind !== "version");
  return (
    <div>
      <button
        type="button"
        className={`tree-row storage-tree-row ${selected === node.path ? "selected" : ""}`}
        title={node.path}
        data-path={node.path}
        data-storage-folder={folder ? node.path : undefined}
        aria-expanded={folder ? expanded : undefined}
        onClick={() => {
          if (folder) setExpanded(!expanded);
          select(node.path);
        }}
      >
        {folder ? (
          <>
            {expanded ? <ChevronDown size={13} /> : <ChevronRight size={13} />}
            <Folder size={15} />
          </>
        ) : (
          <File size={15} />
        )}
        <span>{node.name}</span>
        {node.meta?.status && (
          <Badge kind={node.meta.status === "published" ? "ok" : "neutral"}>
            {node.meta.status}
          </Badge>
        )}
        <small>{formatBytes(node.size_bytes)}</small>
      </button>
      {expanded && folder && (
        <div className="tree-children">
          {node.children?.map((child) => (
            <Branch node={child} key={child.path} selected={selected} select={select} />
          ))}
        </div>
      )}
    </div>
  );
}
