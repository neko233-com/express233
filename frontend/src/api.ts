export type Project = { id: number; name: string; my_role?: string };
export type Version = { id: number; version: string; status: string; created_at: string; published_at?: string };
export type PushDeployment = { id: number; version: string; status: string; created_at: string; targets?: { id: number; host_name: string; server_id: string; status: string; output: string }[] };
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
export type DashboardSnapshot = { generated_at: string; days: number; summary: DashboardSummary; series: DashboardDay[]; recent: DashboardRecord[] };

const tokenKey = "express233_jwt";
export async function api<T>(path: string, init?: RequestInit): Promise<T> {
  const token = localStorage.getItem(tokenKey);
  const response = await fetch(`/api${path}`, { credentials: "include", ...init, headers: { ...(token ? { Authorization: `Bearer ${token}` } : {}), ...(init?.headers ?? {}) } });
  if (!response.ok) { const body = await response.json().catch(() => ({})); throw new Error(body.error ?? response.statusText); }
  return response.status === 204 ? undefined as T : response.json() as Promise<T>;
}
export const values = (value: string) => value.split(",").map((item) => item.trim()).filter(Boolean);
