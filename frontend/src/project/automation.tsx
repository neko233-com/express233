import { Link, useNavigate, useSearchParams } from "react-router";
import { useResource } from "../api";
import { useFeedback } from "../ui";
import { deploymentFeedback, type Deployment, type PushTask } from "./delivery-api";
import { TaskEditor } from "./publish-tasks";
import { HookEditor } from "./release-hooks";
import { useWorkspace } from "./workspace";

export function Automation() {
  const { projectPath, panelLink } = useWorkspace();
  const [search] = useSearchParams();
  const showHooks = search.get("section") === "hooks";
  const navigate = useNavigate();
  const { toast } = useFeedback();
  const tasks = useResource<PushTask[] | null>(projectPath + "/push-tasks");

  function showRun(deployment: Deployment) {
    toast(deploymentFeedback(deployment));
    void navigate(panelLink("delivery", { run: deployment.id }));
  }

  return (
    <div className="section-stack" data-testid="automation-flow-panel">
      <nav className="workspace-subnav" aria-label="高级发布">
        <Link
          className="btn btn-sm"
          to={panelLink("automation")}
          aria-current={showHooks ? undefined : "page"}
        >
          发布任务
        </Link>
        <Link
          className="btn btn-sm"
          to={panelLink("automation", { section: "hooks" })}
          aria-current={showHooks ? "page" : undefined}
        >
          自动 Hook
        </Link>
      </nav>
      {showHooks ? (
        <HookEditor tasks={tasks} />
      ) : (
        <TaskEditor
          tasks={tasks.data ?? []}
          error={tasks.error}
          loading={tasks.loading}
          refresh={tasks.refresh}
          onRun={showRun}
        />
      )}
    </div>
  );
}
