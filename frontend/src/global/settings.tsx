import { useState, type FormEvent } from "react";
import { Link, useSearchParams } from "react-router";
import { parseDocument } from "yaml";
import { query, request, useResource } from "../api";
import { useConsole } from "../app";
import { Code } from "../code";
import { usePolling } from "../project/delivery-api";
import { dateLabel, formText } from "../types";
import {
  Badge,
  Button,
  Card,
  Field,
  PageHeader,
  ResourceNotice,
  Table,
  useAction,
  useFeedback,
} from "../ui";

interface ServerYAMLData {
  content: string;
}
interface Preview {
  rendered_files?: { basename: string; path: string; before: string; after: string }[];
  warnings?: string[];
}

export function ServerYAML() {
  const resource = useResource<ServerYAMLData>("/api/server-yaml");
  return (
    <div className="section-stack">
      <PageHeader title="高级配置" description="编辑当前租户共享的 server.yaml。" />
      <ResourceNotice
        loading={resource.loading && !resource.data}
        error={resource.error}
        retry={resource.refresh}
      />
      {resource.data && <YAMLEditor key={resource.data.content} initial={resource.data.content} />}
      <RawPreview />
    </div>
  );
}

function YAMLEditor({ initial }: { initial: string }) {
  const { session } = useConsole();
  const [content, setContent] = useState(initial);
  const [saved, setSaved] = useState(initial);
  const { toast } = useFeedback();
  const action = useAction();
  return (
    <Card
      title="server.yaml"
      actions={
        <Badge kind={content === saved ? "neutral" : "warn"}>
          {content === saved ? "已保存" : "未保存"}
        </Badge>
      }
    >
      <form
        className="form-stack"
        onSubmit={(event) => {
          event.preventDefault();
          void action.run(async () => {
            const yaml = parseDocument(content);
            if (yaml.errors.length) throw new Error(yaml.errors[0].message);
            await request("/api/server-yaml", { method: "PUT", body: { content } });
            setSaved(content);
            toast("server.yaml 已保存");
          });
        }}
      >
        <textarea
          id="serverYaml"
          className="input code-edit"
          aria-label="server.yaml 编辑器"
          data-testid="server-yaml-editor"
          value={content}
          readOnly={!session.is_admin}
          disabled={action.busy}
          rows={16}
          spellCheck={false}
          onChange={(event) => setContent(event.currentTarget.value)}
        />
        {session.is_admin && (
          <div className="toolbar">
            <Button type="submit" tone="primary" disabled={action.busy}>
              保存
            </Button>
            {content !== saved && (
              <Button small onClick={() => setContent(saved)}>
                恢复已保存内容
              </Button>
            )}
          </div>
        )}
      </form>
    </Card>
  );
}

