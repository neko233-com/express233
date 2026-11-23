import { useState, type FormEvent } from "react";
import { request } from "../api";
import { csv, dateLabel, formText } from "../types";
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
import { submitDeployment, taskScope, type Deployment, type PushTask } from "./delivery-api";
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
  const { project, projectPath, latest, versions, versionsLoading, canWrite } = useWorkspace();
  const [editor, setEditor] = useState<PushTask | "new">();
  const editing = editor === "new" ? undefined : editor;
  const action = useAction();
  const { confirm, toast } = useFeedback();
  const base = projectPath + "/push-tasks";
  const published = versions.filter((row) => row.status === "published");
  const available = (version?: string) =>
    !version || published.some((row) => row.version === version);

  async function runTask(task: PushTask, dryRun: boolean) {
    if (!available(task.version)) {
      setEditor(task);
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
        title: `部署 ${task.name}`,
        confirmText: "确认部署",
        message: `版本：${version}\n目标：${task.server_ids?.join(", ") || "当前项目绑定的 SSH 目标"}\n标签：${task.tags?.join(", ") || "test"}（${task.tag_match === "any" ? "任一匹配" : "全部匹配"}）\n按解析出的目标串行部署。`,
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
      setEditor(undefined);
      refresh();
      toast("发布任务已保存");
      if (submitter?.id === "btnSaveAndRunPushTask") await runTask(task, false);
    });
  }

  return (
    <Card
      title="发布任务"
      description="保存固定版本、目标子集或标签条件，供手动和 Hook 重复执行。"
      id="deliveryTasksDetails"
      actions={
        canWrite &&
        !editor && (
          <Button
            small
            tone="primary"
            disabled={action.busy || loading || !!error}
            onClick={() => setEditor("new")}
          >
            新建任务
          </Button>
        )
      }
    >
      <ResourceNotice loading={loading && !tasks.length} error={error} retry={refresh} />
      {canWrite && editor && (
        <TaskForm
          key={editing?.id ?? "new"}
          task={editing}
          versions={published.map((row) => row.version)}
          latest={latest?.version}
          busy={action.busy || versionsLoading}
          canRun={Boolean(latest)}
          onSubmit={save}
          cancel={() => setEditor(undefined)}
        />
      )}
      {tasks.length > 0 && (
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
                      disabled={action.busy || versionsLoading || !latest}
                      onClick={() => void action.run(() => runTask(task, true))}
                    >
                      预演
                    </Button>
                    <Button
                      small
                      tone="primary"
                      disabled={action.busy || versionsLoading || !latest}
                      onClick={() => void action.run(() => runTask(task, false))}
                    >
                      部署
                    </Button>
                    <Button
                      small
                      tone="ghost"
                      disabled={action.busy}
                      onClick={() => setEditor(task)}
                    >
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
                          if (editing?.id === task.id) setEditor(undefined);
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
      )}
      {!tasks.length && !loading && !error && !editor && <Empty>暂无发布任务。</Empty>}
    </Card>
  );
}

function TaskForm({
  task,
  versions,
  latest,
  busy,
  canRun,
  cancel,
  onSubmit,
}: {
  task?: PushTask;
  versions: string[];
  latest?: string;
  busy: boolean;
  canRun: boolean;
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
            autoFocus
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
        <Field label="筛选标签" hint="留空时使用 test">
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
        <Button
          small
          type="submit"
          id="btnSaveAndRunPushTask"
          tone="primary"
          disabled={busy || !canRun}
        >
          保存并部署
        </Button>
        <Button small tone="ghost" disabled={busy} onClick={cancel}>
          取消编辑
        </Button>
      </div>
    </form>
  );
}
