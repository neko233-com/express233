import { useEffect, useState } from "react";

const tokenKey = "express233_jwt";
const expiredEvent = "express233:session-expired";

export interface Session {
  username: string;
  is_admin: boolean;
  is_root: boolean;
  role: string;
  tenant_id: number;
  tenant_slug: string;
  token?: string;
}

export class ApiError extends Error {
  constructor(
    readonly status: number,
    message: string,
  ) {
    super(message);
  }
}

export function errorMessage(error: unknown) {
  return error instanceof Error ? error.message : String(error);
}

function token() {
  try {
    return localStorage.getItem(tokenKey);
  } catch {
    return null;
  }
}

export function saveToken(next?: string) {
  try {
    if (next) localStorage.setItem(tokenKey, next);
    else localStorage.removeItem(tokenKey);
  } catch {
    /* The same-origin session cookie can still authenticate. */
  }
}

function expireSession(sentToken: string | null) {
  if (token() !== sentToken) return;
  saveToken();
  window.dispatchEvent(new Event(expiredEvent));
}

export function watchSessionExpiry(handler: () => void) {
  window.addEventListener(expiredEvent, handler);
  return () => window.removeEventListener(expiredEvent, handler);
}

interface RequestOptions {
  method?: "POST" | "PUT" | "DELETE";
  body?: unknown;
  headers?: Record<string, string>;
  signal?: AbortSignal;
  auth?: boolean;
}

function responseError(status: number, body: string, fallback: string) {
  try {
    const parsed = JSON.parse(body) as { error?: string; message?: string; errors?: string[] };
    return new ApiError(
      status,
      parsed.error || parsed.message || parsed.errors?.join("；") || fallback,
    );
  } catch {
    return new ApiError(status, fallback);
  }
}

export async function request<T = unknown>(path: string, options: RequestOptions = {}): Promise<T> {
  const { method = "GET", body, headers, signal, auth = true } = options;
  const sentToken = auth ? token() : null;
  const response = await fetch(path, {
    credentials: "same-origin",
    method,
    signal,
    headers: {
      Accept: "application/json",
      ...(body !== undefined ? { "Content-Type": "application/json" } : {}),
      ...(sentToken ? { Authorization: "Bearer " + sentToken } : {}),
      ...headers,
    },
    body: body !== undefined ? JSON.stringify(body) : undefined,
  });
  if (!response.ok) {
    if (response.status === 401 && auth) expireSession(sentToken);
    throw responseError(response.status, await response.text(), `请求失败（${response.status}）`);
  }
  return response.status === 204 ? (undefined as T) : ((await response.json()) as T);
}

export function query(path: string, params: Record<string, string | number | undefined>) {
  const values = new URLSearchParams();
  for (const [key, value] of Object.entries(params)) {
    if (value !== undefined && value !== "") values.set(key, String(value));
  }
  const encoded = values.toString();
  return encoded ? path + "?" + encoded : path;
}

type ResourceState<T> = { path: string; data?: T; error?: string; revision: number };

type ReadResource<T> = (path: string, options: RequestOptions) => Promise<T>;

export function useResource<T>(path?: string, load: ReadResource<T> = request<T>) {
  const [revision, setRevision] = useState(0);
  const [state, setState] = useState<ResourceState<T>>();
  useEffect(() => {
    if (!path) return;
    const controller = new AbortController();
    load(path, { signal: controller.signal }).then(
      (data) => {
        if (!controller.signal.aborted) setState({ path, data, revision });
      },
      (error: unknown) => {
        if (!controller.signal.aborted) setState({ path, error: errorMessage(error), revision });
      },
    );
    return () => controller.abort();
  }, [path, revision, load]);
  const current = state?.path === path ? state : undefined;
  return {
    data: current?.data,
    error: current?.revision === revision ? current?.error : undefined,
    loading: Boolean(path && current?.revision !== revision),
    refresh: () => setRevision((value) => value + 1),
  };
}

export function uploadFile(
  path: string,
  file: File,
  tags: string[],
  progress: (percent: number) => void,
) {
  return new Promise<void>((resolve, reject) => {
    const form = new FormData();
    form.append("file", file);
    form.append("path", file.webkitRelativePath || file.name);
    tags.forEach((tag) => form.append("tags", tag));
    const xhr = new XMLHttpRequest();
    xhr.open("POST", path);
    xhr.timeout = 300_000;
    const sentToken = token();
    if (sentToken) xhr.setRequestHeader("Authorization", "Bearer " + sentToken);
    xhr.upload.onprogress = (event) => {
      if (event.lengthComputable) progress(Math.round((event.loaded * 100) / event.total));
    };
    xhr.onload = () => {
      if (xhr.status >= 200 && xhr.status < 300) {
        resolve();
        return;
      }
      if (xhr.status === 401) expireSession(sentToken);
      reject(responseError(xhr.status, xhr.responseText, `上传失败（${xhr.status}）`));
    };
    xhr.onerror = () => reject(new ApiError(0, "上传网络连接失败"));
    xhr.ontimeout = () => reject(new ApiError(0, "上传超时"));
    xhr.send(form);
  });
}
