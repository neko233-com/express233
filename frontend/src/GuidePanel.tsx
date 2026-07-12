import { BookOpen, ExternalLink, ShieldCheck } from "lucide-react";
import { useCallback, useEffect, useState } from "react";
import { api } from "./api";

type GuideTopic = { id: string; title: string; summary: string; content?: string };
type GuideIndex = { title: string; notice: string; topics: GuideTopic[]; api_path: string };

export function GuidePanel() {
  const [index, setIndex] = useState<GuideIndex | null>(null); const [topic, setTopic] = useState<GuideTopic | null>(null); const [error, setError] = useState("");
  const loadTopic = useCallback(async (id: string) => { try { setTopic(await api<GuideTopic>(`/agent/guide/${encodeURIComponent(id)}`)); setError(""); } catch (reason) { setError((reason as Error).message); } }, []);
  useEffect(() => { void (async () => { try { const guide = await api<GuideIndex>("/agent/guide"); setIndex(guide); if (guide.topics[0]) await loadTopic(guide.topics[0].id); } catch (reason) { setError((reason as Error).message); } })(); }, [loadTopic]);
  return <div className="global-panel guide-panel" data-testid="guide-panel"><header className="page-header"><div><h1>文档目录</h1><p className="subtitle">控制台、Agent、Gitea 与 GitHub 的官方接入说明。</p></div><a className="btn btn-secondary btn-sm" href="/api/agent/guide" target="_blank" rel="noreferrer"><ExternalLink size={14}/>Agent HTTP 指南</a></header><div className="card guide-security-note"><ShieldCheck size={15}/>此目录只加载脱敏的官方静态说明；不包含账号、Token、SSH、服务器、项目或运行态数据。</div>{error ? <div className="agent-error">{error}</div> : null}<div className="guide-layout"><aside className="card guide-topic-list" data-testid="guide-topic-list"><div className="guide-topic-head"><strong>{index?.title || "官方接入指南"}</strong><small>{index?.notice}</small></div>{index?.topics.map((item) => <button key={item.id} className={`guide-topic ${topic?.id === item.id ? "active" : ""}`} onClick={() => void loadTopic(item.id)}><strong><BookOpen size={13}/>{item.title}</strong><small>{item.summary}</small></button>)}</aside><section className="card guide-content-card"><div className="guide-content" data-testid="guide-content">{topic ? <><div className="guide-content-head"><h2>{topic.title}</h2><p>{topic.summary}</p></div><pre>{topic.content}</pre></> : "正在加载官方指南…"}</div></section></div></div>;
}
