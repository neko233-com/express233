import { useState, type FormEvent } from "react";
import { Link, useBeforeUnload } from "react-router";
import { ArrowRight, Copy, Plus } from "lucide-react";
import { request, useResource } from "../api";
import { useConsole } from "../app";
import { formText, type Json, type Override, type ServerEntry } from "../types";
import {
  Badge,
  Button,
  Card,
  Disclosure,
  Empty,
  ResourceNotice,
  useAction,
  useFeedback,
} from "../ui";
import { useWorkspace } from "./workspace";

type Editor = { id: string; baseline: string; raw: string };

function makeEditor(entry: ServerEntry): Editor {
  const raw = JSON.stringify(entry.entry, null, 2);
  return { id: entry.server_id, baseline: raw, raw };
}

function serverPath(id: string) {
  return "/api/servers/" + encodeURIComponent(id);
}

function record(value: unknown): value is Record<string, Json> {
  return Boolean(value && typeof value === "object" && !Array.isArray(value));
}

function parseEntry(raw: string): Override | null {
  try {
    const entry: unknown = JSON.parse(raw);
    return record(entry) && (entry.replacements == null || record(entry.replacements))
      ? (entry as Override)
      : null;
  } catch {
    return null;
  }
}

export function ServerConfigs() {
  const { session } = useConsole();
  const { project, version, target, setTarget, open, refreshServerIDs, invalidateChecks } =
    useWorkspace();
  const inventory = useResource<{ servers: ServerEntry[] }>(
    session.is_admin ? "/api/servers" : undefined,
  );
  const [loadedEditor, setEditor] = useState<Editor>();
  const [search, setSearch] = useState("");
  const [invalid, setInvalid] = useState<Record<string, string>>({});
  const [addDirty, setAddDirty] = useState(false);
  const { confirm, prompt, toast } = useFeedback();
  const action = useAction();
  const entries = inventory.data?.servers ?? [];
  const existing = entries.find((entry) => entry.server_id === target);
  const editor = loadedEditor ?? (existing && makeEditor(existing));
  const entry = editor ? parseEntry(editor.raw) : null;
  const invalidFields = Object.keys(invalid).length > 0;
  const dirty = Boolean(editor && (editor.raw !== editor.baseline || invalidFields || addDirty));

  useBeforeUnload((event) => {
    if (dirty) {
      event.preventDefault();
      event.returnValue = "";
    }
  });

  if (!session.is_admin)
    return (
      <Card title="分服配置">
        <Empty>分服配置由系统管理员维护。</Empty>
      </Card>
    );

  async function discard() {
    return (
      !dirty ||
      (await confirm({
        title: "分服配置尚未保存",
        message: `${editor?.id} 有未保存的修改。`,
        confirmText: "放弃修改并切换",
      }))
    );
  }

  function select(row: ServerEntry) {
    setEditor(makeEditor(row));
    setInvalid({});
    setAddDirty(false);
    setTarget(row.server_id);
  }

  function write(value: Override) {
    if (editor) setEditor({ ...editor, raw: JSON.stringify(value, null, 2) });
  }

  function create(copy: boolean) {
    void action.run(async () => {
      if (!(await discard())) return;
      const name = await prompt({
        title: copy ? "复制 Server 配置" : "新建 Server 配置",
        message: "输入新的 Server ID。",
        value: copy ? `${editor?.id || ""}-copy` : "",
        confirmText: "创建",
      });
      if (name === null) return;
      const id = name.trim();
      if (!id || id.includes("|")) {
        toast("Server ID 不能为空且不能包含 |", "warn");
        return;
      }
      if (entries.some((row) => row.server_id === id)) {
        toast("Server ID 已存在", "warn");
        return;
      }
      const saved = await request<ServerEntry>(serverPath(id), {
        method: "PUT",
        body: copy
          ? parseEntry(editor?.baseline || "")
          : { replacements: {}, post_hook: "", post_hook_env: {} },
      });
      select(saved);
      inventory.refresh();
      refreshServerIDs();
      toast("分服配置已创建");
    });
  }

  const matches = entries.filter(({ server_id }) =>
    `${project.name}|${server_id}`.toLowerCase().includes(search.trim().toLowerCase()),
  );
  return (
    <div className="split-versions" data-testid="server-config-panel">
      <Card
        title="分服配置"
        description="同一租户内按 server_id 共享。"
        actions={
          <Button
            small
            tone="primary"
            disabled={action.busy}
            onClick={() => create(false)}
            aria-label="新建分服配置"
          >
            <Plus size={15} />
            新建
          </Button>
        }
      >
        <input
          className="input full"
          type="search"
          value={search}
          placeholder="搜索 Server ID…"
          aria-label="搜索 Server ID"
          onChange={(event) => setSearch(event.currentTarget.value)}
        />
        <ResourceNotice
          loading={inventory.loading && !entries.length}
          error={inventory.error}
          retry={inventory.refresh}
        />
        <div className="option-list" data-testid="server-config-list">
          {matches.map((row) => (
            <button
              key={row.server_id}
              className={`option-row ${row.server_id === editor?.id ? "selected" : ""}`}
              type="button"
              data-server-config={row.server_id}
              disabled={action.busy}
              onClick={() =>
                void action.run(async () => {
                  if (row.server_id !== editor?.id && (await discard())) select(row);
                })
              }
            >
              <strong>{row.server_id}</strong>
              <small>{Object.keys(row.entry.replacements || {}).length} 个配置文件</small>
            </button>
          ))}
          {!matches.length && !inventory.loading && (
            <p className="hint">{search ? "没有匹配配置" : "尚无分服配置"}</p>
          )}
        </div>
      </Card>
      <Card title={editor ? "配置参数" : "为不同区服设置参数"}>
        {!editor ? (
          <Empty>选择或新建 server_id，再设置配置文件的覆盖值。</Empty>
        ) : (
          <>
            <header className="card-head">
              <div>
                <h2>
                  Server <span id="serverConfigTitle">{editor.id}</span>
                </h2>
                <p className="hint">
                  <code>
                    {project.name}|{editor.id}
                  </code>{" "}
                  · 版本 {version?.version ?? "未选择"}
                </p>
              </div>
              <div className="toolbar">
                <span id="serverConfigSaveState">
                  <Badge kind={dirty ? "warn" : "neutral"}>{dirty ? "未保存" : "已保存"}</Badge>
                </span>
                <Button small disabled={action.busy} onClick={() => create(true)}>
                  <Copy size={14} />
                  复制配置
                </Button>
                <Button
                  small
                  tone="danger"
                  disabled={action.busy}
                  onClick={() =>
                    void action.run(async () => {
                      if (
                        !(await confirm({
                          title: "删除 Server 配置",
                          message: `${project.name}|${editor.id}。SSH 机器绑定需要单独维护。`,
                          danger: true,
                          confirmText: "删除",
                        }))
                      )
                        return;
                      await request(serverPath(editor.id), { method: "DELETE" });
                      setEditor(undefined);
                      setTarget("");
                      setInvalid({});
                      setAddDirty(false);
                      inventory.refresh();
                      refreshServerIDs();
                      invalidateChecks();
                      toast("分服配置已删除");
                    })
                  }
                >
                  删除
                </Button>
              </div>
            </header>
            <div className="form-stack" data-testid="server-override-form">
              {entry ? (
                <Overrides
                  key={editor.id}
                  entry={entry}
                  invalid={invalid}
                  setInvalid={setInvalid}
                  onChange={write}
                  onAddDraft={setAddDirty}
                  disabled={action.busy}
                />
              ) : (
                <p className="error" role="alert">
                  请修正完整 JSON 后保存。
                </p>
              )}
            </div>
            <Disclosure title="高级：完整 JSON 与启动钩子" id="serverConfigRaw">
              <textarea
                id="serverConfigJSON"
                className="input code-edit"
                rows={12}
                spellCheck={false}
                aria-label="Server 覆盖 JSON"
                value={editor.raw}
                disabled={action.busy}
                onChange={(event) => {
                  setInvalid({});
                  setEditor({ ...editor, raw: event.currentTarget.value });
                }}
              />
            </Disclosure>
            <div className="toolbar spaced">
              <Button
                id="btnSaveServerConfig"
                tone="primary"
                disabled={action.busy}
                onClick={() =>
                  void action.run(async () => {
                    if (!entry || invalidFields) {
                      toast("请先修正无效的配置参数", "warn");
                      return;
                    }
                    const result = await request<ServerEntry>(serverPath(editor.id), {
                      method: "PUT",
                      body: entry,
                    });
                    select(result);
                    inventory.refresh();
                    refreshServerIDs();
                    invalidateChecks();
                    open("preview");
                    toast("分服配置已保存");
                  })
                }
              >
                保存配置并进入预览 <ArrowRight size={15} />
              </Button>
              <Link className="btn btn-ghost" to="/server">
                打开 server.yaml
              </Link>
            </div>
          </>
        )}
      </Card>
    </div>
  );
}

