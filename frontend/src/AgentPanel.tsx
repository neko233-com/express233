import { Activity, BookOpen, Clock3, History, Plus, RefreshCw, Route, Server, ShieldCheck, Trash2 } from "lucide-react";
import { useCallback, useEffect, useMemo, useState } from "react";
import { api, AgentCapabilityPayload, PushHost, PushHostCheck, PushServerBinding } from "./api";

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
    <SSHResourceManager hosts={hosts} reload={load}/>
    <section className="card agent-api-card"><div className="card-title-row"><div><h3 className="card-title">Agent HTTP API</h3><p className="hint">能力来自运行中服务，请求模型以 OpenAPI 为准。</p></div><code className="api-origin">/api/agent/capabilities</code></div><div className="agent-capabilities" data-testid="agent-capabilities">{Object.entries(grouped).map(([group, items]) => <section className="api-group" key={group}><h4>{group}</h4>{items?.map((item) => <div className="api-row" key={`${item.method}-${item.path}`}><span className={`http-method method-${item.method.toLowerCase()}`}>{item.method}</span><code>{item.path}</code><span className="api-description">{item.description}</span><span className="api-role">{item.role}</span></div>)}</section>)}</div></section>
  </div>;
}

function SSHResourceManager({ hosts, reload }: { hosts: PushHost[]; reload: () => Promise<void> }) {
  const [selected, setSelected] = useState<number | null>(null); const [bindings, setBindings] = useState<PushServerBinding[]>([]);
  const [name, setName] = useState(""); const [address, setAddress] = useState(""); const [port, setPort] = useState("22"); const [username, setUsername] = useState(""); const [authMode, setAuthMode] = useState<PushHost["auth_mode"]>("private_key"); const [credential, setCredential] = useState(""); const [hostKey, setHostKey] = useState("");
  const [serverID, setServerID] = useState(""); const [labels, setLabels] = useState("test"); const [contentTags, setContentTags] = useState(""); const [remoteRoot, setRemoteRoot] = useState("/opt/game-servers");
  async function createHost() { await api("/push/hosts", { method: "POST", headers: { "Content-Type": "application/json" }, body: JSON.stringify({ name, address, port: Number(port || 22), username, auth_mode: authMode, credential, host_key: hostKey, health_check_enabled: true, health_check_interval_seconds: 3600 }) }); setName(""); setAddress(""); setUsername(""); setCredential(""); setHostKey(""); await reload(); }
  async function selectHost(id: number) { setSelected(id); setBindings((await api<PushServerBinding[] | null>(`/push/hosts/${id}/servers`)) ?? []); }
  async function removeHost(id: number) { if (!window.confirm("删除 SSH 资源及其全部 server_id 绑定？")) return; await api(`/push/hosts/${id}`, { method: "DELETE" }); if (selected === id) { setSelected(null); setBindings([]); } await reload(); }
  const [skipBackup, setSkipBackup] = useState(false);
  async function createBinding() { if (!selected) return; await api(`/push/hosts/${selected}/servers`, { method: "POST", headers: { "Content-Type": "application/json" }, body: JSON.stringify({ server_id: serverID, labels, content_tags: contentTags, remote_root: remoteRoot, skip_backup: skipBackup, os: "linux", arch: "amd64" }) }); setServerID(""); setSkipBackup(false); await selectHost(selected); }
  async function removeBinding(id: number) { if (!selected) return; await api(`/push/hosts/${selected}/servers/${id}`, { method: "DELETE" }); await selectHost(selected); }
  return <section className="card agent-resource-card"><div className="card-title-row"><div><h3 className="card-title">SSH 资源管理</h3><p className="hint">集中维护节点和 server_id 绑定；凭据加密保存，任何 API 均不会回传。</p></div><span className="security-pill"><ShieldCheck/>凭据保存后不可查看</span></div><div className="toolbar wrap"><input className="input" value={name} onChange={(event) => setName(event.target.value)} placeholder="资源名"/><input className="input" value={address} onChange={(event) => setAddress(event.target.value)} placeholder="主机 / IP"/><input className="input" type="number" value={port} onChange={(event) => setPort(event.target.value)} placeholder="端口"/><input className="input" value={username} onChange={(event) => setUsername(event.target.value)} placeholder="SSH 账号"/><select className="input" value={authMode} onChange={(event) => setAuthMode(event.target.value as PushHost["auth_mode"])}><option value="private_key">private_key</option><option value="password">password</option><option value="agent">SSH agent</option></select><input className="input" type="password" autoComplete="new-password" value={credential} onChange={(event) => setCredential(event.target.value)} placeholder="私钥或密码"/><input className="input" value={hostKey} onChange={(event) => setHostKey(event.target.value)} placeholder="Host public key"/><button className="btn btn-secondary btn-sm" onClick={() => void createHost()}><Plus size={14}/>新增 SSH 资源</button></div><div className="table-wrap" data-testid="ssh-resource-list"><table className="data-table"><thead><tr><th>资源</th><th>连接</th><th>认证</th><th>Host key</th><th>操作</th></tr></thead><tbody>{hosts.map((host) => <tr key={host.id} className={selected === host.id ? "selected-row" : ""}><td>{host.name}</td><td><code>{host.username}@{host.address}:{host.port}</code></td><td>{host.auth_mode}</td><td>{host.host_key_fingerprint || (host.host_key ? "已固定" : "待登记")}</td><td><button className="btn btn-ghost btn-sm" onClick={() => void selectHost(host.id)}>绑定</button><button className="btn btn-danger btn-sm" onClick={() => void removeHost(host.id)}><Trash2 size={13}/>删除</button></td></tr>)}</tbody></table></div>{selected ? <><div className="toolbar wrap"><input className="input" value={serverID} onChange={(event) => setServerID(event.target.value)} placeholder="server_id"/><input className="input" value={labels} onChange={(event) => setLabels(event.target.value)} placeholder="发布标签"/><input className="input" value={contentTags} onChange={(event) => setContentTags(event.target.value)} placeholder="内容标签"/><input className="input" value={remoteRoot} onChange={(event) => setRemoteRoot(event.target.value)} placeholder="远端根目录"/><label className="checkbox-label"><input type="checkbox" checked={skipBackup} onChange={(event) => setSkipBackup(event.target.checked)}/>跳过旧版备份</label><button className="btn btn-secondary btn-sm" onClick={() => void createBinding()}>绑定服务器</button></div><div className="table-wrap"><table className="data-table"><thead><tr><th>server_id</th><th>标签</th><th>内容标签</th><th>旧版备份</th><th>远端目录</th><th/></tr></thead><tbody>{bindings.map((binding) => <tr key={binding.id}><td>{binding.server_id}</td><td>{binding.labels}</td><td>{binding.content_tags || "全部"}</td><td>{binding.skip_backup ? "跳过" : "保留"}</td><td><code>{binding.remote_root}</code></td><td><button className="btn btn-danger btn-sm" onClick={() => void removeBinding(binding.id)}>删除</button></td></tr>)}</tbody></table></div></> : null}</section>;
}
