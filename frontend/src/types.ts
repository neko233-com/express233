export interface Project {
  id: number;
  name: string;
  max_published_versions: number;
  my_role?: string;
}

export interface Version {
  id: number;
  version: string;
  status: string;
  created_at: string;
  published_at?: string;
  artifact?: { sha256?: string; file_count?: number; bytes?: number };
  vcs?: { provider?: string; repository?: string; ref?: string; commit?: string; run_url?: string };
}

export interface VersionFile {
  path: string;
  tags?: string[];
}

export interface Validation {
  ok: boolean;
  errors?: string[];
  warnings?: string[];
}

export type Json = string | number | boolean | null | Json[] | { [key: string]: Json };

export interface Override {
  replacements?: Record<string, Record<string, Json>>;
  tags?: string[];
  [key: string]: Json | undefined;
}

export interface ServerEntry {
  server_id: string;
  entry: Override;
}

export function versionPath(projectId: number, version: string) {
  return `/api/projects/${projectId}/versions/${encodeURIComponent(version)}`;
}

export function csv(value: string) {
  return Array.from(
    new Set(
      value
        .split(/[\s,，]+/)
        .map((part) => part.trim())
        .filter(Boolean),
    ),
  );
}

export function suggestedVersion(versions: Version[]) {
  const latest =
    versions
      .map((row) => row.version)
      .filter((version) => /^\d+\.\d+\.\d+$/.test(version))
      .sort((a, b) => b.localeCompare(a, "en", { numeric: true }))[0] || "0.0.0";
  const parts = latest.split(".").map(Number);
  return [parts[0], parts[1], parts[2] + 1].join(".");
}

export function formatBytes(bytes = 0) {
  if (bytes < 1024) return `${bytes} B`;
  let amount = bytes;
  for (const unit of ["KiB", "MiB", "GiB", "TiB"]) {
    amount /= 1024;
    if (amount < 1024 || unit === "TiB") return `${amount.toFixed(amount < 10 ? 1 : 0)} ${unit}`;
  }
  return `${bytes} B`;
}

export function dateLabel(value?: string) {
  if (!value) return "—";
  const date = new Date(value);
  return Number.isNaN(date.valueOf()) ? value : date.toLocaleString("zh-CN", { hour12: false });
}

export function formText(data: FormData, key: string) {
  return String(data.get(key) ?? "").trim();
}
