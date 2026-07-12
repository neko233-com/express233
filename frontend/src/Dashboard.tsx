import { useEffect, useMemo, useRef, useState } from "react";
import { api, DashboardDay, DashboardHealth, DashboardRecord, DashboardSnapshot, Project } from "./api";

const number = new Intl.NumberFormat("zh-CN");
const kindLabels: Record<DashboardRecord["kind"], string> = { upload: "上传", publish: "发布", pull: "拉取", deploy: "SSH 发布" };

function formatBytes(value = 0) {
  if (value < 1024) return `${value} B`;
  if (value < 1024 ** 2) return `${(value / 1024).toFixed(1)} KB`;
  if (value < 1024 ** 3) return `${(value / 1024 ** 2).toFixed(1)} MB`;
  if (value < 1024 ** 4) return `${(value / 1024 ** 3).toFixed(1)} GB`;
  return `${(value / 1024 ** 4).toFixed(1)} TB`;
}

function formatRate(value: number, completed: number) { return completed ? `${value.toFixed(1)}%` : "—"; }
function formatDuration(value = 0) {
  if (!value) return "—";
  if (value < 1000) return `${value} ms`;
  if (value < 60000) return `${(value / 1000).toFixed(1)} s`;
  return `${(value / 60000).toFixed(1)} min`;
}

function StatusBadge({ status }: { status: string }) {
  const normalized = status.toLowerCase();
  const className = ["success", "ok"].includes(normalized) ? "ok" : ["failed", "error"].includes(normalized) ? "warn" : "draft";
  const label: Record<string, string> = { success: "成功", ok: "成功", failed: "失败", error: "失败", running: "执行中", queued: "排队中" };
  return <span className={`badge badge-${className}`}>{label[normalized] ?? normalized}</span>;
}

type ChartKind = "activity" | "failures";
function DashboardChart({ series, kind, testID, label }: { series: DashboardDay[]; kind: ChartKind; testID: string; label: string }) {
  const hostRef = useRef<HTMLDivElement>(null);
  const option = useMemo(() => {
    const configs = kind === "activity" ? [
      ["上传", "uploads", "#10b981"], ["拉取", "pulls", "#3b82f6"], ["SSH 发布", "deployments", "#f59e0b"],
    ] : [["拉取失败", "pull_failures", "#ef4444"], ["SSH 发布失败", "deployment_failures", "#f59e0b"], ["目标失败", "target_failures", "#a855f7"]];
    return { animationDuration: 240, color: configs.map((item) => item[2]), tooltip: { trigger: "axis" }, legend: { top: 2, right: 4, textStyle: { color: "#9ca3af", fontSize: 11 } }, grid: { top: 40, right: 18, bottom: 30, left: 42 }, xAxis: { type: "category", boundaryGap: kind === "failures", data: series.map((day) => day.date.slice(5)), axisLine: { lineStyle: { color: "#303844" } }, axisLabel: { color: "#8792a2", fontSize: 10 } }, yAxis: { type: "value", minInterval: 1, splitLine: { lineStyle: { color: "#262d37" } }, axisLabel: { color: "#8792a2", fontSize: 10 } }, series: configs.map(([name, key]) => ({ name, type: kind === "activity" ? "line" : "bar", stack: kind === "failures" ? "failures" : undefined, smooth: kind === "activity", symbol: "circle", symbolSize: 5, data: series.map((day) => day[key as keyof DashboardDay] as number) })) };
  }, [kind, series]);
  useEffect(() => {
    let disposed = false; let cleanup = () => {};
    void import("./dashboardEcharts").then(({ echarts }) => {
      if (disposed || !hostRef.current) return;
      const chart = echarts.init(hostRef.current, undefined, { renderer: "svg" });
      chart.setOption(option);
      const observer = new ResizeObserver(() => chart.resize()); observer.observe(hostRef.current);
      cleanup = () => { observer.disconnect(); chart.dispose(); };
    });
    return () => { disposed = true; cleanup(); };
  }, [option]);
  return <div ref={hostRef} className="dashboard-chart" data-testid={testID} role="img" aria-label={label}/>;
}