function RawPreview() {
  const { projects } = useConsole();
  const servers = useResource<{ server_ids: string[] }>("/api/server-ids");
  const action = useAction();
  const [preview, setPreview] = useState<Preview>();
  const { toast } = useFeedback();
  return (
    <Card title="配置预览">
      <form
        className="toolbar"
        onSubmit={(event) => {
          event.preventDefault();
          const data = new FormData(event.currentTarget);
          const project = formText(data, "project");
          const server_id = formText(data, "server");
          const version = formText(data, "version");
          if (!server_id || !project || !version) {
            toast("填写 project / version / server_id", "warn");
            return;
          }
          setPreview(undefined);
          void action.run(async () =>
            setPreview(
              await request<Preview>(query("/api/deploy/preview", { project, version, server_id })),
            ),
          );
        }}
      >
        <Field label="server_id">
          <input className="input" name="server" id="previewServerId" list="raw-preview-servers" />
        </Field>
        <datalist id="raw-preview-servers">
          {servers.data?.server_ids?.map((server) => (
            <option key={server} value={server} />
          ))}
        </datalist>
        <Field label="项目">
          <input className="input" name="project" id="previewProject" list="raw-preview-projects" />
        </Field>
        <datalist id="raw-preview-projects">
          {projects.map((project) => (
            <option key={project.id} value={project.name} />
          ))}
        </datalist>
        <Field label="版本">
          <input className="input" name="version" id="previewVersion" />
        </Field>
        <Button type="submit" tone="primary" disabled={action.busy}>
          预览 diff
        </Button>
      </form>
      {preview && (
        <div className="section-stack spaced" id="previewOut">
          {preview.warnings?.map((warning) => (
            <p key={warning} className="notice">
              {warning}
            </p>
          ))}
          {!preview.rendered_files?.length && <p className="hint">没有匹配的覆盖配置。</p>}
          {preview.rendered_files?.map((file) => (
            <div key={file.path}>
              <h3>{file.path || file.basename}</h3>
              <div className="two-col">
                {[
                  ["原始配置", file.before],
                  ["替换后配置", file.after],
                ].map(([name, source]) => (
                  <section key={name}>
                    <h4>{name}</h4>
                    <Code text={source} path={file.path || file.basename} />
                  </section>
                ))}
              </div>
            </div>
          ))}
        </div>
      )}
    </Card>
  );
}

const tabs = [
  ["users", "账号"],
  ["audit", "审计"],
  ["login-security", "登录安全"],
  ["onboarding", "新手引导"],
] as const;

export function Settings() {
  const { session } = useConsole();
  const [params, setParams] = useSearchParams();
  const active = session.is_admin
    ? tabs.find(([key]) => key === params.get("tab"))?.[0] || "users"
    : "users";
  return (
    <div className="section-stack">
      <PageHeader title="系统设置" />
      {session.is_admin && (
        <nav className="segments" aria-label="系统设置" id="settingsTabs">
          {tabs.map(([key, label]) => (
            <button
              type="button"
              data-stab={key}
              key={key}
              className={`seg-tab ${key === active ? "active" : ""}`}
              aria-current={key === active ? "page" : undefined}
              onClick={() => setParams(key === "users" ? {} : { tab: key })}
            >
              {label}
            </button>
          ))}
        </nav>
      )}
      {active === "users" && (
        <div id="stab-users" className="section-stack">
          <PasswordForm />
          {session.is_root && <Maintenance />}
          {session.is_admin && <Accounts />}
        </div>
      )}
      {active === "audit" && <Audit />}
      {active === "login-security" && <LoginSecurity />}
      {active === "onboarding" && (
        <Card title="新手引导" id="stab-onboarding">
          <ol className="onboarding-steps">
            <li>创建一个项目和草稿版本，上传程序、配置文件或归档。</li>
            <li>按需要添加分服覆盖，并预览目标的配置内容。</li>
            <li>检查并发布版本，再从项目部署页预演并确认目标。</li>
          </ol>
          <div className="toolbar">
            <Link className="btn btn-secondary" to="/">
              添加演示项目
            </Link>
            <Link className="btn btn-primary" to="/guide">
              打开接入指南
            </Link>
          </div>
        </Card>
      )}
    </div>
  );
}

function PasswordForm() {
  const action = useAction();
  const { toast } = useFeedback();
  return (
    <Card title="我的账号">
      <form
        className="toolbar"
        onSubmit={(event) => {
          event.preventDefault();
          const form = event.currentTarget;
          const data = new FormData(form);
          void action.run(async () => {
            await request("/api/me/password", {
              method: "POST",
              body: {
                old_password: String(data.get("old") ?? ""),
                new_password: String(data.get("new") ?? ""),
              },
            });
            form.reset();
            toast("密码已更新，请重新登录");
            await request("/api/me").catch(() => {});
          });
        }}
      >
        <Field label="当前密码">
          <input
            type="password"
            id="myOldPass"
            name="old"
            className="input"
            autoComplete="current-password"
            required
          />
        </Field>
        <Field label="新密码">
          <input
            type="password"
            id="myNewPass"
            name="new"
            className="input"
            autoComplete="new-password"
            required
          />
        </Field>
        <Button type="submit" tone="primary" disabled={action.busy}>
          更新密码
        </Button>
      </form>
    </Card>
  );
}

