# HTTP 自动化 Demo

以下示例演示完整自动化运维链路：

1. 用 `root/root` 直接走 HTTP Basic Auth
2. 注册 `server_id` 与替换规则
3. 创建项目与两个版本
4. 上传整个版本目录（`tar.gz`）
5. 预览替换结果与版本 diff
6. 发布并拉取最新版本

假设服务地址为 `http://127.0.0.1:23380`。

## 1. 准备认证

```bash
export EXPRESS233_SERVER=http://127.0.0.1:23380
export EXPRESS233_BASIC_AUTH='root:root'
```

## 2. 注册 server_id 与替换模板

```bash
curl --fail -u "$EXPRESS233_BASIC_AUTH" \
  -H 'Content-Type: application/json' \
  -X PUT "$EXPRESS233_SERVER/api/servers/game-a" \
  -d '{
    "replacements": {
      "game.properties": {
        "server.port": "9001"
      },
      "application.yaml": {
        "spring.profiles.active": "game-a"
      }
    },
    "post_hook": "scripts/reload-{{SERVER_ID}}.sh",
    "post_hook_env": {
      "ZONE": "demo"
    }
  }'
```

列出当前 server 条目：

```bash
curl --fail -u "$EXPRESS233_BASIC_AUTH" "$EXPRESS233_SERVER/api/servers"
curl --fail -u "$EXPRESS233_BASIC_AUTH" "$EXPRESS233_SERVER/api/server-ids"
```

## 3. Get or create 项目，并自动创建两个版本

```bash
PROJECT_ID=$(curl --fail -u "$EXPRESS233_BASIC_AUTH" \
  -H 'Content-Type: application/json' \
  -X POST "$EXPRESS233_SERVER/api/projects" \
  -d '{"name":"demo-auto"}' | jq -r '.id')

VERSION_1=$(curl --fail -u "$EXPRESS233_BASIC_AUTH" \
  -H 'Content-Type: application/json' \
  -X POST "$EXPRESS233_SERVER/api/projects/$PROJECT_ID/versions" \
  -d '{}' | jq -r '.version')

VERSION_2=$(curl --fail -u "$EXPRESS233_BASIC_AUTH" \
  -H 'Content-Type: application/json' \
  -X POST "$EXPRESS233_SERVER/api/projects/$PROJECT_ID/versions" \
  -d '{}' | jq -r '.version')

echo "created versions: $VERSION_1 -> $VERSION_2"
```

## 4. 上传整个版本目录

准备两个版本目录：

```bash
mkdir -p /tmp/express233-demo/v1/conf /tmp/express233-demo/v2/conf

cat >/tmp/express233-demo/v1/conf/game.properties <<'EOF'
server.port=8080
feature.flag=base
EOF

cat >/tmp/express233-demo/v1/conf/application.yaml <<'EOF'
spring:
  profiles:
    active: default
EOF

cat >/tmp/express233-demo/v2/conf/game.properties <<'EOF'
server.port=8181
feature.flag=blue
EOF

cat >/tmp/express233-demo/v2/conf/application.yaml <<'EOF'
spring:
  profiles:
    active: staging
EOF

tar -C /tmp/express233-demo/v1 -czf "/tmp/express233-demo-${VERSION_1}.tar.gz" .
tar -C /tmp/express233-demo/v2 -czf "/tmp/express233-demo-${VERSION_2}.tar.gz" .
```

上传整包：

```bash
curl --fail -u "$EXPRESS233_BASIC_AUTH" \
  -F "file=@/tmp/express233-demo-${VERSION_1}.tar.gz" \
  "$EXPRESS233_SERVER/api/projects/$PROJECT_ID/versions/$VERSION_1/files"

curl --fail -u "$EXPRESS233_BASIC_AUTH" \
  -F "file=@/tmp/express233-demo-${VERSION_2}.tar.gz" \
  "$EXPRESS233_SERVER/api/projects/$PROJECT_ID/versions/$VERSION_2/files"
```

## 5. 预览替换结果与版本差异

预览 `$VERSION_2` 在 `game-a` 上的最终替换效果：

```bash
curl --fail -u "$EXPRESS233_BASIC_AUTH" \
  "$EXPRESS233_SERVER/api/deploy/preview?project=demo-auto&version=$VERSION_2&server_id=game-a"
```

对比两个版本在 `game-a` 上的有效配置差异：

```bash
curl --fail -u "$EXPRESS233_BASIC_AUTH" \
  "$EXPRESS233_SERVER/api/deploy/diff?project=demo-auto&from=$VERSION_1&to=$VERSION_2&server_id=game-a"
```

这里会看到两类变化：

- `game.properties.server.port` 最终都会变成 `9001`，所以不应再被报告为版本差异
- `feature.flag` 会从 `base` 变成 `blue`

## 6. 发布并拉取

```bash
curl --fail -u "$EXPRESS233_BASIC_AUTH" \
  -X POST "$EXPRESS233_SERVER/api/projects/$PROJECT_ID/versions/$VERSION_1/publish"

curl --fail -u "$EXPRESS233_BASIC_AUTH" \
  -X POST "$EXPRESS233_SERVER/api/projects/$PROJECT_ID/versions/$VERSION_2/publish"
```

获取 root 的拉取 token：

```bash
PULL_TOKEN=$(curl --fail -u "$EXPRESS233_BASIC_AUTH" "$EXPRESS233_SERVER/api/users" | jq -r '.[] | select(.username=="root") | .token')
```

查看已发布版本：

```bash
curl --fail "$EXPRESS233_SERVER/api/pull/versions?token=$PULL_TOKEN&project=demo-auto"
```

拉取最新已发布版本：

```bash
curl --fail -o /tmp/demo-auto-latest.tar.gz \
  "$EXPRESS233_SERVER/api/pull?token=$PULL_TOKEN&project=demo-auto&server_id=game-a"

tar -xOf /tmp/demo-auto-latest.tar.gz ./conf/game.properties
tar -xOf /tmp/demo-auto-latest.tar.gz ./conf/application.yaml
```

应能看到：

- `server.port=9001`
- `feature.flag=blue`
- `spring.profiles.active: game-a`

## 7. 细粒度 CRUD

删除单个 server_id：

```bash
curl --fail -u "$EXPRESS233_BASIC_AUTH" -X DELETE "$EXPRESS233_SERVER/api/servers/game-a"
```

仍可保留整文件方式：

```bash
curl --fail -u "$EXPRESS233_BASIC_AUTH" "$EXPRESS233_SERVER/api/server-yaml"
curl --fail -u "$EXPRESS233_BASIC_AUTH" \
  -H 'Content-Type: application/json' \
  -X PUT "$EXPRESS233_SERVER/api/server-yaml" \
  -d @server-yaml-update.json
```
