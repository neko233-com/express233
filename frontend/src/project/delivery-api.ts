import { useEffect, useEffectEvent } from "react";
import { ApiError, errorMessage, request } from "../api";

export interface DeliveryNode {
  id: string;
  delivery_mode: "push" | "pull";
  server_id: string;
  role?: string;
  environment?: string;
  labels?: string[];
  os?: string;
  arch?: string;
  current_version?: string;
  desired_version?: string;
  desired_generation: number;
  applied_generation: number;
  auto_follow: boolean;
  status: string;
  last_error?: string;
  last_seen_at?: string;
  online: boolean;
  drift: boolean;
  host_name?: string;
  skip_backup?: boolean;
}

export interface PushTask {
  id: number;
  name: string;
  version?: string;
  server_ids?: string[];
  tags?: string[];
  tag_match: string;
  run_count?: number;
  last_run_at?: string;
}

export interface Hook {
  id: number;
  name: string;
  task_id: number;
  task_name?: string;
  debounce_seconds: number;
  enabled: boolean;
  pending_events?: number;
  due_at?: string;
  trigger_count?: number;
  merge_count?: number;
  run_count?: number;
  last_status?: string;
  last_error?: string;
}

export interface HookEvent {
  id: number;
  created_at: string;
  hook_name: string;
  kind: string;
  source?: string;
  version?: string;
  status: string;
  deployment_status?: string;
  deployment_id?: number;
  merged_events?: number;
}

interface Selector {
  all_project_targets?: boolean;
  dry_run?: boolean;
  server_ids?: string[];
  tags?: string[];
}
interface PushTarget {
  server_id: string;
  host_name?: string;
  status: string;
  completed_at?: string;
  started_at?: string;
  output?: string;
}

export interface Deployment {
  id: number;
  version: string;
  created_at: string;
  status: string;
  selector?: string;
  task_name?: string;
  dry_run?: boolean;
  replayed?: boolean;
  targets?: PushTarget[];
}

export const previewNote = "仅核对目标，未连接 SSH，未变更服务器";

export function deploymentFeedback(deployment: Deployment) {
  if (deployment.replayed) return `已接续执行 #${deployment.id}`;
  return isPreview(deployment)
    ? `预演 #${deployment.id} 已记录 · ${previewNote}`
    : `部署 #${deployment.id} 已进入串行队列`;
}

export function usePolling(active: boolean, wait: number, refresh: () => void) {
  const tick = useEffectEvent(refresh);
  useEffect(() => {
    if (!active) return;
    const id = window.setInterval(tick, wait);
    return () => window.clearInterval(id);
  }, [active, wait]);
}

export function taskScope(task: PushTask) {
  return task.server_ids?.length ? "固定目标：" + task.server_ids.join(", ") : "跟随项目当前目标";
}

export function selector(deployment: Deployment): Selector {
  try {
    return JSON.parse(deployment.selector || "{}") ?? {};
  } catch {
    return {};
  }
}

export function isPreview(deployment: Deployment) {
  return deployment.dry_run === true || selector(deployment).dry_run === true;
}

export function deploymentStatus(status: string, dryRun: boolean) {
  const labels: Record<string, string> = dryRun
    ? { queued: "等待预演", running: "预演中", success: "预演通过", failed: "预演失败" }
    : { queued: "等待串行执行", running: "部署中", success: "部署成功", failed: "部署失败" };
  if (status === "skipped") return "未执行";
  if (status === "unknown") return "状态未知，需核验";
  return labels[status] ?? status;
}

export function deploymentTone(status: string, dryRun: boolean) {
  if (status === "failed") return "danger";
  if (status === "unknown" || status === "skipped") return "warn";
  if (status === "success") return dryRun ? "info" : "ok";
  return "neutral";
}

export function activeDeployment(deployment: Deployment) {
  return deployment.status === "queued" || deployment.status === "running";
}

export async function submitDeployment(
  path: string,
  namespace: string,
  body: { dry_run: boolean; version?: string; all_project_targets?: boolean },
) {
  const key = "express233:push:" + namespace + ":" + (body.dry_run ? "preview" : "release");
  const fresh =
    globalThis.crypto?.randomUUID?.() ?? `${Date.now()}-${Math.random().toString(16).slice(2)}`;
  let idempotencyKey = "ui:" + fresh;
  try {
    idempotencyKey = sessionStorage.getItem(key) || idempotencyKey;
    sessionStorage.setItem(key, idempotencyKey);
  } catch {
    /* In-memory request remains protected by the active action. */
  }
  function clear() {
    try {
      sessionStorage.removeItem(key);
    } catch {
      /* Storage can be unavailable. */
    }
  }
  try {
    const deployment = await request<Deployment>(path, {
      method: "POST",
      body: { ...body, idempotency_key: idempotencyKey },
      headers: { "Idempotency-Key": idempotencyKey },
    });
    clear();
    return deployment;
  } catch (error) {
    if (error instanceof ApiError && error.status < 500 && error.status !== 408) {
      // A validation or authorization rejection did not create an execution.
      // Conflict keeps the key so it cannot accidentally bypass an active run.
      if (error.status !== 409) clear();
      throw error;
    }
    throw new Error(errorMessage(error) + "。重试将使用同一请求编号。", { cause: error });
  }
}
