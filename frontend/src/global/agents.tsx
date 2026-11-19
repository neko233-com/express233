import { useState, type FormEvent } from "react";
import { Plus, Search } from "lucide-react";
import { request, useResource } from "../api";
import { useConsole } from "../app";
import { dateLabel, formText } from "../types";
import {
  Badge,
  Button,
  Card,
  Empty,
  Field,
  PageHeader,
  ResourceNotice,
  Table,
  useAction,
  useFeedback,
} from "../ui";

interface Host {
  id: number;
  name: string;
  address: string;
  port: number;
  username: string;
  auth_mode: string;
  host_key?: string;
  host_key_fingerprint?: string;
  health_check_enabled: boolean;
  health_check_interval_seconds: number;
  last_check_at?: string;
  last_check_status?: string;
  last_check_latency_ms?: number;
  next_check_at?: string;
}
interface Binding {
  id: number;
  project_name?: string;
  server_id: string;
  target_tag?: string;
  remote_root: string;
  labels: string;
  content_tags?: string;
  os?: string;
  arch?: string;
  skip_backup?: boolean;
}
interface HostCheck {
  id: number;
  status: string;
  checked_at: string;
  latency_ms: number;
  trigger: string;
  error?: string;
}
interface Capability {
  group: string;
  method: string;
  path: string;
  description: string;
  role: string;
}
type InventoryHost = Host & { bindings: Binding[]; bindingError: boolean };
const hostsPath = "/api/push/hosts";
const checksPath = (id: number) => `${hostsPath}/${id}/checks?limit=50`;
const bindingPath = (id: number) => `${hostsPath}/${id}/servers`;

async function loadInventory(
  path: string,
  options: { signal?: AbortSignal },
): Promise<InventoryHost[]> {
  const hosts = (await request<Host[] | null>(path, options)) ?? [];
  const result: InventoryHost[] = [];
  for (let start = 0; start < hosts.length; start += 8) {
    options.signal?.throwIfAborted();
    const batch = await Promise.all(
      hosts.slice(start, start + 8).map(async (host) => {
        try {
          return {
            ...host,
            bindings: (await request<Binding[] | null>(bindingPath(host.id), options)) ?? [],
            bindingError: false,
          };
        } catch (error) {
          if (options.signal?.aborted) throw error;
          return { ...host, bindings: [], bindingError: true };
        }
      }),
    );
    result.push(...batch);
  }
  return result;
}

export function Agents() {
  const { session } = useConsole();
  return session.is_admin ? <AgentDirectory /> : <Empty>机器与目标由系统管理员维护。</Empty>;
}

