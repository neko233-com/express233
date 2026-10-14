# HTTP 自动化 Demo

本例在不关联生产发布 Hook 的隔离测试租户中演示“分服配置 → 创建版本 → 上传 → 预览/差异 → 发布 → 下载”，会写入项目和版本。准备 Linux / WSL Bash、Python 3、curl、jq、tar，以及具有相应管理权限的测试账号。预览会返回文件正文，只使用虚构配置，不将生产响应贴入日志。

## 1. 从环境提供认证

由 Secret 管理系统或本机未跟踪配置注入 `EXPRESS233_SERVER`、`EXPRESS233_USERNAME`、`EXPRESS233_PASSWORD`；服务地址格式示例为 `https://express233.example.invalid`。除本机回环测试外使用 HTTPS。不要把密码、Token、Cookie 或真实线上地址写进脚本。

以下函数在内存中生成 curl 配置，经标准输入提供 URL 和 Basic Auth；命令参数只包含相对 API 路径与非敏感操作参数。不要开启 `set -x`、curl verbose/trace，或打印生成的配置。

```bash
set -euo pipefail
: "${EXPRESS233_SERVER:?inject the central URL}"
: "${EXPRESS233_USERNAME:?inject the test account}"
: "${EXPRESS233_PASSWORD:?inject the password}"
umask 077

api() {
  local api_path="$1"
  shift
  API_PATH="$api_path" python3 - <<'PY' | curl --disable --config - --fail --silent --show-error "$@"
import base64
import json
import os

base = os.environ["EXPRESS233_SERVER"].rstrip("/")
path = os.environ["API_PATH"]
if not path.startswith("/api/"):
    raise SystemExit("expected a relative API path")
credential = (os.environ["EXPRESS233_USERNAME"] + ":" +
              os.environ["EXPRESS233_PASSWORD"]).encode("utf-8")
authorization = "Authorization: Basic " + base64.b64encode(credential).decode("ascii")
print("url = " + json.dumps(base + path))
print("header = " + json.dumps(authorization))
PY
}
```

不要用 `curl -u "$SECRET"`、`--token "$TOKEN"` 或带 token 的 URL 替换此方式：环境变量展开后仍会成为进程参数。生产自动化使用最小权限账号，并单独管理认证和响应输出。

## 2. 注册演示分服配置

```bash
api /api/servers/game-a \
  -H 'Content-Type: application/json' -X PUT \
  -d '{
    "replacements": {
      "game.properties": {"server.port": "9001"},
      "application.yaml": {"spring.profiles.active": "game-a"}
    }
  }'

api /api/servers
api /api/server-ids
```

规则按文件 basename 匹配后替换键。本例没有设置 Hook，避免下载测试顺带运行游戏服脚本；正式部署的启动契约见 [SAFE_DEPLOY.md](SAFE_DEPLOY.md)。

## 3. 创建项目与两个版本

同名项目可复用已有可写项目；重复执行本例仍会创建新版本，所以应使用专门测试租户。空版本请求由服务端原子分配下一 patch，后续必须使用返回值。

```bash
PROJECT_ID=$(api /api/projects -H 'Content-Type: application/json' \
  -X POST -d '{"name":"demo-auto"}' | jq -er '.id')
VERSION_1=$(api "/api/projects/$PROJECT_ID/versions" \
  -H 'Content-Type: application/json' -X POST -d '{}' | jq -er '.version')
VERSION_2=$(api "/api/projects/$PROJECT_ID/versions" \
  -H 'Content-Type: application/json' -X POST -d '{}' | jq -er '.version')
```

## 4. 上传版本目录

使用独立临时目录，归档中仅包含演示配置：

```bash
DEMO_DIR=$(mktemp -d)
mkdir -p "$DEMO_DIR/v1/conf" "$DEMO_DIR/v2/conf"
cat >"$DEMO_DIR/v1/conf/game.properties" <<'EOF'
server.port=8080
feature.flag=base
EOF
cat >"$DEMO_DIR/v2/conf/game.properties" <<'EOF'
server.port=8181
feature.flag=blue
EOF
cat >"$DEMO_DIR/v1/conf/application.yaml" <<'EOF'
spring:
  profiles:
    active: default
EOF
cat >"$DEMO_DIR/v2/conf/application.yaml" <<'EOF'
spring:
  profiles:
    active: staging
EOF
tar -C "$DEMO_DIR/v1" -czf "$DEMO_DIR/v1.tar.gz" .
tar -C "$DEMO_DIR/v2" -czf "$DEMO_DIR/v2.tar.gz" .

api "/api/projects/$PROJECT_ID/versions/$VERSION_1/files" -F "file=@$DEMO_DIR/v1.tar.gz"
api "/api/projects/$PROJECT_ID/versions/$VERSION_2/files" -F "file=@$DEMO_DIR/v2.tar.gz"
```