function leaves(
  value: Record<string, Json>,
  prefix: string[] = [],
): { path: string[]; value: Json }[] {
  return Object.entries(value).flatMap(([key, child]) => {
    const path = [...prefix, key];
    return record(child) && Object.keys(child).length
      ? leaves(child, path)
      : [{ path, value: child }];
  });
}

function replaceValue(
  object: Record<string, Json>,
  path: string[],
  value: Json,
): Record<string, Json> {
  const [key, ...rest] = path;
  if (!key) return object;
  const child = object[key];
  return {
    ...object,
    [key]: rest.length && record(child) ? replaceValue(child, rest, value) : value,
  };
}

function Overrides({
  entry,
  invalid,
  setInvalid,
  onChange,
  onAddDraft,
  disabled,
}: {
  entry: Override;
  invalid: Record<string, string>;
  setInvalid: (input: Record<string, string>) => void;
  onChange: (entry: Override) => void;
  onAddDraft: (dirty: boolean) => void;
  disabled: boolean;
}) {
  const replacements = entry.replacements ?? {};
  const files = Object.entries(replacements);
  return (
    <>
      {!files.length && <p className="hint">尚无覆盖项，可添加分服参数。</p>}
      {files.map(
        ([file, values]) =>
          record(values) && (
            <fieldset className="override-group" key={file} disabled={disabled}>
              <legend>
                <code>{file}</code>
              </legend>
              {leaves(values).map(({ path, value }) => {
                const id = JSON.stringify([file, ...path]);
                const label = `${file} / ${path.join(".")}`;
                const change = (text: string) => {
                  const nextInvalid = { ...invalid };
                  if (
                    typeof value === "number" &&
                    (!text.trim() || !Number.isFinite(Number(text)))
                  ) {
                    setInvalid({ ...nextInvalid, [id]: text });
                    return;
                  }
                  delete nextInvalid[id];
                  setInvalid(nextInvalid);
                  const next =
                    typeof value === "number"
                      ? Number(text)
                      : typeof value === "boolean"
                        ? text === "true"
                        : text;
                  onChange({
                    ...entry,
                    replacements: { ...replacements, [file]: replaceValue(values, path, next) },
                  });
                };
                return (
                  <label className="override-field" key={id}>
                    <span>{path.join(".")}</span>
                    {typeof value === "boolean" ? (
                      <select
                        className="input"
                        aria-label={label}
                        value={String(value)}
                        onChange={(event) => change(event.currentTarget.value)}
                      >
                        <option value="true">开启 (true)</option>
                        <option value="false">关闭 (false)</option>
                      </select>
                    ) : (
                      <input
                        className="input"
                        aria-label={label}
                        value={
                          invalid[id] ??
                          (typeof value === "object" ? JSON.stringify(value) : String(value))
                        }
                        aria-invalid={id in invalid}
                        readOnly={typeof value === "object"}
                        inputMode={typeof value === "number" ? "decimal" : "text"}
                        onChange={(event) => change(event.currentTarget.value)}
                      />
                    )}
                  </label>
                );
              })}
            </fieldset>
          ),
      )}
      <AddParameter entry={entry} onChange={onChange} onDraft={onAddDraft} disabled={disabled} />
    </>
  );
}