interface User {
  id: number;
  username: string;
  role?: string;
  is_admin?: boolean;
  token?: string;
}

function Accounts() {
  const { session } = useConsole();
  const users = useResource<User[] | null>("/api/users");
  const [tokens, setTokens] = useState<Record<number, string>>({});
  const action = useAction();
  const { confirm, prompt, toast } = useFeedback();

  function save(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    const form = event.currentTarget;
    const fields = new FormData(form);
    const role = formText(fields, "role");
    void action.run(async () => {
      const created = await request<User>("/api/users", {
        method: "POST",
        body: {
          username: formText(fields, "username"),
          password: String(fields.get("password") ?? ""),
          role,
          is_admin: role === "admin",
        },
      });
      const token = created.token;
      if (token) setTokens((items) => ({ ...items, [created.id]: token }));
      form.reset();
      users.refresh();
      toast("账号已创建");
    });
  }
  return (
    <Card
      title="账号管理"
      actions={
        <Button small onClick={users.refresh}>
          刷新
        </Button>
      }
    >
      <form className="toolbar editor-panel" onSubmit={save}>
        <Field label="用户名">
          <input id="newUser" name="username" className="input" autoComplete="off" required />
        </Field>
        <Field label="密码">
          <input
            id="newUserPass"
            name="password"
            type="password"
            className="input"
            autoComplete="new-password"
            required
          />
        </Field>
        <Field label="角色">
          <select id="newUserRole" name="role" className="input">
            <option value="viewer">viewer</option>
            <option value="operator">operator</option>
            <option value="admin">admin</option>
          </select>
        </Field>
        <Button type="submit" tone="primary" disabled={action.busy}>
          创建
        </Button>
      </form>
      <ResourceNotice
        loading={users.loading && !users.data}
        error={users.error}
        retry={users.refresh}
      />
      <Table headers={["ID", "用户名", "角色", "拉取 Token", "操作"]} id="userTable">
        {users.data?.map((user) => (
          <tr key={user.id}>
            <td>{user.id}</td>
            <td>{user.username}</td>
            <td>
              <Badge>{user.role || (user.is_admin ? "admin" : "viewer")}</Badge>
            </td>
            <td>
              {tokens[user.id] ? (
                <Button
                  small
                  onClick={() =>
                    void action.run(async () => {
                      await navigator.clipboard.writeText(tokens[user.id]);
                      setTokens((current) => {
                        const remaining = { ...current };
                        delete remaining[user.id];
                        return remaining;
                      });
                      toast("新 Token 已复制");
                    })
                  }
                >
                  复制新 Token
                </Button>
              ) : (
                <span className="hint">已隐藏</span>
              )}
            </td>
            <td>
              <div className="toolbar">
                <Button
                  small
                  disabled={action.busy}
                  onClick={() =>
                    void action.run(async () => {
                      if (
                        !(await confirm({
                          title: "刷新拉取 Token",
                          message: `刷新 ${user.username} 的 Token 后，旧 Token 会立即失效。`,
                          danger: true,
                          confirmText: "刷新 Token",
                        }))
                      )
                        return;
                      const result = await request<{ token: string }>(
                        `/api/users/${user.id}/refresh-token`,
                        { method: "POST" },
                      );
                      setTokens((items) => ({ ...items, [user.id]: result.token }));
                    })
                  }
                >
                  刷新 Token
                </Button>
                {user.username !== session.username && user.username !== "root" && (
                  <>
                    <Button
                      small
                      tone="ghost"
                      disabled={action.busy}
                      onClick={() =>
                        void action.run(async () => {
                          const password = await prompt({
                            title: "重置密码",
                            message: `为 ${user.username} 设置新密码。`,
                            value: "",
                            secret: true,
                            confirmText: "重置",
                          });
                          if (!password) return;
                          await request(`/api/users/${user.id}/password`, {
                            method: "PUT",
                            body: { password },
                          });
                          toast("密码已重置");
                        })
                      }
                    >
                      重置密码
                    </Button>
                    <Button
                      small
                      tone="danger"
                      disabled={action.busy}
                      onClick={() =>
                        void action.run(async () => {
                          if (
                            !(await confirm({
                              title: "删除账号",
                              message: `删除 ${user.username} 并撤销其访问权限。`,
                              danger: true,
                              confirmText: "删除",
                            }))
                          )
                            return;
                          await request(`/api/users/${user.id}`, { method: "DELETE" });
                          users.refresh();
                        })
                      }
                    >
                      删除
                    </Button>
                  </>
                )}
              </div>
            </td>
          </tr>
        ))}
      </Table>
    </Card>
  );
}

