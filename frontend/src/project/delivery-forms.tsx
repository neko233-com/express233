import { useState, type FormEvent, type Ref } from "react";
import { request } from "../api";
import { csv, dateLabel, formText } from "../types";
import {
  Badge,
  Button,
  Disclosure,
  Field,
  ResourceNotice,
  Table,
  useAction,
  useFeedback,
} from "../ui";
import {
  hookPayload,
  submitDeployment,
  taskScope,
  type Deployment,
  type Hook,
  type HookEvent,
  type PushTask,
} from "./delivery-api";
import { useWorkspace } from "./workspace";

export function TaskEditor({
  tasks,
  error,
  loading,
  refresh,
  onRun,
}: {
  tasks: PushTask[];
  error?: string;
  loading: boolean;
  refresh: () => void;
  onRun: (row: Deployment) => void;
}) {
  const { project, projectPath, latest, versions, canWrite } = useWorkspace();
  const [editing, setEditing] = useState<PushTask>();
  const [formKey, setFormKey] = useState(0);
  const action = useAction();
  const { confirm, toast } = useFeedback();
  const base = projectPath + "/push-tasks";
  const published = versions.filter((row) => row.status === "published");
  const available = (version?: string) =>
    !version || published.some((row) => row.version === version);

  function edit(task?: PushTask) {
    setEditing(task);
    setFormKey((n) => n + 1);
  }

  async function runTask(task: PushTask, dryRun: boolean) {
    if (!available(task.version)) {
      edit(task);
      toast("固定版本不可用，请明确选择一个已发布版本", "warn");
      return;
    }
    const version = task.version || latest?.version;
    if (!version) {
      toast("请先发布一个版本", "warn");
      return;
    }
    if (
      !dryRun &&
      !(await confirm({
        title: `发布 ${task.name}`,
        confirmText: "确认发布",
        message: `版本：${version}\n目标：${task.server_ids?.join(", ") || "当前项目全部匹配服务器（串行）"}\n先预拉取，等待旧服务正常退出，再换包、启动与检查。`,
      }))
    )
      return;
    onRun(
      await submitDeployment(base + `/${task.id}/run`, `${project.id}:task:${task.id}`, {
        dry_run: dryRun,
        version,
      }),
    );
  }

  function save(event: FormEvent<HTMLFormElement>, version: string) {
    event.preventDefault();
    const submitter = (event.nativeEvent as SubmitEvent).submitter;
    const data = new FormData(event.currentTarget);
    const fixed = formText(data, "target") === "fixed";
    const server_ids = fixed ? csv(formText(data, "servers")) : [];
    if (!available(version)) {
      toast(`固定版本 ${version} 已删除或不可用，请明确选择一个已发布版本。`, "warn");
      return;
    }
    if (fixed && !server_ids.length) {
      toast("固定目标至少需要一个 server_id", "warn");
      return;
    }
    const payload = {
      name: formText(data, "name"),
      version,
      server_ids,
      tags: csv(formText(data, "tags") || "test"),
      tag_match: formText(data, "match"),
    };
    void action.run(async () => {
      const task = await request<PushTask>(editing ? base + "/" + editing.id : base, {
        method: editing ? "PUT" : "POST",
        body: payload,
      });
      edit();
      refresh();
      toast("发布任务已保存");
      if (submitter?.id === "btnSaveAndRunPushTask") await runTask(task, false);
    });
  }

  return (
    <Disclosure
      title="高级发布任务"
      id="deliveryTasksDetails"
      badge={<Badge>{tasks.length} 个</Badge>}
    >
      <ResourceNotice loading={loading && !tasks.length} error={error} retry={refresh} />
      {canWrite && (
        <TaskForm
          key={formKey}
          task={editing}
          versions={published.map((row) => row.version)}
          latest={latest?.version}
          busy={action.busy}
          onSubmit={save}
          cancel={() => edit()}
        />
      )}
      <Table
        headers={["任务", "版本策略", "目标与标签", "执行次数", "最近运行", "操作"]}
        testId="release-task-list"
      >
        {tasks.map((task) => (
          <tr key={task.id}>
            <td>
              <strong>{task.name}</strong>
              <small>#{task.id}</small>
            </td>
            <td>
              {task.version ? (
                <>
                  <code>{task.version}</code>
                  {!available(task.version) && <small className="error">已删除或不可用</small>}
                </>
              ) : (
                <Badge>每次取最新已发布</Badge>
              )}
            </td>
            <td>
              {taskScope(task)}
              <small>
                {task.tag_match === "any" ? "任一标签" : "全部标签"} ·{" "}
                {task.tags?.join(", ") || "test"}
              </small>
            </td>
            <td>{task.run_count ?? 0}</td>
            <td>{dateLabel(task.last_run_at)}</td>
            <td>
              {canWrite && (
                <div className="toolbar">
                  <Button
                    small
                    disabled={action.busy}
                    onClick={() => void action.run(() => runTask(task, true))}
                  >
                    预演
                  </Button>
                  <Button
                    small
                    tone="primary"
                    disabled={action.busy}
                    onClick={() => void action.run(() => runTask(task, false))}
                  >
                    发布
                  </Button>
                  <Button small tone="ghost" disabled={action.busy} onClick={() => edit(task)}>
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
                            title: "删除发布任务",
                            message: `删除任务 ${task.name}。已执行记录会保留。`,
                            danger: true,
                          }))
                        )
                          return;
                        await request(base + "/" + task.id, { method: "DELETE" });
                        if (editing?.id === task.id) edit();
                        refresh();
                      })
                    }
                  >
                    删除
                  </Button>
                </div>
              )}
            </td>
          </tr>
        ))}
      </Table>
      {!tasks.length && !loading && <p className="hint">暂无高级发布任务。</p>}
    </Disclosure>
  );
}

