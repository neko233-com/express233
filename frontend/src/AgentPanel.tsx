import { Activity, BookOpen, Clock3, History, RefreshCw, Route, Server, ShieldCheck } from "lucide-react";
import { useCallback, useEffect, useMemo, useState } from "react";
import { api, AgentCapabilityPayload, PushHost, PushHostCheck } from "./api";

const intervals = [300, 900, 1800, 3600, 21600, 86400];

function intervalLabel(seconds: number) {
  if (seconds % 86400 === 0) return `${seconds / 86400} 天/次`;
  if (seconds % 3600 === 0) return `${seconds / 3600} 小时/次`;
  return `${Math.round(seconds / 60)} 分钟/次`;
}

function Health({ status }: { status: PushHost["last_check_status"] }) {
  const label = status === "ok" ? "可连接" : status === "failed" ? "连接失败" : "待检查";
  return <span className={`health-status is-${status ?? "unknown"}`}><i/>{label}</span>;
}

export function AgentPanel() {
  const [payload, setPayload] = useState<AgentCapabilityPayload | null>(null);
  const [hosts, setHosts] = useState<PushHost[]>([]);
  const [history, setHistory] = useState<{ host: PushHost; checks: PushHostCheck[] } | null>(null);
  const [checking, setChecking] = useState<number | null>(null);
  const [error, setError] = useState("");
  const load = useCallback(async () => {
    try {
      const [capabilities, hostList] = await Promise.all([api<AgentCapabilityPayload>("/agent/capabilities"), api<PushHost[]>("/push/hosts")]);
      setPayload(capabilities); setHosts(hostList); setError("");
    } catch (reason) { setError((reason as Error).message); }
  }, []);
  useEffect(() => { void load(); }, [load]);
  const grouped = useMemo(() => (payload?.capabilities ?? []).reduce<Record<string, AgentCapabilityPayload["capabilities"]>>((result, item) => {
    (result[item.group] ??= []).push(item);
    return result;
  }, {}), [payload]);
  const healthy = hosts.filter((host) => host.last_check_status === "ok").length;
  const failed = hosts.filter((host) => host.last_check_status === "failed").length;

  async function updateHealth(host: PushHost, enabled: boolean, seconds: number) {
    await api(`/push/hosts/${host.id}`, { method: "PUT", headers: { "Content-Type": "application/json" }, body: JSON.stringify({
      name: host.name, address: host.address, port: host.port, username: host.username, auth_mode: host.auth_mode, host_key: host.host_key,
      health_check_enabled: enabled, health_check_interval_seconds: seconds,
    }) });
    await load();
  }
  async function check(host: PushHost) {
    setChecking(host.id);
    try { await api<PushHostCheck>(`/push/hosts/${host.id}/check`, { method: "POST" }); await load(); await showHistory(host); }
    finally { setChecking(null); }
  }
  async function showHistory(host: PushHost) { setHistory({ host, checks: await api<PushHostCheck[]>(`/push/hosts/${host.id}/checks?limit=50`) }); }

  const stats = [
    [Server, hosts.length, "SSH 节点", ""], [Activity, healthy, "当前可连接", "is-good"], [ShieldCheck, failed, "连接失败", failed ? "is-bad" : ""],
    [Clock3, hosts.filter((host) => host.health_check_enabled).length, "已启用定检", ""], [Route, payload?.capabilities.length ?? 0, "Agent API 操作", ""],
  ] as const;
  return <div className="global-panel agent-panel" data-testid="agent-panel">
    <header className="page-header agent-header"><div><div className="eyebrow"><span className="status-orb"/>CONTROL PLANE</div><h1>Agent 与 SSH 节点</h1><p className="subtitle">统一验证 Agent HTTP API、加密凭据、节点存活和定时检查记录</p></div><div className="toolbar compact"><a className="btn btn-secondary btn-sm" href="/docs/" target="_blank" rel="noreferrer"><BookOpen size={15}/>OpenAPI</a><button className="btn btn-primary btn-sm" onClick={() => void load()}><RefreshCw size={15}/>刷新状态</button></div></header>
    {error ? <div className="agent-error">{error}</div> : null}
    <div className="agent-summary" data-testid="agent-summary">{stats.map(([Icon, value, label, tone]) => <div className={`agent-stat ${tone}`} key={label}><Icon/><div><strong>{value}</strong><span>{label}</span></div></div>)}</div>
    <section className="card agent-host-card"><div className="card-title-row"><div><h3 className="card-title">SSH 节点存活</h3><p className="hint">定时任务串行执行；一次连接失败后不重试，等待下一周期。</p></div><span className="security-pill"><ShieldCheck/>凭据加密且不可回读</span></div><div className="table-wrap" data-testid="agent-host-list">{hosts.length ? <table className="data-table agent-host-table"><thead><tr><th>节点</th><th>连接</th><th>状态</th><th>定时检查</th><th>最近 / 下次</th><th>操作</th></tr></thead><tbody>{hosts.map((host) => <tr key={host.id}><td><strong>{host.name}</strong><small>{host.host_key_fingerprint || (host.host_key ? "Host key 已固定" : "Host key 待登记")}</small></td><td><code>{host.username}@{host.address}:{host.port}</code><small>{host.auth_mode}</small></td><td><Health status={host.last_check_status}/><small>{host.last_check_latency_ms ? `${host.last_check_latency_ms} ms` : "—"}</small></td><td><label className="switch"><input type="checkbox" checked={host.health_check_enabled} onChange={(event) => void updateHealth(host, event.target.checked, host.health_check_interval_seconds)}/><span/></label><select className="input input-sm agent-interval" value={host.health_check_interval_seconds} disabled={!host.health_check_enabled} onChange={(event) => void updateHealth(host, true, Number(event.target.value))}>{[...new Set([...intervals, host.health_check_interval_seconds])].sort((a, b) => a - b).map((seconds) => <option value={seconds} key={seconds}>{intervalLabel(seconds)}</option>)}</select></td><td><small>{host.last_check_at || "尚未检查"}</small><small className="next-check">{host.next_check_at || "未安排"}</small></td><td><button className="btn btn-primary btn-sm" disabled={checking === host.id} onClick={() => void check(host)}><Activity size={14}/>{checking === host.id ? "检查中…" : "立即检查"}</button><button className="btn btn-ghost btn-sm" onClick={() => void showHistory(host)}><History size={14}/>历史</button></td></tr>)}</tbody></table> : <div className="agent-empty">还没有 SSH 节点。进入项目的“发布到远程”创建第一台节点。</div>}</div>
      {history ? <div className="agent-history"><div className="history-header"><div><h4>{history.host.name} · 检查历史</h4><p>每条记录对应一次真实连接尝试，没有隐藏重试。</p></div><button className="btn btn-ghost btn-sm" onClick={() => setHistory(null)}>关闭</button></div><div className="history-list">{history.checks.map((item) => <div className="history-row" key={item.id}><Health status={item.status}/><time>{item.checked_at}</time><span>{item.latency_ms} ms</span><span>{item.trigger === "manual" ? "手动" : "定时"}</span><code title={item.error}>{item.error || "连接与认证正常"}</code></div>)}</div></div> : null}
    </section>
    <section className="card agent-api-card"><div className="card-title-row"><div><h3 className="card-title">Agent HTTP API</h3><p className="hint">能力来自运行中服务，请求模型以 OpenAPI 为准。</p></div><code className="api-origin">/api/agent/capabilities</code></div><div className="agent-capabilities" data-testid="agent-capabilities">{Object.entries(grouped).map(([group, items]) => <section className="api-group" key={group}><h4>{group}</h4>{items?.map((item) => <div className="api-row" key={`${item.method}-${item.path}`}><span className={`http-method method-${item.method.toLowerCase()}`}>{item.method}</span><code>{item.path}</code><span className="api-description">{item.description}</span><span className="api-role">{item.role}</span></div>)}</section>)}</div></section>
  </div>;
}