function AgentDirectory() {
  const { projects } = useConsole();
  const inventory = useResource(hostsPath, loadInventory);
  const capabilities = useResource<{ capabilities?: Capability[] }>("/api/agent/capabilities");
  const hosts = inventory.data ?? [];
  const operations = capabilities.data?.capabilities ?? [];
  const [text, setText] = useState("");
  const [status, setStatus] = useState("");
  const [project, setProject] = useState("");
  const [hostEditor, setHostEditor] = useState<Host | "new">();
  const [bindingFor, setBindingFor] = useState<Host>();
  const [history, setHistory] = useState<Host>();
  const checks = useResource<HostCheck[] | null>(history && checksPath(history.id));
  const action = useAction();
  const { confirm, toast } = useFeedback();
  const needle = text.trim().toLowerCase();
  const visible = hosts.filter((host) => {
    const tags = host.bindings.map(
      (binding) => binding.target_tag || `${binding.project_name}|${binding.server_id}`,
    );
    return (
      (!needle || [host.name, host.address, ...tags].join(" ").toLowerCase().includes(needle)) &&
      (!status || (host.last_check_status || "unknown") === status) &&
      (!project || host.bindings.some((binding) => binding.project_name === project))
    );
  });
  const summary = [
    ["SSH 节点", hosts.length],
    ["连接正常", hosts.filter((host) => host.last_check_status === "ok").length],
    ["连接失败", hosts.filter((host) => host.last_check_status === "failed").length],
    ["已启用定检", hosts.filter((host) => host.health_check_enabled).length],
    ["Agent API 操作", operations.length],
  ] as const;

  return (
    <div className="section-stack" data-testid="agent-panel">
      <PageHeader title="机器与目标" description="管理 SSH 机器、交付目标和连接检查。">
        <Button
          small
          onClick={() => {
            inventory.refresh();
            capabilities.refresh();
            if (history) checks.refresh();
          }}
        >
          刷新
        </Button>
        <Button tone="primary" onClick={() => setHostEditor("new")}>
          <Plus size={16} aria-hidden="true" />
          新增机器
        </Button>
      </PageHeader>
      <div className="stat-grid" data-testid="agent-summary">
        {summary.map(([label, value]) => (
          <div className="card stat" key={label}>
            <span>{label}</span>
            <strong>{value}</strong>
          </div>
        ))}
      </div>
      {hostEditor && (
        <HostEditor
          key={hostEditor === "new" ? "new" : hostEditor.id}
          host={hostEditor === "new" ? undefined : hostEditor}
          close={() => setHostEditor(undefined)}
          onSave={() => {
            setHostEditor(undefined);
            inventory.refresh();
          }}
        />
      )}
      <Card title="SSH 机器">
        <div className="toolbar spaced">
          <div className="input-icon">
            <Search size={15} aria-hidden="true" />
            <input
              className="input"
              type="search"
              value={text}
              placeholder="搜索名称、地址或目标…"
              aria-label="搜索 SSH 机器"
              onChange={(event) => setText(event.currentTarget.value)}
            />
          </div>
          <select
            className="input"
            value={status}
            aria-label="连接状态"
            onChange={(event) => setStatus(event.currentTarget.value)}
          >
            <option value="">全部状态</option>
            <option value="ok">可连接</option>
            <option value="failed">连接失败</option>
            <option value="unknown">待检查</option>
          </select>
          <select
            className="input"
            value={project}
            aria-label="所属项目"
            onChange={(event) => setProject(event.currentTarget.value)}
          >
            <option value="">全部项目</option>
            {projects.map((row) => (
              <option key={row.id} value={row.name}>
                {row.name}
              </option>
            ))}
          </select>
        </div>
        <ResourceNotice
          loading={inventory.loading && !hosts.length}
          error={inventory.error}
          retry={inventory.refresh}
        />
        <div data-testid="agent-host-list">
          <Table
            testId="ssh-resource-list"
            headers={["机器", "SSH 地址", "状态", "目标标签", "最近检查", "操作"]}
          >
            {visible.map((host) => (
              <tr key={host.id}>
                <td>
                  <strong>{host.name}</strong>
                  <small>
                    {host.host_key_fingerprint ||
                      (host.host_key ? "Host key 已固定" : "Host key 待登记")}
                  </small>
                </td>
                <td>
                  <code>
                    {host.username}@{host.address}:{host.port}
                  </code>
                  <small>{host.auth_mode}</small>
                </td>
                <td>
                  <Badge
                    kind={
                      host.last_check_status === "ok"
                        ? "ok"
                        : host.last_check_status === "failed"
                          ? "danger"
                          : "neutral"
                    }
                  >
                    {host.last_check_status === "ok"
                      ? "可连接"
                      : host.last_check_status === "failed"
                        ? "连接失败"
                        : "待检查"}
                  </Badge>
                </td>
                <td>
                  {host.bindingError ? (
                    <span className="warn-text">读取目标失败</span>
                  ) : host.bindings.length ? (
                    host.bindings.map((binding) => (
                      <small key={binding.id}>
                        <code>
                          {binding.target_tag ||
                            `${binding.project_name || "未归属"}|${binding.server_id}`}
                        </code>
                      </small>
                    ))
                  ) : (
                    <span className="hint">未绑定目标</span>
                  )}
                </td>
                <td>
                  {dateLabel(host.last_check_at)}
                  <small>
                    {host.health_check_enabled
                      ? `定检 · ${intervalLabel(host.health_check_interval_seconds)}`
                      : "定检停用"}
                  </small>
                </td>
                <td>
                  <div className="toolbar">
                    <Button
                      small
                      tone="primary"
                      disabled={action.busy}
                      onClick={() =>
                        void action.run(async () => {
                          const result = await request<HostCheck>(`${hostsPath}/${host.id}/check`, {
                            method: "POST",
                          });
                          setHistory(host);
                          checks.refresh();
                          inventory.refresh();
                          toast(
                            result.status === "ok"
                              ? `SSH 可连接 · ${result.latency_ms} ms`
                              : "SSH 连接失败，详情见检查历史",
                            result.status === "ok" ? "ok" : "error",
                          );
                        })
                      }
                    >
                      立即检查
                    </Button>
                    <Button
                      small
                      tone="ghost"
                      onClick={() => {
                        setHistory(host);
                        checks.refresh();
                      }}
                    >
                      历史
                    </Button>
                    <Button small tone="ghost" onClick={() => setBindingFor(host)}>
                      目标绑定
                    </Button>
                    <Button small tone="ghost" onClick={() => setHostEditor(host)}>
                      编辑
                    </Button>
                    <Button
                      small
                      tone="danger"
                      disabled={action.busy}
                      onClick={() =>
                        void action.run(async () => {
                          if (
                            !(await confirm({
                              title: "删除 SSH 机器",
                              message: `删除 ${host.name} 及其目标绑定。`,
                              danger: true,
                              confirmText: "删除",
                            }))
                          )
                            return;
                          await request(`${hostsPath}/${host.id}`, { method: "DELETE" });
                          if (bindingFor?.id === host.id) setBindingFor(undefined);
                          if (hostEditor !== "new" && hostEditor?.id === host.id)
                            setHostEditor(undefined);
                          if (history?.id === host.id) setHistory(undefined);
                          inventory.refresh();
                        })
                      }
                    >
                      删除
                    </Button>
                  </div>
                </td>
              </tr>
            ))}
          </Table>
          {!visible.length && !inventory.loading && (
            <Empty>{hosts.length ? "没有匹配机器" : "尚未添加 SSH 机器"}</Empty>
          )}
        </div>
      </Card>
      {bindingFor && (
        <Bindings
          key={bindingFor.id}
          host={bindingFor}
          close={() => setBindingFor(undefined)}
          onSave={inventory.refresh}
        />
      )}
      {history && (
        <div data-testid="agent-check-history">
          <Card
            title={`${history.name} · 检查历史`}
            actions={
              <Button small onClick={() => setHistory(undefined)}>
                关闭
              </Button>
            }
          >
            <ResourceNotice
              loading={checks.loading && !checks.data}
              error={checks.error}
              retry={checks.refresh}
            />
            <Table headers={["时间", "来源", "结果", "延迟", "详情"]}>
              {checks.data?.map((row) => (
                <tr key={row.id}>
                  <td>{dateLabel(row.checked_at)}</td>
                  <td>{row.trigger === "manual" ? "手动" : "定时"}</td>
                  <td>
                    <Badge kind={row.status === "ok" ? "ok" : "danger"}>
                      {row.status === "ok" ? "成功" : "失败"}
                    </Badge>
                  </td>
                  <td>{row.latency_ms} ms</td>
                  <td className="wrap-cell">{row.error || "连接与认证正常"}</td>
                </tr>
              ))}
            </Table>
          </Card>
        </div>
      )}
      <Card title="Agent API 能力">
        <ResourceNotice
          loading={capabilities.loading && !operations.length}
          error={capabilities.error}
          retry={capabilities.refresh}
        />
        <Table testId="agent-capabilities" headers={["分组", "方法", "路径", "说明", "权限"]}>
          {operations.map((item) => (
            <tr key={item.method + item.path}>
              <td>{item.group}</td>
              <td>
                <Badge>{item.method}</Badge>
              </td>
              <td>
                <code>{item.path}</code>
              </td>
              <td className="wrap-cell">{item.description}</td>
              <td>{item.role}</td>
            </tr>
          ))}
        </Table>
      </Card>
    </div>
  );
}

