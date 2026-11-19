import type { FormEvent } from "react";
import { request, useResource } from "../api";
import { dateLabel, formText } from "../types";
import { Badge, Button, Card, Empty, ResourceNotice, Table, useAction, useFeedback } from "../ui";
import type { DeliveryNode } from "./delivery-api";
import { useWorkspace } from "./workspace";

export function Cluster() {
  const { projectPath, versions, canWrite } = useWorkspace();
  const resource = useResource<DeliveryNode[] | null>(projectPath + "/delivery-nodes");
  const nodes = resource.data ?? [];
  const published = versions
    .filter((version) => version.status === "published")
    .map((row) => row.version);
  const pull = nodes.filter((row) => row.delivery_mode === "pull").length;
  return (
    <div className="section-stack" data-testid="cluster-panel">
      <div className="stat-grid">
        {[
          ["SSH Push", "clusterPushCount", nodes.length - pull],
          ["Pull Agent", "clusterPullCount", pull],
          [
            "在线",
            "clusterOnlineCount",
            `${nodes.filter((row) => row.online).length}/${nodes.length}`,
          ],
          ["待收敛", "clusterDriftCount", nodes.filter((row) => row.drift).length],
        ].map(([name, id, count]) => (
          <div className="card stat" key={id}>
            <span>{name}</span>
            <strong id={String(id)}>{count}</strong>
          </div>
        ))}
      </div>
      <Card
        title="集群交付节点"
        description="查看 SSH 绑定、Pull Agent 心跳与期望版本。"
        actions={
          <Button small onClick={resource.refresh}>
            刷新
          </Button>
        }
      >
        <ResourceNotice
          loading={resource.loading && !nodes.length}
          error={resource.error}
          retry={resource.refresh}
        />
        {!nodes.length && !resource.loading ? (
          <Empty>节点在首次心跳或 SSH 绑定后出现。</Empty>
        ) : (
          <Table
            testId="delivery-node-list"
            headers={["节点", "模式", "拓扑", "版本 / Generation", "心跳", "策略"]}
          >
            {nodes.map((node) => (
              <tr key={node.id}>
                <td>
                  <strong>{node.server_id}</strong>
                  <small>
                    {node.os || "—"} / {node.arch || "—"}
                  </small>
                  <small>{node.labels?.join(" · ")}</small>
                </td>
                <td>
                  <Badge kind={node.delivery_mode === "pull" ? "info" : "neutral"}>
                    {node.delivery_mode === "pull" ? "Pull" : "Push"}
                  </Badge>
                </td>
                <td>
                  {node.role || "未标角色"}
                  <small>{node.environment || node.host_name || "—"}</small>
                </td>
                <td>
                  <code>{node.current_version || "未部署"}</code>
                  {node.delivery_mode === "pull" && (
                    <>
                      <span> → </span>
                      <code>{node.desired_version || "未设置"}</code>
                      <small>
                        generation {node.applied_generation}/{node.desired_generation} ·{" "}
                        {node.drift ? "待收敛" : "已收敛"}
                      </small>
                    </>
                  )}
                  {node.last_error && <small className="error">{node.last_error}</small>}
                </td>
                <td>
                  <Badge kind={node.online ? "ok" : "neutral"}>
                    {node.online ? "在线" : "离线"}
                  </Badge>
                  <small>{dateLabel(node.last_seen_at)}</small>
                </td>
                <td>
                  {node.delivery_mode === "pull" ? (
                    <NodePolicy
                      key={`${node.id}:${node.desired_generation}:${node.auto_follow}`}
                      node={node}
                      published={published}
                      canWrite={canWrite}
                      refresh={resource.refresh}
                    />
                  ) : (
                    <span className="hint">任务 / Hook 驱动</span>
                  )}
                </td>
              </tr>
            ))}
          </Table>
        )}
      </Card>
    </div>
  );
}

function NodePolicy({
  node,
  published,
  canWrite,
  refresh,
}: {
  node: DeliveryNode;
  published: string[];
  canWrite: boolean;
  refresh: () => void;
}) {
  const { projectPath } = useWorkspace();
  const { confirm, toast } = useFeedback();
  const action = useAction();
  const endpoint = `${projectPath}/delivery-nodes/${encodeURIComponent(node.server_id)}`;
  function save(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    const data = new FormData(event.currentTarget);
    void action.run(async () => {
      await request(endpoint + "/desired", {
        method: "PUT",
        body: { version: formText(data, "version"), auto_follow: data.has("auto") },
      });
      refresh();
      toast("节点期望状态已更新");
    });
  }
  return (
    <form className="form-stack node-policy" onSubmit={save}>
      <select
        className="input"
        aria-label={`${node.server_id} 的期望版本`}
        name="version"
        disabled={!canWrite || action.busy}
        defaultValue={node.desired_version || ""}
      >
        <option value="">选择已发布版本</option>
        {node.desired_version && !published.includes(node.desired_version) && (
          <option disabled>{node.desired_version}</option>
        )}
        {published.map((version) => (
          <option key={version}>{version}</option>
        ))}
      </select>
      <label className="check-field">
        <input
          name="auto"
          type="checkbox"
          disabled={!canWrite || action.busy}
          defaultChecked={node.auto_follow}
        />
        自动跟随新版本
      </label>
      {canWrite && (
        <div className="toolbar">
          <Button type="submit" small tone="primary" disabled={action.busy}>
            应用
          </Button>
          <Button
            small
            tone="danger"
            disabled={action.busy}
            onClick={() =>
              void action.run(async () => {
                if (
                  !(await confirm({
                    title: "移除 Pull 节点",
                    message: `移除 ${node.server_id} 的控制面状态。`,
                    danger: true,
                  }))
                )
                  return;
                await request(endpoint, { method: "DELETE" });
                refresh();
              })
            }
          >
            移除
          </Button>
        </div>
      )}
    </form>
  );
}
