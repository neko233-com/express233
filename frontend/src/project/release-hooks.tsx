import { useState, type FormEvent } from "react";
import { Link } from "react-router";
import { query, request, useResource } from "../api";
import { dateLabel, formText } from "../types";
import {
  Badge,
  Button,
  Card,
  Empty,
  Field,
  ResourceNotice,
  Table,
  useAction,
  useFeedback,
} from "../ui";
import {
  deploymentStatus,
  taskScope,
  usePolling,
  type Hook,
  type HookEvent,
  type PushTask,
} from "./delivery-api";
import { useWorkspace } from "./workspace";

export function HookEditor({
  tasks: taskResource,
}: {
  tasks: ReturnType<typeof useResource<PushTask[] | null>>;
}) {
  const { projectPath, latest, versionsLoading, canWrite, panelLink } = useWorkspace();
  const [editor, setEditor] = useState<Hook | "new">();
  const editing = editor === "new" ? undefined : editor;
  const action = useAction();
  const { confirm, toast } = useFeedback();
  const base = projectPath + "/release-hooks";
  const hookResource = useResource<Hook[] | null>(base);
  const eventResource = useResource<HookEvent[] | null>(
    query(projectPath + "/release-hook-events", { limit: 100 }),
  );
  const tasks = taskResource.data ?? [];
  const hooks = hookResource.data ?? [];
  const events = eventResource.data ?? [];
  const enabledCount = hooks.filter((hook) => hook.enabled).length;
  const tasksByID = new Map(tasks.map((task) => [task.id, task]));
  const blocked = action.busy || hookResource.loading;

  usePolling(
    !hookResource.loading &&
      !eventResource.loading &&
      hooks.some((hook) => !!hook.pending_events || hook.last_status === "running"),
    1800,
    refresh,
  );

  function refresh() {
    hookResource.refresh();
    eventResource.refresh();
  }

  function setEnabled(rows: Hook[], enabled: boolean, bulk: boolean) {
    void action.run(async () => {
      if (
        bulk &&
        !(await confirm({
          title: enabled ? "批量启用 Hook" : "批量停用 Hook",
          message: enabled
            ? `启用 ${rows.length} 个 Hook。后续版本发布会触发这些规则。`
            : `停用 ${rows.length} 个 Hook，并取消仍在合并窗口中等待的触发。`,
          confirmText: enabled ? "启用" : "停用",
        }))
      )
        return;
      try {
        for (const hook of rows) {
          await request(`${base}/${hook.id}`, { method: "PUT", body: { enabled } });
        }
        if (bulk) toast(`${rows.length} 个 Hook 已${enabled ? "启用" : "停用"}`);
      } finally {
        refresh();
      }
    });
  }

  function trigger(hook: Hook) {
    if (!latest) return;
    const version = latest.version;
    void action.run(async () => {
      if (
        !(await confirm({
          title: `触发 ${hook.name}`,
          message: `已发布版本 ${version} 将进入 ${hook.debounce_seconds} 秒的合并窗口。窗口结束后，Hook 按最后一次触发的版本部署关联任务。`,
          confirmText: "确认触发",
        }))
      )
        return;
      await request(`${base}/${hook.id}/trigger`, {
        method: "POST",
        body: { source: "manual_ui", version },
      });
      refresh();
      toast("已进入 Hook 合并窗口");
    });
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
      setEditor(undefined);
      refresh();
      toast("Hook 已保存");
    });
  }
  return (
    <>
      <Card
        title="自动 Hook"
        id="deliveryHooksDetails"
        description="发布版本时触发，合并窗口结束后按关联任务的范围部署。"
        actions={
          canWrite && (
            <>
              {hooks.length > 0 && (
                <>
                  <Button
                    small
                    id="deliveryAutomationToggle"
                    disabled={blocked || enabledCount === hooks.length}
                    onClick={() =>
                      setEnabled(
                        hooks.filter((hook) => !hook.enabled),
                        true,
                        true,
                      )
                    }
                  >
                    启用全部
                  </Button>
                  <Button
                    small
                    disabled={blocked || !enabledCount}
                    onClick={() =>
                      setEnabled(
                        hooks.filter((hook) => hook.enabled),
                        false,
                        true,
                      )
                    }
                  >
                    停用全部
                  </Button>
                </>
              )}
              {!editor && (
                <Button
                  small
                  tone="primary"
                data-testid="new-release-hook"
                  disabled={blocked || !tasks.length || taskResource.loading}
                  onClick={() => setEditor("new")}
                >
                  新建 Hook
                </Button>
              )}
            </>
          )
        }
      >
        <ResourceNotice
          loading={hookResource.loading && !hookResource.data}
          error={hookResource.error}
          retry={refresh}
        />
        <ResourceNotice
          loading={taskResource.loading && !taskResource.data}
          error={taskResource.error}
          retry={taskResource.refresh}
        />
        {canWrite && editor && (
          <form
            key={editing?.id ?? "new"}
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
                  autoFocus
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
              <Button
                tone="ghost"
                small
                disabled={action.busy}
                onClick={() => setEditor(undefined)}
              >
                取消编辑
              </Button>
            </div>
          </form>
        )}
        {hooks.length > 0 && (
          <Table
            headers={["Hook", "关联任务", "开关", "合并窗口", "触发统计", "最近状态", "操作"]}
            testId="release-hook-list"
          >
            {hooks.map((hook) => {
              const task = tasksByID.get(hook.task_id);
              return (
                <tr key={hook.id}>
                  <td>
                    <strong>{hook.name}</strong>
                    <small>#{hook.id}</small>
                  </td>
                  <td>
                    <strong>{hook.task_name || task?.name || "任务已失效"}</strong>
                    <small>{task ? taskScope(task) : "任务不可用"}</small>
                  </td>
                  <td>
                    <label className="switch">
                      <input
                        type="checkbox"
                        checked={hook.enabled}
                        aria-label={`启用 ${hook.name}`}
                        disabled={blocked || !canWrite}
                        onChange={(event) => setEnabled([hook], event.currentTarget.checked, false)}
                      />
                      <span aria-hidden="true" />
                    </label>
                    <small>{hook.enabled ? "已启用" : "已停用"}</small>
                  </td>
                  <td>
                    {hook.debounce_seconds} 秒
                    {Boolean(hook.pending_events) && (
                      <small>等待中 · {hook.pending_events} 次</small>
                    )}
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
                          disabled={!hook.enabled || blocked || versionsLoading || !latest}
                          onClick={() => trigger(hook)}
                        >
                          触发 Hook
                        </Button>
                        <Button
                          small
                          tone="ghost"
                          disabled={blocked}
                          onClick={() => setEditor(hook)}
                        >
                          编辑
                        </Button>
                        <Button
                          small
                          tone="danger"
                          disabled={blocked}
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
                              if (editing?.id === hook.id) setEditor(undefined);
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
        )}
        {!hooks.length && !editor && !hookResource.loading && !hookResource.error && (
          <Empty>
            暂无自动 Hook。
            {canWrite && !tasks.length && !taskResource.loading && !taskResource.error && (
              <Link to={panelLink("automation")}>先创建发布任务</Link>
            )}
          </Empty>
        )}
      </Card>
      <Card
        title="Hook 触发记录"
        actions={
          <Button small onClick={refresh}>
            刷新
          </Button>
        }
      >
        <ResourceNotice
          loading={eventResource.loading && !eventResource.data}
          error={eventResource.error}
          retry={eventResource.refresh}
        />
        {events.length > 0 ? (
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
                  {event.deployment_status
                    ? deploymentStatus(event.deployment_status, false)
                    : event.status}
                  {event.deployment_id && (
                    <small>
                      <Link to={panelLink("delivery", { run: event.deployment_id })}>
                        部署 #{event.deployment_id}
                      </Link>
                    </small>
                  )}
                </td>
              </tr>
            ))}
          </Table>
        ) : (
          !eventResource.loading && !eventResource.error && <Empty>暂无触发记录。</Empty>
        )}
      </Card>
    </>
  );
}