function DashboardHealthPanel({ health, generatedAt }: { health: DashboardHealth | undefined; generatedAt?: string }) {
  const value = health ?? { pull_nodes: 0, pull_online: 0, pull_drift: 0, ssh_hosts: 0, ssh_healthy: 0, ssh_failing: 0, ssh_unknown: 0, hooks_enabled: 0, hooks_pending: 0, hook_failures: 0 };
  const items = [["Pull 在线", `${value.pull_online}/${value.pull_nodes}`, value.pull_drift ? `${value.pull_drift} 个节点待收敛` : "无配置漂移", value.pull_drift ? "warn" : "ok"], ["SSH 存活（租户）", `${value.ssh_healthy}/${value.ssh_hosts}`, value.ssh_failing ? `${value.ssh_failing} 台检测失败` : `${value.ssh_unknown} 台待首次检测`, value.ssh_failing ? "warn" : "info"], ["自动 Hook", number.format(value.hooks_enabled), value.hooks_pending ? `${value.hooks_pending} 个等待防抖` : "无等待任务", value.hooks_pending ? "warn" : "primary"], ["Hook 失败", number.format(value.hook_failures), value.latest_event_at ? `最近事件 ${value.latest_event_at}` : "当前筛选暂无事件", value.hook_failures ? "warn" : "ok"]] as const;
  return <section className="card dashboard-health-card" data-testid="dashboard-health"><div className="card-title-row"><div><h3 className="card-title">当前交付态势</h3><p className="hint">实时拓扑快照与按日历史趋势分开计算，便于直接判断是否需要介入。</p></div><span className="badge badge-draft">{generatedAt ? `刷新于 ${new Date(generatedAt).toLocaleString("zh-CN", { hour12: false })}` : "等待数据"}</span></div><div className="dashboard-health-grid">{items.map(([label, metric, meta, tone]) => <article className={`dashboard-health ${tone}`} key={label}><span>{label}</span><strong>{metric}</strong><small>{meta}</small></article>)}</div></section>;
}

