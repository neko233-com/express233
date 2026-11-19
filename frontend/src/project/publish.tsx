import { Check, CircleAlert } from "lucide-react";
import type { Validation } from "../types";
import { Badge, Button, Card, Empty } from "../ui";
import { useWorkspace } from "./workspace";

export function ValidationResult({
  result,
  checking,
  testId,
}: {
  result?: Validation;
  checking?: boolean;
  testId: string;
}) {
  return (
    <div role="status" aria-live="polite" data-testid={testId}>
      {checking ? (
        <p className="hint">正在检查版本文件…</p>
      ) : (
        result && (
          <div className={`validation ${result.ok ? "success" : "error"}`}>
            <div>
              {result.ok ? <Check size={17} /> : <CircleAlert size={17} />}
              <strong>{result.ok ? "可以发布，文件检查通过" : "暂时无法发布"}</strong>
            </div>
            {result.errors?.map((error) => (
              <p key={error}>{error}</p>
            ))}
            {result.warnings?.map((warning) => (
              <p className="hint" key={warning}>
                {warning}
              </p>
            ))}
          </div>
        )
      )}
    </div>
  );
}

export function Publish() {
  const {
    project,
    version,
    confirmed,
    target,
    busy,
    busyKind,
    validation,
    validate,
    publish,
    canWrite,
    open,
  } = useWorkspace();
  if (!version)
    return (
      <Card title="发布版本包">
        <Empty>先在版本包工作区选择一个版本。</Empty>
        <Button onClick={() => open("versions")}>选择版本</Button>
      </Card>
    );
  const published = version.status === "published";
  return (
    <div className="two-col publish-grid" data-testid="publish-flow-panel">
      <Card title="发布版本包" description="检查制品与配置，然后锁定版本。">
        <dl className="checklist">
          <div>
            <dt>版本</dt>
            <dd id="publishVersionCheck">{version.version}</dd>
          </div>
          <div>
            <dt>分服配置</dt>
            <dd id="publishConfigCheck">
              {target ? `当前配置：${target}` : "按需配置 · 无分服差异时可跳过"}
            </dd>
          </div>
          <div>
            <dt>配置预览</dt>
            <dd id="publishPreviewCheck">
              {confirmed ? "配置预览已确认" : "可选 · 检查替换后的文件"}
            </dd>
          </div>
        </dl>
        <ValidationResult
          result={validation}
          checking={busyKind === "validate"}
          testId="flow-validate-result"
        />
      </Card>
      <Card title="确认版本">
        <dl className="checklist">
          <div>
            <dt>项目</dt>
            <dd>{project.name}</dd>
          </div>
          <div>
            <dt>版本</dt>
            <dd>
              <code>{version.version}</code>
            </dd>
          </div>
          <div>
            <dt>状态</dt>
            <dd>
              <Badge kind={published ? "ok" : "neutral"}>{version.status}</Badge>
            </dd>
          </div>
        </dl>
        <p className="hint">
          {published
            ? "已发布版本可供目标拉取和部署。"
            : "发布后锁定文件，并触发已开启的自动发布规则。"}
        </p>
        <div className="toolbar">
          {!published && (
            <Button id="btnFlowValidate" disabled={busy} onClick={() => void validate()}>
              发布前检查
            </Button>
          )}
          {!published && canWrite && (
            <Button
              id="btnFlowPublish"
              tone="primary"
              disabled={busy || version.status === "rejected" || validation?.ok === false}
              onClick={() => void publish()}
            >
              {busyKind === "publish" ? "正在发布…" : "正式发布"}
            </Button>
          )}
          {published && (
            <Button id="btnPublishedDeploy" tone="primary" onClick={() => open("delivery")}>
              进入部署
            </Button>
          )}
        </div>
      </Card>
    </div>
  );
}
