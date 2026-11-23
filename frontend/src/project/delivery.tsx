import { useEffect, useRef } from "react";
import { Link, useNavigate, useSearchParams } from "react-router";
import { useResource } from "../api";
import { useConsole } from "../app";
import { dateLabel } from "../types";
import { Badge, Button, Card, Empty, ResourceNotice, Table, useAction, useFeedback } from "../ui";
import {
  activeDeployment,
  deploymentFeedback,
  deploymentStatus,
  deploymentTone,
  isPreview,
  previewNote,
  submitDeployment,
  usePolling,
  type DeliveryNode,
  type Deployment,
} from "./delivery-api";
import { useWorkspace } from "./workspace";

export function Delivery() {
  const { project, projectPath, latest, versionsLoading, canWrite, panelLink } = useWorkspace();
  const nodes = useResource<DeliveryNode[] | null>(projectPath + "/delivery-nodes");
  const history = useResource<Deployment[] | null>(projectPath + "/push-deployments");
  const navigate = useNavigate();
  const { session } = useConsole();
  const { confirm, toast } = useFeedback();
  const action = useAction();
  const allNodes = nodes.data ?? [];
  const push = allNodes.filter((node) => node.delivery_mode === "push");
  const pullCount = allNodes.length - push.length;
  const backupsSkipped = push.filter((node) => node.skip_backup).length;
  const backup = !push.length
    ? "—"
    : backupsSkipped
      ? `${backupsSkipped}/${push.length} 个目标跳过备份`
      : "目标发布前保留备份";
  const ready = Boolean(
    latest && push.length && !nodes.loading && !nodes.error && !versionsLoading,
  );

  function quickRun(dryRun: boolean) {
    if (!ready || !latest || !canWrite) return;
    const version = latest.version;
    void action.run(async () => {
      if (
        !dryRun &&
        !(await confirm({
          title: `部署 ${project.name}`,
          confirmText: "确认部署",
          message: `版本：${version}（最新已发布）\nSSH 部署 ${push.length} 个目标：\n${push
            .map((node) => `• ${node.server_id}${node.host_name ? `（${node.host_name}）` : ""}`)
            .join("\n")}\n备份：${backup}\n执行时解析当前项目的全部 SSH 绑定，逐个部署。`,
        }))
      )
        return;
      const deployment = await submitDeployment(
        projectPath + "/push-deployments",
        project.id + ":quick",
        { version, all_project_targets: true, dry_run: dryRun },
      );
      toast(deploymentFeedback(deployment));
      history.refresh();
      await navigate(panelLink("delivery", { run: deployment.id }));
    });
  }

  return (
    <div className="section-stack" data-testid="delivery-flow-panel">
      <Card title="SSH 手动部署">
        <dl className="delivery-plan">
          <div>
            <dt>最新已发布版本</dt>
            <dd id="deliveryVersionName">
              {latest ? (
                <code>{latest.version}</code>
              ) : versionsLoading ? (
                "正在加载…"
              ) : (
                "暂无已发布版本"
              )}
            </dd>
          </div>
          <div>
            <dt>交付目标</dt>
            <dd id="deliveryTargetSummary">
              <span>
                {nodes.loading && !nodes.data
                  ? "正在加载…"
                  : nodes.error
                    ? "读取失败"
                    : `SSH 部署 ${push.length} 个目标`}
              </span>
              {pullCount > 0 && <Link to={panelLink("cluster")}>Pull 节点 {pullCount} 个</Link>}
            </dd>
          </div>
          <div>
            <dt>备份策略</dt>
            <dd id="deliveryBackupSummary">{backup}</dd>
          </div>
        </dl>
        <ResourceNotice
          loading={nodes.loading && !nodes.data}
          error={nodes.error}
          retry={nodes.refresh}
        />
        {push.length > 0 && (
          <Table headers={["server_id", "机器名称", "部署前备份"]} testId="delivery-target-list">
            {push.map((node) => (
              <tr key={node.id}>
                <td>
                  <code>{node.server_id}</code>
                </td>
                <td>{node.host_name || "—"}</td>
                <td>{node.skip_backup ? <Badge kind="warn">跳过</Badge> : "保留"}</td>
              </tr>
            ))}
          </Table>
        )}
        {!push.length && !nodes.loading && !nodes.error && (
          <p className="hint spaced">
            尚未绑定 SSH 部署目标。{session.is_admin && <Link to="/agents">前往机器与目标</Link>}
          </p>
        )}
        {!latest && !versionsLoading && (
          <p className="hint spaced">
            <Link to={panelLink("publish")}>发布版本</Link>后可执行部署。
          </p>
        )}
        {canWrite && (
          <div className="toolbar spaced">
            <Button
              data-testid="quick-deploy-preview"
              disabled={!ready || action.busy}
              onClick={() => quickRun(true)}
            >
              预演
            </Button>
            <Button
              tone="primary"
              data-testid="quick-deploy"
              disabled={!ready || action.busy}
              onClick={() => quickRun(false)}
            >
              部署最新版本
            </Button>
            {ready && <span className="hint">预演只核对目标</span>}
          </div>
        )}
      </Card>
      <DeliveryHistory history={history} />
    </div>
  );
}

