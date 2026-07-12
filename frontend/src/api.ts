export type Project = { id: number; name: string; my_role?: string };
export type VCSProvenance = { provider?: "git" | "github" | "gitea"; repository?: string; ref?: string; commit?: string; run_url?: string; run_id?: string };
export type ArtifactManifest = { sha256?: string; file_count?: number; bytes?: number };
export type Version = { id: number; version: string; status: string; created_at: string; published_at?: string; vcs: VCSProvenance; artifact: ArtifactManifest };
export type PushDeployment = { id: number; task_id?: number; task_name?: string; version: string; selector: string; status: string; created_at: string; targets?: { id: number; host_name: string; server_id: string; status: string; output: string }[] };
export type PushDeploymentTask = { id: number; project_id: number; name: string; version?: string; server_ids: string[]; tags: string[]; tag_match: "all" | "any"; run_count: number; last_run_at?: string; created_at: string; updated_at: string };
export type PushHost = {
  id: number; name: string; address: string; port: number; username: string; auth_mode: "private_key" | "password" | "agent";
  host_key: string; host_key_fingerprint?: string; health_check_enabled: boolean; health_check_interval_seconds: number;
  last_check_at?: string; last_check_status: "unknown" | "ok" | "failed"; last_check_error?: string; last_check_latency_ms: number; next_check_at?: string;
};
export type PushHostCheck = { id: number; host_id: number; status: "ok" | "failed"; error?: string; latency_ms: number; trigger: "manual" | "scheduled"; checked_at: string };
export type PushServerBinding = { id: number; host_id: number; server_id: string; labels: string; content_tags?: string; remote_root: string; os?: string; arch?: string };
export type ReleaseHook = { id: number; project_id: number; task_id: number; task_name?: string; name: string; enabled: boolean; debounce_seconds: number; pending_version?: string; pending_source?: string; pending_since?: string; due_at?: string; pending_events: number; trigger_count: number; merge_count: number; run_count: number; last_trigger_at?: string; last_run_at?: string; last_deployment_id?: number; last_status?: string; last_error?: string };
export type ReleaseHookEvent = { id: number; hook_id: number; hook_name: string; kind: "trigger" | "dispatch"; source: string; version: string; status: string; merged_events: number; deployment_id?: number; deployment_status?: string; detail?: string; created_at: string; completed_at?: string };
export type DeliveryNode = {
  id: string; delivery_mode: "push" | "pull"; server_id: string; role?: string; environment?: string; labels: string[];
  os?: string; arch?: string; current_version?: string; desired_version?: string; desired_generation: number; applied_generation: number;
  auto_follow: boolean; status: string; last_error?: string; last_seen_at?: string; online: boolean; drift: boolean;
  host_id?: number; host_name?: string;
};
export type AgentCapability = { group: string; method: string; path: string; role: string; description: string };
export type AgentCapabilityPayload = { authentication: string[]; credential_policy: string; log_retention?: string; openapi: string; capabilities: AgentCapability[] };
export type DashboardSummary = {
  uploads: number; upload_failures: number; upload_bytes: number; uploaded_files: number; publishes: number;
  pulls: number; pull_failures: number; pull_success_rate: number;
  deployments: number; deployment_successes: number; deployment_failures: number; deployment_success_rate: number;
  targets: number; target_failures: number; average_deployment_millis: number;
};
export type DashboardDay = {
  date: string; uploads: number; upload_failures: number; upload_bytes: number; uploaded_files: number; publishes: number;
  pulls: number; pull_failures: number; deployments: number; deployment_successes: number; deployment_failures: number;
  targets: number; target_failures: number;
};
export type DashboardRecord = {
  at: string; kind: "upload" | "publish" | "pull" | "deploy"; project_id: number; project: string;
  version?: string; status: string; actor?: string; server_id?: string; bytes?: number; files?: number; detail?: string;
};
export type DashboardHealth = { pull_nodes: number; pull_online: number; pull_drift: number; ssh_hosts: number; ssh_healthy: number; ssh_failing: number; ssh_unknown: number; hooks_enabled: number; hooks_pending: number; hook_failures: number; latest_event_at?: string };
export type DashboardSnapshot = { generated_at: string; days: number; summary: DashboardSummary; health: DashboardHealth; series: DashboardDay[]; recent: DashboardRecord[] };

const tokenKey = "express233_jwt";
export async function api<T>(path: string, init?: RequestInit): Promise<T> {
  const token = localStorage.getItem(tokenKey);
  const response = await fetch(`/api${path}`, { credentials: "include", ...init, headers: { ...(token ? { Authorization: `Bearer ${token}` } : {}), ...(init?.headers ?? {}) } });
  if (!response.ok) { const body = await response.json().catch(() => ({})); throw new Error(body.error ?? response.statusText); }
  return response.status === 204 ? undefined as T : response.json() as Promise<T>;
}
export const values = (value: string) => value.split(",").map((item) => item.trim()).filter(Boolean);