function intervalLabel(seconds = 3600) {
  return seconds >= 3600 && seconds % 3600 === 0
    ? `每 ${seconds / 3600} 小时`
    : `每 ${Math.round(seconds / 60)} 分钟`;
}

function HostEditor({
  host,
  close,
  onSave,
}: {
  host?: Host;
  close: () => void;
  onSave: () => void;
}) {
  const [mode, setMode] = useState(host?.auth_mode || "private_key");
  const [enabled, setEnabled] = useState(host?.health_check_enabled ?? true);
  const action = useAction();
  const { toast } = useFeedback();
  const interval = host?.health_check_interval_seconds || 3600;
  const intervals = [...new Set([300, 900, 1800, 3600, 21600, 86400, interval])].sort(
    (a, b) => a - b,
  );

  function save(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    const data = new FormData(event.currentTarget);
    const body = {
      name: formText(data, "name"),
      address: formText(data, "address"),
      port: Number(formText(data, "port")),
      username: formText(data, "username"),
      auth_mode: mode,
      host_key: formText(data, "hostkey"),
      health_check_enabled: enabled,
      health_check_interval_seconds: Number(formText(data, "interval")),
      credential: mode === "agent" ? "" : String(data.get("credential") ?? ""),
    };
    void action.run(async () => {
      await request(host ? hostsPath + "/" + host.id : hostsPath, {
        method: host ? "PUT" : "POST",
        body,
      });
      onSave();
      toast("SSH 机器已保存");
    });
  }

  return (
    <Card
      title={host ? "编辑 SSH 机器" : "新增 SSH 机器"}
      description="建议固定 SSH Host key。凭据写入后不可回读。"
      actions={
        <Button small onClick={close} disabled={action.busy}>
          关闭
        </Button>
      }
    >
      <form className="form-stack" onSubmit={save}>
        <div className="form-grid">
          <Field label="资源名称">
            <input className="input" name="name" required defaultValue={host?.name} />
          </Field>
          <Field label="主机地址">
            <input className="input" name="address" required defaultValue={host?.address} />
          </Field>
          <Field label="端口">
            <input
              className="input"
              name="port"
              type="number"
              min={1}
              max={65535}
              required
              defaultValue={host?.port || 22}
            />
          </Field>
          <Field label="SSH 账号">
            <input className="input" name="username" required defaultValue={host?.username} />
          </Field>
          <Field label="认证方式">
            <select
              className="input"
              name="auth"
              value={mode}
              onChange={(event) => setMode(event.currentTarget.value)}
            >
              <option value="private_key">私钥</option>
              <option value="password">密码</option>
              <option value="agent">SSH Agent</option>
            </select>
          </Field>
          <Field label="Host public key">
            <input className="input" name="hostkey" defaultValue={host?.host_key} />
          </Field>
          {mode !== "agent" && (
            <Field
              label={mode === "password" ? "SSH 密码" : "SSH 私钥"}
              hint={host ? "留空保留当前凭据" : undefined}
            >
              {mode === "password" ? (
                <input
                  className="input"
                  name="credential"
                  type="password"
                  autoComplete="new-password"
                  required={!host || host.auth_mode !== mode}
                />
              ) : (
                <textarea
                  className="input code-edit"
                  name="credential"
                  rows={3}
                  spellCheck={false}
                  autoComplete="off"
                  required={!host || host.auth_mode !== mode}
                />
              )}
            </Field>
          )}
          <Field label="检查间隔">
            <select className="input" name="interval" defaultValue={interval}>
              {intervals.map((value) => (
                <option key={value} value={value}>
                  {intervalLabel(value)}
                </option>
              ))}
            </select>
          </Field>
        </div>
        <label className="check-field">
          <input
            type="checkbox"
            checked={enabled}
            onChange={(event) => setEnabled(event.currentTarget.checked)}
          />
          启用定时检查
        </label>
        <Button type="submit" tone="primary" disabled={action.busy}>
          保存机器
        </Button>
      </form>
    </Card>
  );
}