上传支持单文件、`.zip`、`.tar`、`.tar.gz` / `.tgz`。真实版本包需排除数据、日志、环境文件、私钥和其它 Secret；不可直接把整个工作区不加筛选地归档。

## 5. 预览与比较

```bash
api "/api/deploy/preview?project=demo-auto&version=$VERSION_2&server_id=game-a"
api "/api/deploy/diff?project=demo-auto&from=$VERSION_1&to=$VERSION_2&server_id=game-a"
```

预览响应的 `files` 为键级摘要，`rendered_files` 为文件 before/after 全文；差异响应的 `file_diffs` 为文件级对比。本例两个版本的 `server.port` 都应被覆盖成 `9001`，`feature.flag` 则从 `base` 变为 `blue`。

这些接口预览配置，不执行部署脚本。请求里的演示名称和版本只包含 URL 安全字符；自动化传入任意名称时应使用正确的 URL 编码。

## 6. 发布并下载

```bash
api "/api/projects/$PROJECT_ID/versions/$VERSION_1/publish" -X POST
api "/api/projects/$PROJECT_ID/versions/$VERSION_2/publish" -X POST
api "/api/projects/$PROJECT_ID/versions"

api '/api/pull?project=demo-auto&server_id=game-a' -o "$DEMO_DIR/latest.tar.gz"
tar -tzf "$DEMO_DIR/latest.tar.gz"
tar -xOf "$DEMO_DIR/latest.tar.gz" conf/game.properties
tar -xOf "$DEMO_DIR/latest.tar.gz" conf/application.yaml
```

发布会执行服务端校验；如果项目配置了审批，先按审批流程取得批准，不绕过失败。若项目存在已启用的版本发布 Hook，发布还会触发对应任务，因此测试项目应不绑定任何生产 Hook。下载使用同一账号的 Basic Auth，不读取管理员拉取 Token，不把 Token 拼进 URL。

预期结果为 `server.port=9001`、`feature.flag=blue`、`spring.profiles.active: game-a`。仅下载及查看归档不会执行 Hook。

CLI 也可以读取上述账号环境变量；使用前确认本机 CLI 配置没有遗留 Token（Token 优先于账号认证，不要用 `config show` 向日志泄露它）。拉取只落到新暂存目录：

```bash
export EXPRESS233_PROJECT=demo-auto
CLI_STAGE=$(mktemp -d)
express233-cli pull --server-id game-a --dest "$CLI_STAGE" --skip-hook --retries 5
```

CLI 默认最多尝试下载 3 次，完整下载后才开始解压；`--retries 5` 改为最多 5 次。解压仍是直接写目标目录，并非原子换包，失败时可能留下部分文件。`deploy` 与 `pull` 是别名，二者都只有加 `--skip-hook` 才会跳过包内 Hook。运行中的游戏服必须走安全部署脚本。

## 7. 配置维护

仅清理本例的演示分服条目时：

```bash
api /api/servers/game-a -X DELETE
```

整文件配置接口仍可用，但会影响该租户多个分服；先读取并核对，再在明确需要时写入。本例只展示将响应保存到受保护临时目录，避免把生产配置直接输出：

```bash
api /api/server-yaml -o "$DEMO_DIR/server-yaml.json"
# 维护者审阅 server-yaml-update.json 后才执行：
# api /api/server-yaml -H 'Content-Type: application/json' -X PUT -d @server-yaml-update.json
```

完成后按测试环境的清理策略处理演示项目和临时目录，不使用真实业务项目做删除练习。SSH 推送的目标预览、幂等键与执行结果见 [SSH_PUSH_DEPLOY.md](SSH_PUSH_DEPLOY.md)。