function DeliveryHistory({
  history,
}: {
  history: ReturnType<typeof useResource<Deployment[] | null>>;
}) {
  const { projectPath, panelLink } = useWorkspace();
  const [search] = useSearchParams();
  const selected = search.get("run");
  const deployments = history.data ?? [];
  const id = selected || deployments[0]?.id;
  const details = useResource<Deployment>(
    id ? `${projectPath}/push-deployments/${encodeURIComponent(id)}` : undefined,
  );
  const deployment = details.data;
  const preview = Boolean(deployment && isPreview(deployment));
  const result = useRef<HTMLElement>(null);
  const { confirm } = useFeedback();
  const logs = useAction();
  const active =
    deployments.some(activeDeployment) || Boolean(deployment && activeDeployment(deployment));

  usePolling(active && !history.loading && !details.loading, 3000, refresh);

  useEffect(() => {
    if (!selected) return;
    const frame = requestAnimationFrame(() => result.current?.scrollIntoView({ block: "nearest" }));
    return () => cancelAnimationFrame(frame);
  }, [selected]);

  function refresh() {
    history.refresh();
    if (id) details.refresh();
  }

  function openLogs(row: Deployment) {
    void logs.run(async () => {
      const output = row.targets
        ?.map(
          (target) =>
            `# ${target.host_name || "—"} / ${target.server_id} [${deploymentStatus(target.status, preview)}]\n${target.output || "暂无输出"}`,
        )
        .join("\n\n");
      await confirm({
        title: `${preview ? "预演" : "部署"} #${row.id} · ${row.task_name || "当前项目"}`,
        message: (preview ? previewNote + "。\n\n" : "") + (output || "没有目标"),
        onlyClose: true,
        confirmText: "关闭",
      });
    });
  }

  return (
    <Card
      title="部署记录"
      id="deliveryHistoryDetails"
      actions={
        <Button small onClick={refresh}>
          刷新
        </Button>
      }
    >
      <div className="deployment-history">
        <div id="ptab-pushlogs">
          <ResourceNotice
            loading={history.loading && !history.data}
            error={history.error}
            retry={history.refresh}
          />
          {deployments.length > 0 ? (
            <Table id="pushLogList" headers={["执行 / 来源", "版本", "状态"]}>
              {deployments.map((row) => {
                const dryRun = isPreview(row);
                const current = String(row.id) === String(id);
                return (
                  <tr key={row.id} className={current ? "current-deployment" : undefined}>
                    <td>
                      <Link
                        to={panelLink("delivery", { run: row.id })}
                        aria-current={current ? "true" : undefined}
                      >
                        {dryRun ? "预演" : "部署"} #{row.id}
                      </Link>
                      <small>
                        {row.task_name || "当前项目"} · {dateLabel(row.created_at)}
                      </small>
                    </td>
                    <td>
                      <code>{row.version}</code>
                    </td>
                    <td>
                      <Badge kind={deploymentTone(row.status, dryRun)}>
                        {deploymentStatus(row.status, dryRun)}
                      </Badge>
                    </td>
                  </tr>
                );
              })}
            </Table>
          ) : (
            !history.loading && !history.error && <Empty>当前项目暂无部署记录。</Empty>
          )}
        </div>
        {id && (
          <section className="deployment-result" ref={result} aria-label="执行结果">
            <ResourceNotice
              loading={details.loading && !deployment}
              error={details.error}
              retry={details.refresh}
            />
            {deployment && (
              <>
                <div className="card-head">
                  <div>
                    <h3>
                      {preview ? "预演" : "部署"} #{deployment.id}
                    </h3>
                    <p className="hint" id="deliveryLatestMeta">
                      {deployment.version} · {deploymentStatus(deployment.status, preview)}
                      {preview && " · " + previewNote}
                    </p>
                  </div>
                  <Button
                    small
                    data-push-log={deployment.id}
                    disabled={logs.busy}
                    onClick={() => openLogs(deployment)}
                  >
                    {preview ? "查看预演明细" : "查看逐服输出"}
                  </Button>
                </div>
                <Table
                  headers={["server_id", "机器", "状态", "时间"]}
                  testId="delivery-latest-targets"
                >
                  {deployment.targets?.map((target) => (
                    <tr key={target.server_id}>
                      <td>
                        <code>{target.server_id}</code>
                      </td>
                      <td>{target.host_name || "—"}</td>
                      <td>
                        <Badge kind={deploymentTone(target.status, preview)}>
                          {deploymentStatus(target.status, preview)}
                        </Badge>
                      </td>
                      <td>{dateLabel(target.completed_at || target.started_at)}</td>
                    </tr>
                  ))}
                </Table>
              </>
            )}
          </section>
        )}
      </div>
    </Card>
  );
}