function TaskForm({
  task,
  versions,
  latest,
  busy,
  cancel,
  onSubmit,
}: {
  task?: PushTask;
  versions: string[];
  latest?: string;
  busy: boolean;
  cancel: () => void;
  onSubmit: (event: FormEvent<HTMLFormElement>, version: string) => void;
}) {
  const [mode, setMode] = useState(task?.server_ids?.length ? "fixed" : "project");
  const [version, setVersion] = useState(task?.version ?? "");
  const unavailable = version && !versions.includes(version);
  return (
    <form
      className="form-stack editor-panel"
      data-testid="release-task-editor"
      onSubmit={(event) => onSubmit(event, version)}
    >
      <div className="form-grid">
        <Field label="任务名称">
          <input
            id="pushTaskName"
            name="name"
            className="input"
            required
            maxLength={100}
            defaultValue={task?.name}
          />
        </Field>
        <Field label="发布版本">
          <select
            id="pushTaskVersion"
            name="version"
            className="input"
            value={version}
            onChange={(event) => setVersion(event.currentTarget.value)}
          >
            <option value="">最新已发布版本{latest ? ` · ${latest}` : ""}</option>
            {unavailable && (
              <option value={version} disabled>
                {version} · 已删除或不可用
              </option>
            )}
            {versions.map((value) => (
              <option key={value} value={value}>
                {value}
              </option>
            ))}
          </select>
        </Field>
        <Field label="目标策略">
          <select
            id="pushTaskTargetMode"
            name="target"
            className="input"
            value={mode}
            onChange={(event) => setMode(event.currentTarget.value)}
          >
            <option value="project">跟随项目当前目标</option>
            <option value="fixed">固定 server_id 子集</option>
          </select>
        </Field>
        <Field label="固定 server_id">
          <input
            id="pushTaskServerIDs"
            name="servers"
            className="input"
            disabled={mode !== "fixed"}
            placeholder={mode === "fixed" ? "多个 ID 以逗号分隔" : "执行时解析当前项目绑定"}
            defaultValue={task?.server_ids?.join(", ")}
          />
        </Field>
        <Field label="筛选标签">
          <input
            className="input"
            id="pushTaskTags"
            name="tags"
            defaultValue={task?.tags?.join(", ") ?? "test"}
          />
        </Field>
        <Field label="匹配规则">
          <select
            className="input"
            id="pushTaskTagMatch"
            name="match"
            defaultValue={task?.tag_match ?? "all"}
          >
            <option value="all">标签同时满足</option>
            <option value="any">任一标签匹配</option>
          </select>
        </Field>
      </div>
      <div className="toolbar">
        <Button small type="submit" id="btnSavePushTask" disabled={busy}>
          {task ? "更新任务" : "保存任务"}
        </Button>
        <Button small type="submit" id="btnSaveAndRunPushTask" tone="primary" disabled={busy}>
          保存并发布
        </Button>
        {task && (
          <Button small tone="ghost" disabled={busy} onClick={cancel}>
            取消编辑
          </Button>
        )}
      </div>
    </form>
  );
}

