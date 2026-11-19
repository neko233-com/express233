import { useState } from "react";
import { Copy, Users } from "lucide-react";
import { request, useResource } from "../api";
import { dateLabel, formText } from "../types";
import { Badge, Button, Card, Field, ResourceNotice, Table, useAction, useFeedback } from "../ui";
import { useWorkspace } from "./workspace";

interface Member {
  user_id: number;
  username: string;
  role: string;
  created_at?: string;
}

export function Team() {
  const { project, projectPath, canWrite } = useWorkspace();
  const members = useResource<Member[] | null>(projectPath + "/members");
  const [invite, setInvite] = useState("");
  const { toast, confirm } = useFeedback();
  const action = useAction();
  return (
    <div className="section-stack">
      <Card title="团队成员" actions={<Users size={18} aria-hidden="true" />}>
        <ResourceNotice
          loading={members.loading && !members.data}
          error={members.error}
          retry={members.refresh}
        />
        <Table headers={["账号", "权限", "加入时间", ...(canWrite ? ["操作"] : [])]}>
          {members.data?.map((member) => (
            <tr key={member.user_id}>
              <td>{member.username}</td>
              <td>
                <Badge>{member.role === "admin" ? "项目管理员" : "只读成员"}</Badge>
              </td>
              <td>{dateLabel(member.created_at)}</td>
              {canWrite && (
                <td>
                  <Button
                    small
                    tone="danger"
                    disabled={action.busy}
                    onClick={() =>
                      void action.run(async () => {
                        if (
                          !(await confirm({
                            title: "移除项目成员",
                            message: `移除 ${member.username} 在 ${project.name} 的访问权限。`,
                            danger: true,
                            confirmText: "移除",
                          }))
                        )
                          return;
                        await request(`${projectPath}/members/${member.user_id}`, {
                          method: "DELETE",
                        });
                        members.refresh();
                      })
                    }
                  >
                    移除
                  </Button>
                </td>
              )}
            </tr>
          ))}
        </Table>
      </Card>
      {canWrite && (
        <Card title="邀请成员" description="链接有效期为 7 天。">
          <form
            className="toolbar"
            onSubmit={(event) => {
              event.preventDefault();
              const role = formText(new FormData(event.currentTarget), "role");
              void action.run(async () => {
                const result = await request<{ url: string }>(projectPath + "/invites", {
                  method: "POST",
                  body: { role, valid_hours: 168 },
                });
                setInvite(result.url);
                toast("邀请链接已生成");
              });
            }}
          >
            <Field label="项目权限">
              <select name="role" id="inviteRole" className="input">
                <option value="viewer">只读成员</option>
                <option value="admin">项目管理员</option>
              </select>
            </Field>
            <Button type="submit" tone="primary" disabled={action.busy}>
              生成邀请链接
            </Button>
          </form>
          {invite && (
            <div className="toolbar spaced">
              <input
                id="inviteUrl"
                className="input grow"
                value={invite}
                readOnly
                aria-label="邀请链接"
              />
              <Button
                small
                onClick={() =>
                  void action.run(async () => {
                    await navigator.clipboard.writeText(invite);
                    toast("邀请链接已复制");
                  })
                }
              >
                <Copy size={14} aria-hidden="true" />
                复制
              </Button>
            </div>
          )}
        </Card>
      )}
    </div>
  );
}
