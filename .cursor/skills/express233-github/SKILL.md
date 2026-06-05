---
name: express233-github
description: >-
  GitHub CLI (gh), Actions CI/Lint gates, release policy, and Node 24 LTS for
  express233. Use when fixing CI, pushing tags, creating releases, inspecting
  workflow logs, or addressing PR checks.
---

# express233 — GitHub & CI

## 必备工具

| 工具 | 安装 | 验证 |
|------|------|------|
| **gh** | 仓库根 `install-gh.cmd` 或 `winget install GitHub.cli` | `gh auth status` |
| **golangci-lint** v2.x | `go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@latest` | `golangci-lint run` |
| **Node.js** 24 LTS | `.nvmrc` 为 `24`；Actions 用 `setup-node` + `node-version: "24"` | `node -v` |

`gh` 需 scope：`repo`、`read:org`、`workflow`（查 Actions 日志时）。

## Node.js 24（强制）

- 所有 `.github/workflows/*.yml` 设置 `FORCE_JAVASCRIPT_ACTIONS_TO_NODE24: true`。
- `test/visual` 与 Playwright workflow：**仅使用 Node 24 LTS**（`engines.node: ">=24"`）。
- 禁止在 workflow 中降级到 Node 20/22。

## CI / Lint

| Workflow | 作用 |
|----------|------|
| `ci.yml` | 三平台测试、构建、冒烟、脚本校验 |
| `lint.yml` | `golangci-lint`（`.golangci.yml` 须 `version: "2"`） |

本地对齐：

```bash
go test ./... -count=1
make smoke          # 可选
golangci-lint run --timeout=5m
```

## 发布门禁（必须）

**禁止**在 CI 或 Lint 未绿时打 tag / 发布。

1. 合并到 `main` 后等待 [CI](https://github.com/neko233-com/express233/actions/workflows/ci.yml) 与 [Lint](https://github.com/neko233-com/express233/actions/workflows/lint.yml) 全部 success。
2. 本地可选：`bash scripts/require-green-checks.sh <commit-sha>`
3. 再打 tag：`git tag v0.x.y && git push origin v0.x.y`
4. `release.yml` 首 job `verify-checks` 会再次校验该 commit 的 Check Runs，失败则**不**产出 Release。

必需 Check 名称（与 `scripts/require-green-checks.sh` 一致）：

- `golangci-lint`
- `test (ubuntu-latest)` / `test (windows-latest)` / `test (macos-latest)`
- `build binaries`
- `validate scripts`

`visual-e2e.yml` **不参与**发布门禁。

## 调试失败 CI

1. `gh auth status`
2. `gh run list --workflow lint.yml --limit 5`
3. `gh run view <id> --log-failed`
4. 修 `.golangci.yml` / 代码后 push，直到 Lint 绿。

可配合用户全局 skill：`gh-fix-ci`（`~/.codex/skills/gh-fix-ci`）、`gh-address-comments`（PR 评论）。

## PR / 推送前检查清单

- [ ] `go test ./...`
- [ ] `golangci-lint run --timeout=5m`（配置 v2）
- [ ] 若改 workflow：已含 `FORCE_JAVASCRIPT_ACTIONS_TO_NODE24`
- [ ] 若改 `test/visual`：Node 24
- [ ] 发布前：`require-green-checks.sh` 或确认 GitHub Checks 全绿

## 相关文档

- [AGENTS.md](../../AGENTS.md) — 总规范
- [docs/GITHUB_ACTIONS.md](../../docs/GITHUB_ACTIONS.md)
