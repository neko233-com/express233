import { useRef, type Ref } from "react";
import { Link } from "react-router";
import { query, request, useResource } from "../api";
import { dateLabel } from "../types";
import {
  Badge,
  Button,
  Card,
  Disclosure,
  ResourceNotice,
  Table,
  useAction,
  useFeedback,
} from "../ui";
import {
  deploymentStatus,
  hookPayload,
  isPreview,
  selector,
  submitDeployment,
  usePolling,
  type DeliveryNode,
  type Deployment,
  type Hook,
  type HookEvent,
  type PushTask,
} from "./delivery-api";
import { HookEditor, TaskEditor } from "./delivery-forms";
import { useWorkspace } from "./workspace";

const previewNote = "仅核对目标，未连接 SSH，未变更服务器";

export function Delivery() {
  const { project, projectPath, latest, canWrite } = useWorkspace();
  const nodes = useResource<DeliveryNode[] | null>(projectPath + "/delivery-nodes");
  const runHistory = useResource<Deployment[] | null>(projectPath + "/push-deployments");
  const tasks = useResource<PushTask[] | null>(projectPath + "/push-tasks");
  const hooks = useResource<Hook[] | null>(projectPath + "/release-hooks");
  const hookEvents = useResource<HookEvent[] | null>(
    query(projectPath + "/release-hook-events", { limit: 100 }),
  );
  const historyRef = useRef<HTMLDetailsElement>(null);
  const hookRef = useRef<HTMLDetailsElement>(null);
  const { confirm, toast } = useFeedback();
  const deployments = runHistory.data ?? [];
  const allNodes = nodes.data ?? [];
  const push = allNodes.filter((node) => node.delivery_mode === "push");
  const pullCount = allNodes.length - push.length;
  const hooksList = hooks.data ?? [];
  const enabledCount = hooksList.filter((hook) => hook.enabled).length;
  const action = useAction();
  const automation = useAction();
  const backupsSkipped = push.filter((node) => node.skip_backup).length;
  const backup = !push.length
    ? "按目标策略"
    : backupsSkipped
      ? `${backupsSkipped}/${push.length} 个目标跳过备份`
      : "目标发布前保留备份";

  usePolling(
    !runHistory.loading && deployments.some((row) => ["queued", "running"].includes(row.status)),
    3000,
    runHistory.refresh,
  );
  usePolling(
    !hooks.loading &&
      hooksList.some((hook) => !!hook.pending_events || hook.last_status === "running"),
    1800,
    refreshHooks,
  );

  function refreshHooks() {
    hooks.refresh();
    hookEvents.refresh();
    runHistory.refresh();
  }

  function onRun(deployment: Deployment) {
    const preview = isPreview(deployment);
    toast(
      deployment.replayed
        ? `已接续执行 #${deployment.id}`
        : preview
          ? `预演 #${deployment.id} 已记录 · ${previewNote}`
          : `部署 #${deployment.id} 已进入串行队列`,
    );
    runHistory.refresh();
    tasks.refresh();
    if (historyRef.current) historyRef.current.open = true;
  }

  function quickRun(dryRun: boolean) {
    const version = latest?.version;
    if (!version || !push.length || !canWrite) return;
    void action.run(async () => {
      if (
        !dryRun &&
        !(await confirm({
          title: `部署 ${project.name}`,
          confirmText: "确认部署",
          message: `版本：${version}（最新已发布）\nSSH 部署 ${push.length} 个目标：\n${push
            .map((node) => `• ${node.server_id}${node.host_name ? `（${node.host_name}）` : ""}`)
            .join("\n")}\n备份：${backup}\nPull 节点通过自身 Agent 收敛。`,
        }))
      )
        return;
      const deployment = await submitDeployment(
        projectPath + "/push-deployments",
        project.id + ":quick",
        {
          version,
          all_project_targets: true,
          dry_run: dryRun,
        },
      );
      onRun(deployment);
    });
  }

  return (
    <div className="section-stack" data-testid="delivery-flow-panel">
      <div className="two-col delivery-grid">
        <Card
          title="部署到服务器"
          description="核对当前项目的最新发布版本和 SSH 目标。"
          actions={
            <Badge kind={latest && push.length ? "ok" : "neutral"}>
              {latest && push.length ? "可以部署" : !latest ? "缺少已发布版本" : "缺少部署目标"}
            </Badge>
          }
        >
          <dl className="checklist">
            <div>
              <dt>项目</dt>
              <dd id="deliveryProjectName">{project.name}</dd>
            </div>
            <div>
              <dt>版本</dt>
              <dd id="deliveryVersionName">{latest?.version || "暂无已发布版本"}</dd>
            </div>
            <div>
              <dt>目标</dt>
              <dd id="deliveryTargetSummary">
                {push.length ? `SSH 部署 ${push.length} 个目标` : "尚未绑定 SSH 部署目标"}
                {pullCount > 0 && ` · 自主拉取 ${pullCount} 个节点`}
              </dd>
            </div>
            <div>
              <dt>备份</dt>
              <dd id="deliveryBackupSummary">{backup}</dd>
            </div>
          </dl>
          <ResourceNotice
            loading={nodes.loading && !push.length}
            error={nodes.error}
            retry={nodes.refresh}
          />
          {push.length ? (
            <Table headers={["server_id", "机器名称"]} testId="delivery-target-list">
              {push.map((node) => (
                <tr key={node.id}>
                  <td>
                    <code>{node.server_id}</code>
                  </td>
                  <td>{node.host_name || "—"}</td>
                </tr>
              ))}
            </Table>
          ) : (
            <p className="hint">
              <Link to="/agents">在机器与目标中绑定 SSH 目标</Link>
            </p>
          )}
          {canWrite && (
            <div className="toolbar spaced">
              <Button
                small
                data-testid="quick-deploy-preview"
                disabled={!latest || !push.length || action.busy || !!nodes.error}
                onClick={() => quickRun(true)}
              >
                预演
              </Button>
              <Button
                tone="primary"
                data-testid="quick-deploy"
                disabled={!latest || !push.length || action.busy || !!nodes.error}
                onClick={() => quickRun(false)}
              >
                部署最新版本
              </Button>
            </div>
          )}
        </Card>
        <Card
          title="自动发布"
          actions={
            <label className="switch">
              <input
                type="checkbox"
                aria-label="启用当前项目 Hook"
                id="deliveryAutomationToggle"
                checked={enabledCount > 0}
                disabled={automation.busy || !canWrite}
                onChange={(event) => {
                  const enabled = event.currentTarget.checked;
                  if (!hooksList.length) {
                    if (hookRef.current) hookRef.current.open = true;
                    toast("先创建当前项目的 Hook", "warn");
                    return;
                  }
                  void automation.run(async () => {
                    try {
                      for (const hook of hooksList.filter((row) => row.enabled !== enabled)) {
                        await request(`${projectPath}/release-hooks/${hook.id}`, {
                          method: "PUT",
                          body: hookPayload(hook, enabled),
                        });
                      }
                      toast(enabled ? "当前项目自动发布已启用" : "当前项目自动发布已停用");
                    } finally {
                      refreshHooks();
                    }
                  });
                }}
              />
              <span aria-hidden="true" />
            </label>
          }
        >
          <ResourceNotice
            loading={hooks.loading && !hooksList.length}
            error={hooks.error}
            retry={hooks.refresh}
          />
          <p className="hint">
            {hooksList.length
              ? `${enabledCount} / ${hooksList.length} 个 Hook 已启用`
              : "未配置自动发布。"}
          </p>
          <Button
            small
            onClick={() => {
              if (hookRef.current) {
                hookRef.current.open = true;
                hookRef.current.scrollIntoView({ block: "nearest" });
              }
            }}
          >
            配置 Hook
          </Button>
        </Card>
      </div>
      <DeliveryHistory
        deployments={deployments}
        loading={runHistory.loading}
        error={runHistory.error}
        refresh={runHistory.refresh}
        historyRef={historyRef}
      />
      <TaskEditor
        tasks={tasks.data ?? []}
        error={tasks.error}
        loading={tasks.loading}
        refresh={tasks.refresh}
        onRun={onRun}
      />
      <HookEditor
        hooks={hooksList}
        tasks={tasks.data ?? []}
        events={hookEvents.data ?? []}
        error={hooks.error || hookEvents.error}
        refresh={refreshHooks}
        hookRef={hookRef}
      />
    </div>
  );
}