export function Dashboard({ projects }: { projects: Project[] }) {
  const [days, setDays] = useState("30");
  const [projectID, setProjectID] = useState("");
  const [refresh, setRefresh] = useState(0);
  const [data, setData] = useState<DashboardSnapshot | null>(null);
  const [error, setError] = useState("");
  const [loading, setLoading] = useState(true);
  useEffect(() => {
    const controller = new AbortController();
    const load = async () => {
      const query = new URLSearchParams({ days });
      if (projectID) query.set("project_id", projectID);
      setLoading(true);
      try {
        const snapshot = await api<DashboardSnapshot>(`/dashboard?${query}`, { signal: controller.signal });
        setData(snapshot); setError("");
      } catch (reason) {
        if (!controller.signal.aborted) setError((reason as Error).message);
      } finally {
        if (!controller.signal.aborted) setLoading(false);
      }
    };
    void load();
    const timer = window.setInterval(load, 60000);
    return () => { controller.abort(); window.clearInterval(timer); };
  }, [days, projectID, refresh]);

  const summary = data?.summary;
  const completedDeploys = (summary?.deployment_successes ?? 0) + (summary?.deployment_failures ?? 0);
  const kpis = [
    ["上传请求", number.format(summary?.uploads ?? 0), `${summary?.upload_failures ?? 0} 次失败`, summary?.upload_failures ? "warn" : "ok"],
    ["上传数据", formatBytes(summary?.upload_bytes), `${summary?.uploaded_files ?? 0} 个文件`, "info"],
    ["发布版本", number.format(summary?.publishes ?? 0), "已发布版本事件", "primary"],
    ["节点拉取", number.format(summary?.pulls ?? 0), `成功率 ${formatRate(summary?.pull_success_rate ?? 0, summary?.pulls ?? 0)}`, summary?.pull_failures ? "warn" : "ok"],
    ["SSH 发布", number.format(summary?.deployments ?? 0), `成功率 ${formatRate(summary?.deployment_success_rate ?? 0, completedDeploys)}`, summary?.deployment_failures ? "warn" : "ok"],
    ["发布目标", number.format(summary?.targets ?? 0), `平均耗时 ${formatDuration(summary?.average_deployment_millis)}`, summary?.target_failures ? "warn" : "info"],
  ];

  return <div id="globalDashboard" className="global-panel dashboard-panel" data-testid="dashboard-panel">
    <header className="page-header dashboard-header"><div><h1>发布数据大盘</h1><p className="subtitle">上传、发布、节点拉取与 SSH 推送的按日运营视图</p></div><div className="dashboard-filters">
      <select className="input input-sm" value={projectID} onChange={(event) => setProjectID(event.target.value)} data-testid="dashboard-project-filter"><option value="">全部可访问项目</option>{projects.map((project) => <option value={project.id} key={project.id}>{project.name}</option>)}</select>
      <select className="input input-sm" value={days} onChange={(event) => setDays(event.target.value)} data-testid="dashboard-days-filter"><option value="7">最近 7 天</option><option value="30">最近 30 天</option></select>
      <button className="btn btn-secondary btn-sm" onClick={() => setRefresh((value) => value + 1)}>刷新</button>
    </div></header>
    {error ? <div className="toast err">加载数据大盘失败：{error}</div> : null}
    <div className={`dashboard-kpi-grid ${loading ? "loading" : ""}`} data-testid="dashboard-kpis">{kpis.map(([label, value, meta, tone]) => <article className={`dashboard-kpi ${tone}`} key={label}><span className="dashboard-kpi-label">{label}</span><strong className="dashboard-kpi-value">{value}</strong><span className="dashboard-kpi-meta">{meta}</span></article>)}</div>
    <DashboardHealthPanel health={data?.health} generatedAt={data?.generated_at}/>
    <div className="dashboard-chart-grid"><section className="card dashboard-chart-card"><div className="card-title-row"><div><h3 className="card-title">每日发布活动</h3><p className="hint">同一自然日内的上传、拉取和 SSH 发布次数</p></div></div><DashboardChart series={data?.series ?? []} kind="activity" testID="dashboard-chart" label="每日发布活动趋势图"/></section><section className="card dashboard-chart-card"><div className="card-title-row"><div><h3 className="card-title">失败压力</h3><p className="hint">拉取、SSH 发布与目标失败按日堆叠。</p></div></div><DashboardChart series={data?.series ?? []} kind="failures" testID="dashboard-quality-chart" label="每日交付失败趋势图"/></section></div>
    <section className="card dashboard-daily-card"><div className="card-title-row"><div><h3 className="card-title">每日明细</h3><p className="hint">{data ? `统计周期 ${data.days} 天 · 更新于 ${new Date(data.generated_at).toLocaleString("zh-CN", { hour12: false })}` : "等待加载"}</p></div></div><div className="table-wrap dashboard-table-wrap"><table className="data-table" data-testid="dashboard-daily-table"><thead><tr><th>日期</th><th>上传</th><th>上传流量</th><th>发布版本</th><th>拉取</th><th>拉取失败</th><th>SSH 发布</th><th>目标节点</th><th>发布失败</th></tr></thead><tbody>{[...(data?.series ?? [])].reverse().map((day) => <tr key={day.date}><td><code>{day.date}</code></td><td>{day.uploads}{day.upload_failures ? <span className="metric-failure"> -{day.upload_failures}</span> : null}</td><td>{formatBytes(day.upload_bytes)}</td><td>{day.publishes}</td><td>{day.pulls}</td><td className={day.pull_failures ? "metric-failure" : ""}>{day.pull_failures}</td><td>{day.deployments}</td><td>{day.targets}</td><td className={day.deployment_failures + day.target_failures ? "metric-failure" : ""}>{day.deployment_failures + day.target_failures}</td></tr>)}</tbody></table></div></section>
    <section className="card dashboard-records-card"><div className="card-title-row"><div><h3 className="card-title">最近记录</h3><p className="hint">最多展示筛选范围内最近 100 条真实操作记录</p></div></div><div className="table-wrap dashboard-table-wrap"><table className="data-table" data-testid="dashboard-records-table"><thead><tr><th>时间</th><th>类型</th><th>项目 / 版本</th><th>操作者 / 节点</th><th>数据量</th><th>状态</th><th>详情</th></tr></thead><tbody>{data?.recent.length ? data.recent.map((record, index) => <tr key={`${record.kind}-${record.at}-${index}`}><td className="dashboard-record-time">{record.at}</td><td><span className={`record-kind ${record.kind}`}>{kindLabels[record.kind]}</span></td><td><strong>{record.project}</strong><span className="dashboard-record-sub">{record.version || "—"}</span></td><td>{[record.actor, record.server_id].filter(Boolean).join(" / ") || "—"}</td><td>{record.kind === "upload" ? `${formatBytes(record.bytes)} · ${record.files ?? 0} 文件` : record.kind === "deploy" ? `${record.files ?? 0} 目标` : "—"}</td><td><StatusBadge status={record.status}/></td><td className="dashboard-record-detail" title={record.detail}>{record.detail || "—"}</td></tr>) : <tr><td colSpan={7} className="table-empty">当前筛选范围暂无记录</td></tr>}</tbody></table></div></section>
  </div>;
}
