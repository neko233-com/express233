import { useState } from "react";
import { Link } from "react-router";
import { query, useResource } from "../api";
import { useConsole } from "../app";
import { dateLabel, formatBytes } from "../types";
import { Badge, Button, Card, PageHeader, ResourceNotice, Table } from "../ui";

interface Day {
  date: string;
  uploads: number;
  upload_failures: number;
  upload_bytes: number;
  publishes: number;
  pulls: number;
  pull_failures: number;
  deployments: number;
  deployment_failures: number;
  target_failures: number;
}
interface Summary extends Omit<Day, "date"> {
  deployment_success_rate: number;
  pull_success_rate: number;
  average_deployment_millis: number;
}
interface Health {
  pull_nodes: number;
  pull_online: number;
  pull_drift: number;
  ssh_hosts: number;
  ssh_healthy: number;
  ssh_failing: number;
  ssh_unknown: number;
  hooks_enabled: number;
  hooks_pending: number;
  hook_failures: number;
  latest_event_at?: string;
}
interface RecordRow {
  at: string;
  kind: string;
  project_id: number;
  project: string;
  version?: string;
  status: string;
  actor?: string;
  server_id?: string;
  bytes?: number;
  detail?: string;
}
interface Snapshot {
  generated_at: string;
  summary: Summary;
  health: Health;
  series?: Day[];
  recent?: RecordRow[];
}

export function Dashboard() {
  const { projects } = useConsole();
  const [project, setProject] = useState("");
  const [days, setDays] = useState(30);
  const resource = useResource<Snapshot>(query("/api/dashboard", { days, project_id: project }));
  const data = resource.data;
  const summary = data?.summary;
  const health = data?.health;
  const series = data?.series ?? [];
  return (
    <div className="section-stack" data-testid="dashboard-panel">
      <PageHeader
        title="发布数据大盘"
        description={
          data?.generated_at
            ? `更新于 ${dateLabel(data.generated_at)}`
            : "上传、版本发布、拉取与部署活动。"
        }
      >
        <select
          className="input"
          value={project}
          onChange={(event) => setProject(event.currentTarget.value)}
          aria-label="筛选项目"
          data-testid="dashboard-project-filter"
        >
          <option value="">全部项目</option>
          {projects.map((row) => (
            <option key={row.id} value={row.id}>
              {row.name}
            </option>
          ))}
        </select>
        <select
          className="input"
          value={days}
          onChange={(event) => setDays(Number(event.currentTarget.value))}
          aria-label="时间范围"
          data-testid="dashboard-days-filter"
        >
          {[1, 7, 14, 30].map((day) => (
            <option key={day} value={day}>
              最近 {day} 天
            </option>
          ))}
        </select>
        <Button small onClick={resource.refresh}>
          刷新
        </Button>
      </PageHeader>
      <ResourceNotice
        loading={resource.loading && !data}
        error={resource.error}
        retry={resource.refresh}
      />
      <div className="stat-grid" data-testid="dashboard-kpis">
        <Stat
          label="上传请求"
          value={summary?.uploads}
          detail={
            summary &&
            `上传量 ${formatBytes(summary.upload_bytes)} · 失败 ${summary.upload_failures}`
          }
        />
        <Stat label="已发布版本" value={summary?.publishes} />
        <Stat
          label="拉取次数"
          value={summary?.pulls}
          detail={
            summary?.pulls ? `成功率 ${(summary.pull_success_rate * 100).toFixed(1)}%` : undefined
          }
        />
        <Stat
          label="SSH 部署"
          value={summary?.deployments}
          detail={
            summary?.deployments
              ? `失败 ${summary.deployment_failures} · 平均 ${(summary.average_deployment_millis / 1000).toFixed(1)} 秒`
              : undefined
          }
        />
      </div>
      <div data-testid="dashboard-health">
        <Card title="当前交付态势">
          <dl className="checklist three-col">
            <div>
              <dt>SSH 可连接</dt>
              <dd>
                {health ? `${health.ssh_healthy} / ${health.ssh_hosts}` : "—"}
                <small>{health?.ssh_failing ?? 0} 台连接失败</small>
              </dd>
            </div>
            <div>
              <dt>Pull Agent 在线</dt>
              <dd>
                {health ? `${health.pull_online} / ${health.pull_nodes}` : "—"}
                <small>{health?.pull_drift ?? 0} 个待收敛</small>
              </dd>
            </div>
            <div>
              <dt>已启用 Hook</dt>
              <dd>
                {health?.hooks_enabled ?? "—"}
                <small>{health?.hooks_pending ?? 0} 个等待派发</small>
              </dd>
            </div>
          </dl>
        </Card>
      </div>
      <div className="two-col">
        <Card title="每日发布活动">
          <Trend
            days={series}
            testId="dashboard-chart"
            metrics={[
              { label: "上传", field: "uploads", color: "var(--info)" },
              { label: "发布", field: "publishes", color: "var(--text)" },
              { label: "拉取", field: "pulls", color: "var(--success)" },
              { label: "部署", field: "deployments", color: "var(--accent-alt)" },
            ]}
          />
        </Card>
        <Card title="失败压力">
          <Trend
            days={series}
            testId="dashboard-quality-chart"
            metrics={[
              { label: "上传失败", field: "upload_failures", color: "var(--warn)" },
              { label: "拉取失败", field: "pull_failures", color: "var(--info)" },
              { label: "部署失败", field: "deployment_failures", color: "var(--danger)" },
              { label: "目标失败", field: "target_failures", color: "var(--accent-alt)" },
            ]}
          />
        </Card>
      </div>
      <Card title="每日明细">
        <Table
          testId="dashboard-daily-table"
          headers={["日期", "上传", "上传量", "发布", "拉取", "部署", "部署失败", "失败目标"]}
        >
          {series.toReversed().map((day) => (
            <tr key={day.date}>
              <td>{day.date}</td>
              <td>{day.uploads}</td>
              <td>{formatBytes(day.upload_bytes)}</td>
              <td>{day.publishes}</td>
              <td>{day.pulls}</td>
              <td>{day.deployments}</td>
              <td>{day.deployment_failures}</td>
              <td>{day.target_failures}</td>
            </tr>
          ))}
        </Table>
      </Card>
      <Card title="最近记录" description="最近 30 天内的交付事件。">
        <Table
          testId="dashboard-records-table"
          headers={["时间", "事件", "项目 / 版本", "状态", "操作者", "详情"]}
        >
          {data?.recent?.map((row, index) => (
            <tr key={`${row.at}:${index}`}>
              <td>{dateLabel(row.at)}</td>
              <td>{row.kind}</td>
              <td>
                <Link
                  to={
                    `/projects/${row.project_id}/versions` +
                    (row.version ? "?version=" + encodeURIComponent(row.version) : "")
                  }
                >
                  {row.project}
                </Link>
                <small>{row.version || row.server_id || "—"}</small>
              </td>
              <td>
                <Badge
                  kind={
                    row.status === "failed" ? "danger" : row.status === "success" ? "ok" : "neutral"
                  }
                >
                  {row.status}
                </Badge>
              </td>
              <td>{row.actor || "—"}</td>
              <td className="wrap-cell">
                {row.detail || (row.bytes ? formatBytes(row.bytes) : "—")}
              </td>
            </tr>
          ))}
        </Table>
        {!data?.recent?.length && !resource.loading && <p className="hint">暂无交付记录。</p>}
      </Card>
    </div>
  );
}