function AddParameter({
  entry,
  onChange,
  onDraft,
  disabled,
}: {
  entry: Override;
  onChange: (entry: Override) => void;
  onDraft: (dirty: boolean) => void;
  disabled: boolean;
}) {
  const [error, setError] = useState("");
  function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    const form = event.currentTarget;
    const data = new FormData(form);
    const file = formText(data, "file");
    const key = formText(data, "key");
    const text = String(data.get("value") ?? "");
    const kind = formText(data, "kind");
    const replacements = entry.replacements ?? {};
    const current = Object.hasOwn(replacements, file) ? replacements[file] : {};
    if (!/^[^/\\]+\.(json|ya?ml|properties)$/i.test(file) || !key || !record(current)) {
      setError("请填写有效的配置文件名和参数名。");
      return;
    }
    if (
      leaves(current).some(({ path }) => {
        const name = path.join(".");
        return name === key || name.startsWith(key + ".") || key.startsWith(name + ".");
      })
    ) {
      setError("此参数已存在或与现有结构冲突。");
      return;
    }
    if (kind === "number" && (!text.trim() || !Number.isFinite(Number(text)))) {
      setError("请输入有效数字。");
      return;
    }
    if (kind === "boolean" && !["true", "false"].includes(text.trim())) {
      setError("开关类型需要 true 或 false。");
      return;
    }
    const value =
      kind === "number" ? Number(text) : kind === "boolean" ? text.trim() === "true" : text;
    onChange({ ...entry, replacements: { ...replacements, [file]: { ...current, [key]: value } } });
    form.reset();
    onDraft(false);
    setError("");
  }
  return (
    <form
      className="form-stack"
      onSubmit={submit}
      onChange={(event) => {
        const data = new FormData(event.currentTarget);
        onDraft(["file", "key", "value"].some((key) => Boolean(formText(data, key))));
      }}
    >
      <div className="toolbar">
        <input
          name="file"
          className="input"
          aria-label="覆盖文件名"
          placeholder="application.yaml"
          required
          disabled={disabled}
        />
        <input
          name="key"
          className="input"
          aria-label="覆盖参数名"
          placeholder="server.port"
          required
          disabled={disabled}
        />
        <input
          name="value"
          className="input"
          aria-label="覆盖参数值"
          placeholder="参数值"
          disabled={disabled}
        />
        <select name="kind" className="input" aria-label="参数值类型" disabled={disabled}>
          <option value="string">文本</option>
          <option value="number">数字</option>
          <option value="boolean">开关</option>
        </select>
        <Button small type="submit" disabled={disabled}>
          添加参数
        </Button>
      </div>
      {error && (
        <p className="error" role="status">
          {error}
        </p>
      )}
    </form>
  );
}
