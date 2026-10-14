# 发布脚本契约

express233 控制台负责任务选择、请求幂等、全局串行、SSH 调度和保留期内不可手工删除的发布日志。游戏服仓库自己的发布脚本负责进程识别、优雅停服、换包、启动与健康检查。`scripts/safe-deploy.sh` 是通用参考实现，不包含某个游戏服的业务退出逻辑。`express233-cli pull` 和 `deploy` 直接解压并默认执行 Hook，不能替代此契约；暂存拉取必须加 `--skip-hook`。

## 控制台提供的上下文

正式 SSH 发布会向目标脚本传入：

| 环境变量 | 含义 |
|---|---|
| `EXPRESS233_DEPLOYMENT_ID` | 控制台执行 ID；用于日志关联 |
| `EXPRESS233_IDEMPOTENCY_KEY` | 本次请求幂等键；网络重试保持不变 |
| `EXPRESS233_TARGET_TAG` | 唯一目标，格式 `project|serverId` |
| `EXPRESS233_PROJECT` | 项目名 |
| `EXPRESS233_SERVER_ID` | 目标 serverId |
| `VERSION` | 已解析的发布版本 |
| `EXPRESS233_SERVER` | 控制台地址 |
| `EXPRESS233_TOKEN` | 短期拉取凭据；不得写日志或状态文件 |
| `EXPRESS233_TAGS` | 内容筛选标签，逗号分隔 |
| `GAME_ROOT` | 目标机数据根目录 |

控制台用标准输入向远端 `sh -s` 发送命令，凭据不会出现在远端 shell 的 argv。用户脚本仍不得执行 `set -x`、打印完整环境或保存 Token。

控制台根据绑定的 `skip_backup` 决定是否向安全脚本传 `--backup`；安全脚本调用 `scripts/restart.sh` 时进一步注入 `EXPRESS233_SKIP_BACKUP=true/false`。下游脚本应遵守该选择，不能在用户明确跳过旧备份时另外复制旧二进制、数据或配置。可跟踪的节点配置只保存账号名和 Secret 引用，Secret 值由运行环境或未跟踪配置提供；当前 API 并没有 `password_ref` / `private_key_ref` 字段，不能把引用名当成密码传入。

## 幂等规则

1. 一次用户点击生成一个 `Idempotency-Key`。HTTP/网络重试复用该键，控制台返回原执行，不创建第二条任务。
2. 用户明确“再次发布”生成新键，可以重新执行相同版本。
3. 目标脚本应以 `EXPRESS233_TARGET_TAG` 获取独占锁。锁等待期间不得停止旧服务。
4. 拉取完成后计算“解析后内容”指纹。只有“指纹相同、PID/服务存活、健康检查通过”三项同时成立，才可直接返回幂等成功。
5. 完成标记只能在新进程启动并通过健康检查后原子写入。不能仅凭版本号跳过，因为同版本的 serverId 覆盖配置可能变化。

建议状态键：

```text
target_tag + resolved_content_sha256
```

## 最短停机顺序

```text
拉取到独立临时目录
→ 静态校验与内容指纹
→ 获取 project|serverId 独占锁
→ 再次检查幂等状态
→ 合并需要保留的本地配置，按策略准备回滚包
→ 校验 PID / 服务身份
→ SIGTERM
→ 高频轮询，等待进程正常退出
→ 快速换包
→ 启动
→ 健康门禁
→ 原子提交完成标记
```

备份、下载、解压、模板渲染、配置校验都应在 SIGTERM 前完成；修改目标目录或归档旧发布文件须持有目标锁。正在运行的逻辑服应收到 SIGTERM 后保存玩家数据并正常退出；默认超时应在换包前失败，不能直接覆盖运行中的二进制。正常发布禁止 SIGKILL，强杀只能用于人工明确授权的应急处置。

通用脚本按 `GAME_ROOT/server_id` 隔离锁和运行目录，并不把项目名放入路径；同机不同项目必须使用不同 `GAME_ROOT` 或不同 `server_id`。PID 文件必须验证为目标进程，不能只凭数字发送信号。受 systemd 等管理的实例还要协调自动重启和超时策略。

换包和回滚必须保留实时 `data/`、`logs/`、`run/`、`.env`；通用脚本的 rsync 分支实现了这些排除规则，因此生产目标应安装 rsync。新包 `config/` 的同名文件优先，旧目录独有配置补入暂存。停服前的普通 tar 不是在线数据库一致性快照，不能自动回退玩家数据。没有回滚源时应报告失败，不能宣称已恢复。

## 脚本返回约定

| 退出码 | 含义 |
|---|---|
| `0` | 发布成功，或健康的相同内容幂等命中 |
| `75` | 目标正被其他发布占用，可稍后重试 |
| 其他非零 | 发布失败；输出会进入逐服务器日志 |

脚本输出应包含阶段名，但不得包含密码、Token、私钥或数据库完整连接串。

## 参考伪代码

```bash
stage_and_validate
resolved_digest="$(digest_stage)"

with_target_lock "$EXPRESS233_TARGET_TAG" {
  already_healthy "$resolved_digest" && exit 0
  merge_preserved_config
  prepare_rollback_if_enabled
  verify_process_identity
  send_sigterm
  wait_until_process_exits_normally
  swap_release_tree
  start_release
  health_gate
  commit_state_atomically "$resolved_digest" "$EXPRESS233_DEPLOYMENT_ID"
}
```

以上是接口伪代码，不可直接作为 Bash 执行。实际 PID 位置、systemd 单元、玩家保存完成条件和健康接口由游戏服仓库维护。控制台不推断这些业务细节。通用脚本的健康脚本是可选项；缺少它时，启动脚本返回成功不代表游戏客户端已经可用。