function Stat({ label, value, detail }: { label: string; value?: number; detail?: string }) {
  return (
    <div className="card stat">
      <span>{label}</span>
      <strong>{value ?? "—"}</strong>
      {detail && <small>{detail}</small>}
    </div>
  );
}

function Trend({
  days,
  metrics,
  testId,
}: {
  days: Day[];
  metrics: { label: string; field: Exclude<keyof Day, "date">; color: string }[];
  testId: string;
}) {
  const [hidden, setHidden] = useState<string[]>([]);
  const visible = metrics.filter((metric) => !hidden.includes(metric.field));
  const peak = Math.max(
    1,
    ...days.flatMap((day) => visible.map((metric) => day[metric.field] || 0)),
  );
  const x = (index: number) => (days.length < 2 ? 240 : 33 + (425 * index) / (days.length - 1));
  const y = (value: number) => 145 - (110 * value) / peak;
  return (
    <div className="trend" data-testid={testId}>
      <div className="toolbar chart-legend">
        {metrics.map(({ label, field, color }) => (
          <button
            type="button"
            className="chart-toggle"
            key={field}
            aria-pressed={!hidden.includes(field)}
            onClick={() =>
              setHidden((list) =>
                list.includes(field) ? list.filter((value) => value !== field) : [...list, field],
              )
            }
          >
            <span className="legend-dot" style={{ backgroundColor: color }} />
            {label}
          </button>
        ))}
      </div>
      <svg viewBox="0 0 480 174" role="img" aria-label="最近每日趋势">
        {[0, 0.25, 0.5, 0.75, 1].map((tick) => (
          <g key={tick}>
            <path d={`M33 ${y(tick * peak)}H458`} stroke="var(--border)" />
            <text x="25" y={y(tick * peak) + 4} textAnchor="end">
              {Math.round(tick * peak)}
            </text>
          </g>
        ))}
        {days.length > 0 && (
          <g>
            <text x={x(0)} y={165} textAnchor="start">
              {days[0].date.slice(5)}
            </text>
            {days.length > 1 && (
              <text x={x(days.length - 1)} y={165} textAnchor="end">
                {days.at(-1)?.date.slice(5)}
              </text>
            )}
          </g>
        )}
        {visible.map(({ field, color, label }) => (
          <g key={field}>
            <polyline
              points={days.map((day, index) => `${x(index)},${y(day[field] || 0)}`).join(" ")}
              stroke={color}
              strokeWidth="2"
              fill="none"
            />
            {days.map((day, index) => (
              <circle
                key={day.date}
                cx={x(index)}
                cy={y(day[field] || 0)}
                r={days.length <= 7 ? 3 : 2}
                fill={color}
              >
                <title>
                  {day.date} · {label} {day[field] || 0}
                </title>
              </circle>
            ))}
          </g>
        ))}
      </svg>
    </div>
  );
}
