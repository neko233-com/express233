# 发布脚本契约

express233 控制台只负责任务选择、请求幂等、全局串行、SSH 调度和不可变日志。游戏服仓库自己的发布脚本负责进程识别、优雅停服、换包、启动与健康检查。`scripts/safe-deploy.sh` 是通用参考实现，不包含某个游戏服的业务退出逻辑。

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
→ 停服前准备回滚包
→ 获取 project|serverId 独占锁
→ 再次检查幂等状态
→ SIGTERM
→ 高频轮询，等待进程正常退出
→ 快速换包
→ 启动
→ 健康门禁
→ 原子提交完成标记
```

备份、下载、解压、模板渲染、配置校验都必须在 SIGTERM 前完成。对 `111` 这类正在运行的逻辑服，脚本应发送 SIGTERM 后等待游戏服保存玩家数据并正常退出；默认超时应在换包前失败，不能直接覆盖运行中的二进制。是否允许超时强杀由游戏服仓库显式决定。

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
prepare_rollback_before_downtime

with_target_lock "$EXPRESS233_TARGET_TAG" {
  already_healthy "$resolved_digest" && exit 0
  send_sigterm
  wait_until_process_exits_normally
  swap_release_tree
  start_release
  health_gate
  commit_state_atomically "$resolved_digest" "$EXPRESS233_DEPLOYMENT_ID"
}
```

实际 PID 位置、systemd 单元、玩家保存完成条件和健康接口由游戏服仓库维护。控制台不推断这些业务细节。