interface AuditRow {
  id?: number;
  at: string;
  username: string;
  action: string;
  detail: string;
  ip?: string;
}

function Audit() {
  const rows = useResource<AuditRow[] | null>("/api/audit-logs");
  const [filter, setFilter] = useState("");
  const visible =
    rows.data?.filter((row) =>
      `${row.username} ${row.action} ${row.detail}`
        .toLowerCase()
        .includes(filter.trim().toLowerCase()),
    ) ?? [];
  return (
    <Card
      title="操作审计"
      id="stab-audit"
      actions={
        <>
          <input
            className="input"
            type="search"
            placeholder="搜索操作…"
            aria-label="搜索审计"
            value={filter}
            onChange={(event) => setFilter(event.currentTarget.value)}
          />
          <Button small onClick={rows.refresh}>
            刷新
          </Button>
        </>
      }
    >
      <ResourceNotice
        loading={rows.loading && !rows.data}
        error={rows.error}
        retry={rows.refresh}
      />
      <Table id="auditTable" headers={["时间", "用户", "动作", "详情", "来源 IP"]}>
        {visible.map((row, index) => (
          <tr key={row.id ?? index}>
            <td>{dateLabel(row.at)}</td>
            <td>{row.username}</td>
            <td>
              <code>{row.action}</code>
            </td>
            <td className="wrap-cell">{row.detail}</td>
            <td>{row.ip || "—"}</td>
          </tr>
        ))}
      </Table>
      {!rows.loading && !visible.length && <p className="hint">没有匹配的审计记录。</p>}
    </Card>
  );
}

