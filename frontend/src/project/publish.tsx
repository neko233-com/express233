import { Check, CircleAlert } from "lucide-react";
import { Link } from "react-router";
import type { Validation } from "../types";
import { Button, Card, Empty, Field, ResourceNotice, VersionBadge } from "../ui";
import { useWorkspace } from "./workspace";

function ValidationResult({
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
    versions,
    version,
    latest,
    missingVersion,
    versionsLoading,
    files,
    filesLoading,
    filesError,
    refreshFiles,
    confirmed,
    target,
    busy,
    busyKind,
    validation,
    validate,
    publish,
    review,
    canRetryPublish,
    canWrite,
    panelLink,
    selectVersion,
  } = useWorkspace();
  const published = version?.status === "published";
  const allowPublish = version?.status === "draft" || version?.status === "pending_review";
  const blocked = busy || versionsLoading;
  return (
    <div className="publish-flow" data-testid="publish-flow-panel">
      <Card title="发布版本包" actions={version && <VersionBadge status={version.status} />}>
        {versions.length > 0 && (
          <div className="toolbar publish-version-select">
            <Field label="版本">
              <select
                id="publishVersionCheck"
                className="input"
                value={version?.version ?? ""}
                disabled={blocked}
                onChange={(event) => selectVersion(event.currentTarget.value)}
              >
                {!version && (
                  <option value="" disabled>
                    选择版本
                  </option>
                )}
                {versions.map((row) => (
                  <option key={row.id} value={row.version}>
                    {row.version}
                  </option>
                ))}
              </select>
            </Field>
            <Link className="muted-link" to={panelLink("versions")}>
              查看版本文件
            </Link>
          </div>
        )}
        {!version ? (
          <Empty>
            {missingVersion
              ? `版本 ${missingVersion} 已删除或不可用。`
              : "请先在版本包工作区创建版本。"}{" "}
            {!versions.length && <Link to={panelLink("versions")}>前往版本包</Link>}
          </Empty>
        ) : (
          <>
            <dl className="checklist">
              <div>
                <dt>版本文件</dt>
                <dd>
                  <ResourceNotice loading={filesLoading} error={filesError} retry={refreshFiles} />
                  {!filesLoading && !filesError && `${files.length} 个文件`}
                </dd>
              </div>
              <div>
                <dt>分服配置</dt>
                <dd id="publishConfigCheck">
                  <Link to={panelLink("config")}>{target || "按需设置"}</Link>
                </dd>
              </div>
              <div>
                <dt>配置预览</dt>
                <dd id="publishPreviewCheck">
                  <Link to={panelLink("preview")}>
                    {confirmed ? `${target} 已确认` : "查看预览（可选）"}
                  </Link>
                </dd>
              </div>
            </dl>
            <div data-testid="validate-result">
              <ValidationResult
                result={published ? undefined : validation}
                checking={busyKind === "validate"}
                testId="flow-validate-result"
              />
            </div>
            <p className="hint spaced">
              {published ? (
                "已发布版本可供目标拉取和部署。"
              ) : (
                <>
                  发布会锁定版本文件，并通知已启用的{" "}
                  <Link to={panelLink("automation", { section: "hooks" })}>Hook</Link> 和自动跟随的{" "}
                  <Link to={panelLink("cluster")}>Pull 节点</Link>。
                </>
              )}
            </p>
            <div className="toolbar">
              {!published && (
                <Button
                  id="btnFlowValidate"
                  data-testid="validate-version"
                  disabled={blocked}
                  onClick={() => void validate()}
                >
                  {busyKind === "validate" ? "正在检查…" : "发布前检查"}
                </Button>
              )}
              {canWrite && (version.status === "draft" || version.status === "rejected") && (
                <Button disabled={blocked} onClick={() => void review("submit-review")}>
                  提交审批
                </Button>
              )}
              {canWrite && version.status === "pending_review" && (
                <Button tone="danger" disabled={blocked} onClick={() => void review("reject")}>
                  驳回
                </Button>
              )}
              {!published && canWrite && (
                <Button
                  id="btnFlowPublish"
                  data-testid="publish-version"
                  tone="primary"
                  disabled={blocked || !allowPublish || validation?.ok === false}
                  onClick={() => void publish()}
                >
                  {busyKind === "publish" ? "正在发布…" : "正式发布"}
                </Button>
              )}
              {published && (
                <Link
                  id="btnPublishedDeploy"
                  className="btn btn-primary"
                  to={panelLink(version === latest ? "delivery" : "automation")}
                >
                  {version === latest ? "进入部署" : "历史版本部署"}
                </Link>
              )}
              {canRetryPublish && canWrite && (
                <Button disabled={blocked} onClick={() => void publish()}>
                  重试发布通知
                </Button>
              )}
            </div>
          </>
        )}
      </Card>
    </div>
  );
}
