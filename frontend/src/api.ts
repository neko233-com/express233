export type Project = { id: number; name: string; my_role?: string };
export type Version = { id: number; version: string; status: string; created_at: string; published_at?: string };
export type PushDeployment = { id: number; version: string; status: string; created_at: string; targets?: { id: number; host_name: string; server_id: string; status: string; output: string }[] };

const tokenKey = "express233_jwt";
export async function api<T>(path: string, init?: RequestInit): Promise<T> {
  const token = localStorage.getItem(tokenKey);
  const response = await fetch(`/api${path}`, { credentials: "include", ...init, headers: { ...(token ? { Authorization: `Bearer ${token}` } : {}), ...(init?.headers ?? {}) } });
  if (!response.ok) { const body = await response.json().catch(() => ({})); throw new Error(body.error ?? response.statusText); }
  return response.status === 204 ? undefined as T : response.json() as Promise<T>;
}
export const values = (value: string) => value.split(",").map((item) => item.trim()).filter(Boolean);