interface Protection {
  enabled: boolean;
  failure_limit: number;
  ban_seconds: number;
}
interface LoginFailure {
  ip: string;
  username?: string;
  attempt_count: number;
  failures: number;
  banned_until?: string;
  last_attempt_times?: string[];
}
function LoginSecurity() {
  const config = useResource<Protection>("/api/security/login-protection");
  const activity = useResource<LoginFailure[] | null>("/api/security/login-ip-bans");
  const action = useAction();
  const { confirm } = useFeedback();
  const settings = config.data;
  return (
    <Card
      id="stab-login-security"
      title="登录安全"
      description={
        settings?.enabled
          ? `同一 IP 连续失败 ${settings.failure_limit} 次后暂停 ${settings.ban_seconds} 秒。`
          : "记录登录失败来源。"
      }
      actions={
        <>
          <label className="switch">
            <input
              type="checkbox"
              id="loginProtectionEnabled"
              aria-label="启用登录失败保护"
              checked={settings?.enabled ?? false}
              disabled={!settings || action.busy}
              onChange={(event) => {
                const enabled = event.currentTarget.checked;
                void action.run(async () => {
                  await request("/api/security/login-protection", {
                    method: "PUT",
                    body: { enabled },
                  });
                  config.refresh();
                });
              }}
            />
            <span aria-hidden="true" />
          </label>
          <Button
            small
            onClick={() => {
              config.refresh();
              activity.refresh();
            }}
          >
            刷新
          </Button>
        </>
      }
    >
      <ResourceNotice
        loading={(config.loading && !settings) || (activity.loading && !activity.data)}
        error={config.error || activity.error}
        retry={() => {
          config.refresh();
          activity.refresh();
        }}
      />
      <Table
        id="loginAttackTable"
        headers={["来源 IP", "目标账号", "累计失败", "当前窗口", "状态", "最近失败", "操作"]}
      >
        {activity.data?.map((row) => (
          <tr key={row.ip}>
            <td>
              <code>{row.ip}</code>
            </td>
            <td>{row.username || "—"}</td>
            <td>{row.attempt_count}</td>
            <td>{row.failures}</td>
            <td>
              {row.banned_until && Date.parse(row.banned_until) > Date.now() ? (
                <Badge kind="warn">暂停至 {dateLabel(row.banned_until)}</Badge>
              ) : (
                "记录中"
              )}
            </td>
            <td>
              {row.last_attempt_times?.map((value) => (
                <small key={value}>{dateLabel(value)}</small>
              ))}
            </td>
            <td>
              <Button
                small
                tone="ghost"
                disabled={action.busy}
                onClick={() =>
                  void action.run(async () => {
                    if (
                      !(await confirm({
                        title: "清除登录记录",
                        message: `清除 ${row.ip} 的失败记录和暂停状态。`,
                        danger: true,
                        confirmText: "清除",
                      }))
                    )
                      return;
                    await request("/api/security/login-ip-bans/" + encodeURIComponent(row.ip), {
                      method: "DELETE",
                    });
                    activity.refresh();
                  })
                }
              >
                清除
              </Button>
            </td>
          </tr>
        ))}
      </Table>
    </Card>
  );
}

interface UpdateStatus {
  running: boolean;
  ok: boolean;
  current_version: string;
  current_commit: string;
  target_version?: string;
  started_at?: string;
  finished_at?: string;
  error?: string;
  output?: string;
}

function Maintenance() {
  const resource = useResource<UpdateStatus>("/api/system/update");
  const status = resource.data;
  const action = useAction();
  const { confirm } = useFeedback();
  usePolling(!resource.loading && Boolean(status?.running), 2200, resource.refresh);
  return (
    <Card
      title="系统维护"
      description={
        status ? (
          <code>
            {status.current_version} {status.current_commit}
          </code>
        ) : undefined
      }
      actions={
        <>
          <Button small onClick={resource.refresh}>
            刷新状态
          </Button>
          <Button
            small
            tone="primary"
            disabled={!status || status.running || action.busy}
            onClick={() =>
              void action.run(async () => {
                if (
                  !(await confirm({
                    title: "更新中央服",
                    message: "更新到最新 GitHub Release，更新期间服务会短暂重启。",
                    confirmText: "开始更新",
                  }))
                )
                  return;
                await request("/api/system/update", { method: "POST" });
                resource.refresh();
              })
            }
          >
            更新到最新版本
          </Button>
        </>
      }
    >
      <ResourceNotice
        loading={resource.loading && !status}
        error={resource.error}
        retry={resource.refresh}
      />
      {status && (
        <div className="form-stack">
          <div className="toolbar">
            <Badge
              kind={
                status.running ? "info" : status.error ? "danger" : status.ok ? "ok" : "neutral"
              }
            >
              {status.running
                ? "更新中"
                : status.finished_at
                  ? status.ok
                    ? "已完成"
                    : "失败"
                  : "空闲"}
            </Badge>
            {status.target_version && (
              <span className="hint">目标版本 {status.target_version}</span>
            )}
          </div>
          {status.error && <p className="error">{status.error}</p>}
          {status.output && <Code text={status.output} id="systemUpdateStatus" />}
        </div>
      )}
    </Card>
  );
}
