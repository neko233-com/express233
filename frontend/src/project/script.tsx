import { useState } from "react";
import { Copy, Download } from "lucide-react";
import canonicalDeploy from "../../../scripts/safe-deploy.sh?raw";
import { Code } from "../code";
import { Badge, Button, Card, Field, useAction, useFeedback } from "../ui";
import { ServerID } from "./preview";
import { useWorkspace } from "./workspace";

const serverIdPattern = /^[A-Za-z0-9][A-Za-z0-9._-]{0,127}$/;
const bash = (value: string) => "'" + value.replaceAll("'", "'\"'\"'") + "'";
const powershell = (value: string) => "'" + value.replaceAll("'", "''") + "'";

export function DeployScript() {
  const { project, version, target, serverIDs } = useWorkspace();
  const [os, setOS] = useState<"linux" | "windows">("linux");
  const [server, setServer] = useState(target);
  const [root, setRoot] = useState("");
  const [dryRun, setDryRun] = useState(true);
  const action = useAction();
  const { toast } = useFeedback();
  const valid = serverIdPattern.test(server.trim());
  const id = valid ? server.trim() : "server-id";
  const linux = os === "linux";
  const filename = `express233-${id}.${linux ? "sh" : "ps1"}`;
  const env = {
    EXPRESS233_SERVER: window.location.origin,
    EXPRESS233_PROJECT: project.name,
    EXPRESS233_SERVER_ID: id,
    VERSION: version?.version ?? "",
    ...(root && linux ? { GAME_ROOT: root } : {}),
  };

  const source = linux
    ? [
        "#!/usr/bin/env bash",
        "# express233-cli pull 使用环境变量中的拉取凭据。",
        ...Object.entries(env).map(([name, value]) => `export ${name}=${bash(value)}`),
        "export EXPRESS233_FORCE_KILL_AFTER_TIMEOUT=0",
        `set -- --backup${dryRun ? " --dry-run" : ""} "$@"`,
        canonicalDeploy.replace(/^#![^\r\n]*\r?\n/, ""),
      ].join("\n")
    : [
        "# PowerShell 7 · 将制品拉取到独立暂存目录。",
        "$ErrorActionPreference = 'Stop'",
        ...Object.entries(env).map(([name, value]) => `$env:${name} = ${powershell(value)}`),
        "$staging = Join-Path ([IO.Path]::GetTempPath()) ('express233-' + [Guid]::NewGuid().ToString('N'))",
        `& express233-cli pull --dest $staging --skip-hook${version ? ` --version ${powershell(version.version)}` : ""}`,
        "if ($LASTEXITCODE -ne 0) { throw 'pull failed' }",
        "Write-Output $staging",
        "",
      ].join("\n");

  function download() {
    const url = URL.createObjectURL(new Blob([source], { type: "text/plain;charset=utf-8" }));
    const anchor = document.createElement("a");
    anchor.href = url;
    anchor.download = filename;
    anchor.click();
    setTimeout(() => URL.revokeObjectURL(url), 1000);
  }

  return (
    <Card
      title="部署脚本"
      description="在目标主机安装 express233-cli，并通过环境变量提供拉取凭据。"
    >
      <nav className="segments" aria-label="操作系统">
        <Button
          small
          tone={linux ? "primary" : "ghost"}
          aria-pressed={linux}
          onClick={() => setOS("linux")}
        >
          Linux (.sh)
        </Button>
        <Button
          small
          tone={linux ? "ghost" : "primary"}
          aria-pressed={!linux}
          onClick={() => setOS("windows")}
        >
          Windows (.ps1)
        </Button>
      </nav>
      <div className="toolbar spaced">
        <Field label="server_id">
          <ServerID id="deployServerId" ids={serverIDs} value={server} setValue={setServer} />
        </Field>
        {linux && (
          <Field label="实例根目录">
            <input
              className="input"
              id="deployTempDir"
              value={root}
              placeholder="/opt/game-servers"
              onChange={(event) => setRoot(event.currentTarget.value)}
            />
          </Field>
        )}
        {linux && (
          <label className="check-field">
            <input
              type="checkbox"
              checked={dryRun}
              onChange={(event) => setDryRun(event.currentTarget.checked)}
            />
            预演
          </label>
        )}
        {!linux && <Badge kind="info">拉取到暂存目录</Badge>}
      </div>
      <p className="hint">
        {linux
          ? "Linux 脚本使用仓库的安全部署流程。"
          : "Windows 脚本生成独立暂存目录，供节点的部署流程取用。"}
      </p>
      <Code text={source} path={filename} testId="deploy-cmd" />
      <div className="toolbar spaced">
        <Button
          small
          tone="primary"
          disabled={!valid || action.busy}
          onClick={() =>
            void action.run(async () => {
              await navigator.clipboard.writeText(source);
              toast("部署脚本已复制");
            })
          }
        >
          <Copy size={15} aria-hidden="true" />
          复制脚本
        </Button>
        <Button small disabled={!valid} onClick={download}>
          <Download size={15} aria-hidden="true" />
          下载文件
        </Button>
      </div>
    </Card>
  );
}