export function HookEditor({
  hooks,
  tasks,
  events,
  error,
  refresh,
  hookRef,
}: {
  hooks: Hook[];
  tasks: PushTask[];
  events: HookEvent[];
  error?: string;
  refresh: () => void;
  hookRef: Ref<HTMLDetailsElement>;
}) {
  const { project, projectPath, canWrite } = useWorkspace();
  const [editing, setEditing] = useState<Hook>();
  const [formKey, setFormKey] = useState(0);
  const action = useAction();
  const { confirm, toast } = useFeedback();
  const base = projectPath + "/release-hooks";
  function edit(hook?: Hook) {
    setEditing(hook);
    setFormKey((key) => key + 1);
  }
  function save(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    const data = new FormData(event.currentTarget);
    const body = {
      name: formText(data, "name"),
      task_id: Number(formText(data, "task")),
      debounce_seconds: Number(formText(data, "delay")),
      enabled: data.has("enabled"),
    };
    void action.run(async () => {
      await request(editing ? `${base}/${editing.id}` : base, {
        method: editing ? "PUT" : "POST",
        body,
      });
      edit();
      refresh();
      toast("Hook 已保存");
    });
  }
  return (
    <Disclosure
      title="自动 Hook"
      id="deliveryHooksDetails"
      ref={hookRef}
      badge={<Badge>{hooks.length} 个</Badge>}
    >
      <ResourceNotice error={error} retry={refresh} />
      {canWrite && (
        <form
          key={formKey}
          className="form-stack editor-panel"
          data-testid="release-hook-editor"
          onSubmit={save}
        >
          <div className="form-grid">
            <Field label="Hook 名称">
              <input
                id="releaseHookName"
                className="input"
                name="name"
                defaultValue={editing?.name}
                required
                maxLength={100}
              />
            </Field>
            <Field label="关联发布任务">
              <select
                id="releaseHookTask"
                className="input"
                name="task"
                required
                defaultValue={editing?.task_id ?? ""}
              >
                <option value="">选择当前项目的发布任务</option>
                {tasks.map((task) => (
                  <option key={task.id} value={task.id}>
                    {task.name} · {taskScope(task)}
                  </option>
                ))}
              </select>
            </Field>
            <Field label="合并窗口（秒）">
              <input
                id="releaseHookDebounce"
                type="number"
                className="input"
                name="delay"
                min={1}
                max={3600}
                required
                defaultValue={editing?.debounce_seconds ?? 30}
              />
            </Field>
            <label className="check-field">
              <input
                name="enabled"
                id="releaseHookEnabled"
                type="checkbox"
                defaultChecked={editing?.enabled ?? true}
              />
              启用 Hook
            </label>
          </div>
          <div className="toolbar">
            <Button type="submit" small tone="primary" disabled={action.busy || !tasks.length}>
              {editing ? "更新 Hook" : "保存 Hook"}
            </Button>
            {editing && (
              <Button tone="ghost" small disabled={action.busy} onClick={() => edit()}>
                取消编辑
              </Button>
            )}
          </div>
        </form>
      )}
      <Table
        headers={["Hook", "关联任务", "开关", "合并窗口", "触发统计", "最近状态", "操作"]}
        testId="release-hook-list"
      >
        {hooks.map((hook) => {
          const task = tasks.find((item) => item.id === hook.task_id);
          return (
            <tr key={hook.id}>
              <td>
                <strong>{hook.name}</strong>
                <small>#{hook.id}</small>
              </td>
              <td>
                <strong>{hook.task_name || task?.name || "任务已失效"}</strong>
                <small>项目 · {project.name}</small>
                <small>{task ? taskScope(task) : "任务不可用"}</small>
              </td>
              <td>
                <label className="switch">
                  <input
                    type="checkbox"
                    checked={hook.enabled}
                    aria-label={`启用 ${hook.name}`}
                    disabled={action.busy || !canWrite}
                    onChange={(event) => {
                      const enabled = event.currentTarget.checked;
                      void action.run(async () => {
                        await request(`${base}/${hook.id}`, {
                          method: "PUT",
                          body: hookPayload(hook, enabled),
                        });
                        refresh();
                      });
                    }}
                  />
                  <span aria-hidden="true" />
                </label>
                <small>{hook.enabled ? "已启用" : "已停用"}</small>
              </td>
              <td>
                {hook.debounce_seconds} 秒
                {Boolean(hook.pending_events) && <small>等待中 · {hook.pending_events} 次</small>}
              </td>
              <td>
                {hook.trigger_count ?? 0} 次触发
                <small>
                  {hook.merge_count ?? 0} 合并 · {hook.run_count ?? 0} 派发
                </small>
              </td>
              <td>
                <Badge kind={hook.last_status === "failed" ? "danger" : "neutral"}>
                  {hook.last_status || "尚未触发"}
                </Badge>
                {hook.last_error && <small>{hook.last_error}</small>}
              </td>
              <td>
                {canWrite && (
                  <div className="toolbar">
                    <Button
                      small
                      disabled={!hook.enabled || action.busy}
                      onClick={() =>
                        void action.run(async () => {
                          await request(`${base}/${hook.id}/trigger`, {
                            method: "POST",
                            body: { source: "manual_ui" },
                          });
                          refresh();
                          toast("已进入 Hook 合并窗口");
                        })
                      }
                    >
                      立即触发
                    </Button>
                    <Button small tone="ghost" disabled={action.busy} onClick={() => edit(hook)}>
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
                              title: "删除自动 Hook",
                              message: `删除 ${hook.name}。触发历史保留 30 天。`,
                              danger: true,
                            }))
                          )
                            return;
                          await request(`${base}/${hook.id}`, { method: "DELETE" });
                          if (editing?.id === hook.id) edit();
                          refresh();
                        })
                      }
                    >
                      删除
                    </Button>
                  </div>
                )}
              </td>
            </tr>
          );
        })}
      </Table>
      <header className="card-head spaced">
        <h3>Hook 触发记录</h3>
        <Button small onClick={refresh}>
          刷新
        </Button>
      </header>
      <Table
        headers={["时间", "Hook", "事件", "来源 / 版本", "合并", "结果"]}
        testId="release-hook-events"
      >
        {events.map((event) => (
          <tr key={event.id}>
            <td>{dateLabel(event.created_at)}</td>
            <td>{event.hook_name}</td>
            <td>
              <Badge>
                {event.status === "cancelled"
                  ? "等待已取消"
                  : event.kind === "dispatch"
                    ? "最终派发"
                    : event.status === "merged"
                      ? "合并触发"
                      : "首次触发"}
              </Badge>
            </td>
            <td>
              {event.source || "system"}
              <small>{event.version || "—"}</small>
            </td>
            <td>{event.merged_events || 1} 次</td>
            <td>
              {event.deployment_status || event.status}
              {event.deployment_id && <small>发布 #{event.deployment_id}</small>}
            </td>
          </tr>
        ))}
      </Table>
    </Disclosure>
  );
}