function Bindings({ host, close, onSave }: { host: Host; close: () => void; onSave: () => void }) {
  const endpoint = bindingPath(host.id);
  const resource = useResource<Binding[] | null>(endpoint);
  const [editor, setEditor] = useState<Binding>();
  const [sequence, reset] = useState(0);
  const action = useAction();
  const { toast, confirm } = useFeedback();
  const { projects } = useConsole();
  function edit(binding?: Binding) {
    setEditor(binding);
    reset((n) => n + 1);
  }
  function save(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    const data = new FormData(event.currentTarget);
    const payload = {
      project_name: formText(data, "project"),
      server_id: formText(data, "sid"),
      labels: formText(data, "labels"),
      content_tags: formText(data, "tags"),
      remote_root: formText(data, "root"),
      os: editor?.os || "linux",
      arch: formText(data, "arch"),
      skip_backup: data.has("skip"),
    };
    void action.run(async () => {
      await request(editor ? `${endpoint}/${editor.id}` : endpoint, {
        method: editor ? "PUT" : "POST",
        body: payload,
      });
      edit();
      resource.refresh();
      onSave();
      toast("目标绑定已保存");
    });
  }
  return (
    <Card
      title={`${host.name} · 目标绑定`}
      description="目标标签由项目名称和 server_id 组成。"
      actions={
        <Button small onClick={close}>
          关闭绑定面板
        </Button>
      }
    >
      <form key={sequence} className="editor-panel form-stack" onSubmit={save}>
        <div className="form-grid">
          <Field label="项目">
            <select
              className="input"
              name="project"
              required
              defaultValue={editor?.project_name ?? ""}
            >
              <option value="">选择项目</option>
              {projects.map((project) => (
                <option key={project.id} value={project.name}>
                  {project.name}
                </option>
              ))}
            </select>
          </Field>
          <Field label="Server ID">
            <input className="input" name="sid" required defaultValue={editor?.server_id} />
          </Field>
          <Field label="发布标签">
            <input className="input" name="labels" defaultValue={editor?.labels ?? "prod"} />
          </Field>
          <Field label="内容标签">
            <input className="input" name="tags" defaultValue={editor?.content_tags} />
          </Field>
          <Field label="远端实例根目录">
            <input
              className="input"
              name="root"
              required
              defaultValue={editor?.remote_root || "/opt/game-servers"}
            />
          </Field>
          <Field label="架构">
            <select className="input" name="arch" defaultValue={editor?.arch || "amd64"}>
              <option value="amd64">AMD64</option>
              <option value="arm64">ARM64</option>
            </select>
          </Field>
        </div>
        <label className="check-field">
          <input type="checkbox" name="skip" defaultChecked={editor?.skip_backup ?? false} />
          按该目标策略跳过备份
        </label>
        <div className="toolbar">
          <Button type="submit" tone="primary" disabled={action.busy}>
            {editor ? "保存修改" : "添加绑定"}
          </Button>
          {editor && (
            <Button small tone="ghost" onClick={() => edit()}>
              取消编辑
            </Button>
          )}
        </div>
      </form>
      <ResourceNotice
        loading={resource.loading && !resource.data}
        error={resource.error}
        retry={resource.refresh}
      />
      <Table headers={["目标标签", "发布 / 内容标签", "根目录", "备份", "操作"]}>
        {resource.data?.map((binding) => (
          <tr key={binding.id}>
            <td>
              <code>
                {binding.target_tag || `${binding.project_name || "未归属"}|${binding.server_id}`}
              </code>
            </td>
            <td>
              {binding.labels || "—"}
              <small>{binding.content_tags || "全部内容"}</small>
            </td>
            <td>
              <code>{binding.remote_root}</code>
            </td>
            <td>{binding.skip_backup ? "跳过" : "保留"}</td>
            <td>
              <div className="toolbar">
                <Button small onClick={() => edit(binding)} disabled={action.busy}>
                  编辑
                </Button>
                <Button
                  small
                  tone="danger"
                  disabled={action.busy}
                  onClick={() =>
                    void action.run(async () => {
                      if (
                        !(await confirm({
                          title: "删除目标绑定",
                          message: binding.target_tag || binding.server_id,
                          danger: true,
                          confirmText: "删除",
                        }))
                      )
                        return;
                      await request(`${endpoint}/${binding.id}`, { method: "DELETE" });
                      if (editor?.id === binding.id) edit();
                      resource.refresh();
                      onSave();
                    })
                  }
                >
                  删除
                </Button>
              </div>
            </td>
          </tr>
        ))}
      </Table>
    </Card>
  );
}