function DeliveryHistory({
  deployments,
  loading,
  error,
  refresh,
  historyRef,
}: {
  deployments: Deployment[];
  loading: boolean;
  error?: string;
  refresh: () => void;
  historyRef: Ref<HTMLDetailsElement>;
}) {
  const { projectPath, project } = useWorkspace();
  const first = deployments[0];
  const details = useResource<Deployment>(first && `${projectPath}/push-deployments/${first.id}`);
  const deployment = details.data ?? first;
  const preview = deployment && isPreview(deployment);
  const { confirm } = useFeedback();
  const logs = useAction();
  usePolling(
    !details.loading && !!first && ["queued", "running"].includes(first.status),
    3000,
    details.refresh,
  );
  function openLogs(id: number) {
    void logs.run(async () => {
      const row = await request<Deployment>(`${projectPath}/push-deployments/${id}`);
      const dryRun = isPreview(row);
      const lines = row.targets
        ?.map(
          (target) =>
            `# ${target.host_name || "—"} / ${target.server_id} [${deploymentStatus(target.status, dryRun)}]\n${target.output || "等待执行"}`,
        )
        .join("\n\n");
      await confirm({
        title: `${dryRun ? "预演" : "部署"} #${row.id} · ${row.task_name || "当前项目"}`,
        message: (dryRun ? previewNote + "。\n\n" : "") + (lines || "没有目标"),
        onlyClose: true,
        confirmText: "关闭",
      });
    });
  }
  return (
    <>
      <Card
        title="最近一次执行"
        actions={
          <Button
            small
            onClick={() => {
              refresh();
              details.refresh();
            }}
          >
            刷新
          </Button>
        }
      >
        <ResourceNotice loading={loading && !first} error={error} retry={refresh} />
        <p className="hint" id="deliveryLatestMeta">
          {deployment
            ? `${preview ? "预演" : "部署"} #${deployment.id} · ${deployment.version} · ${dateLabel(deployment.created_at)} · ${deploymentStatus(deployment.status, Boolean(preview))}${preview ? " · " + previewNote : ""}`
            : "当前项目暂无部署记录。"}
        </p>
        <ResourceNotice
          loading={details.loading && !deployment?.targets}
          error={details.error}
          retry={details.refresh}
        />
        <Table headers={["目标", "机器", "状态", "完成时间"]} testId="delivery-latest-targets">
          {deployment?.targets?.map((target) => (
            <tr key={target.server_id}>
              <td>
                <code>
                  {project.name}|{target.server_id}
                </code>
              </td>
              <td>{target.host_name || "—"}</td>
              <td>
                <Badge
                  kind={
                    target.status === "failed"
                      ? "danger"
                      : target.status === "success"
                        ? preview
                          ? "info"
                          : "ok"
                        : "neutral"
                  }
                >
                  {deploymentStatus(target.status, Boolean(preview))}
                </Badge>
              </td>
              <td>{dateLabel(target.completed_at || target.started_at)}</td>
            </tr>
          ))}
        </Table>
      </Card>
      <Disclosure
        title="部署历史"
        id="deliveryHistoryDetails"
        ref={historyRef}
        badge={<Badge>{deployments.length} 条</Badge>}
      >
        <div id="ptab-pushlogs">
          <h3>发布日志</h3>
          <p className="hint">逐目标输出保留 30 天，发布日志不可删除。</p>
          <Table id="pushLogList" headers={["时间", "类型 / 来源", "版本", "目标", "状态", "明细"]}>
            {deployments.map((row) => {
              const dryRun = isPreview(row);
              const match = selector(row);
              return (
                <tr key={row.id}>
                  <td>{dateLabel(row.created_at)}</td>
                  <td>
                    <strong>
                      {dryRun ? "预演" : "部署"} · {row.task_name || "当前项目"}
                    </strong>
                    <small>
                      #{row.id}
                      {dryRun && " · 未连接 SSH"}
                    </small>
                  </td>
                  <td>
                    <code>{row.version}</code>
                  </td>
                  <td>
                    {match.all_project_targets
                      ? "当前项目全部目标"
                      : match.server_ids?.join(", ") || "匹配服务器"}
                  </td>
                  <td>
                    <Badge
                      kind={
                        row.status === "failed"
                          ? "danger"
                          : row.status === "success" && !dryRun
                            ? "ok"
                            : "neutral"
                      }
                    >
                      {deploymentStatus(row.status, dryRun)}
                    </Badge>
                  </td>
                  <td>
                    <Button
                      small
                      tone="ghost"
                      disabled={logs.busy}
                      data-push-log={row.id}
                      onClick={() => openLogs(row.id)}
                    >
                      {dryRun ? "查看预演明细" : "查看逐服输出"}
                    </Button>
                  </td>
                </tr>
              );
            })}
          </Table>
        </div>
      </Disclosure>
    </>
  );
}
