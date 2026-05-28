---
title: "镜像更新"
date: '2026-05-26T00:00:00+08:00'
weight: 60
---

Composia 检测新的镜像标签并可以自动应用更新。镜像检查任务在 agent 上运行，并将发现结果上报给控制器。

## 工作原理

控制器根据服务的更新配置调度定期的 `image_check` 任务。每次检查：

1. Agent 下载服务包。
2. 读取 `docker compose config --format json` 以发现正在运行的镜像。
3. 上报每个镜像的本地和远程摘要。
4. 对于在 `update.images` 中配置的镜像，使用配置的发现源检查新的候选标签。
5. 将结果上报给控制器。控制器记录可用的更新并可自动应用它们。

## 控制器默认值

全局默认值在控制器配置中设置：

```yaml
controller:
  updates:
    default_check_schedule: "0 */6 * * *"
    auto_apply: false
    backup_before_update: true
    digest_pin: false
    semver:
      default_allow:
        - patch
        - minor
    forge_auth:
      github:
        url: "https://github.com"
        token: "REPLACE"
        api_url: "https://api.github.com"
```

服务级别的 `update` 部分会覆盖这些默认值。

## 服务配置

```yaml
update:
  enabled: true
  auto_apply: false
  check_schedule: "0 */6 * * *"
  backup_before_update: true
  digest_pin: false
  backup_data:
    - name: db
      enabled: true
  discovery_sources:
    upstream-gh:
      sources:
        - type: github
          repo: owner/repo
      combine: first_success
      include_prerelease: false
  images:
    api:
      image: ghcr.io/example/api
      current:
        env:
          file: .env
          key: API_VERSION
      discovery: upstream-gh
      filter:
        type: semver
        allow:
          - patch
          - minor
```

### `update` 顶级键

| 键 | 类型 | 描述 |
|-----|------|-------------|
| `enabled` | `bool` | 为此服务启用更新检查。 |
| `auto_apply` | `bool` | 自动应用检测到的更新。 |
| `check_schedule` | `string` | 更新检查的 cron 计划。 |
| `backup_before_update` | `bool` | 在应用更新之前运行备份。 |
| `backup_data` | `[]object` | 更新前要备份的受保护数据项。每个项有 `name` 和可选的 `enabled`。 |
| `digest_pin` | `bool` | 通过摘要锁定镜像以提高可重复性。 |
| `discovery_sources` | `map[string]object` | 命名的可复用发现配置。 |
| `images` | `map[string]object` | 每镜像更新配置。键是与要检查的镜像匹配的任意名称。 |

### `images.<name>`

| 键 | 类型 | 必填 | 描述 |
|-----|------|----------|-------------|
| `image` | `string` | 是 | 完整镜像引用，例如 `ghcr.io/example/api`。 |
| `auto_apply` | `bool` | 否 | 每镜像自动应用覆盖。 |
| `check_schedule` | `string` | 否 | 每镜像检查计划。 |
| `backup_before_update` | `bool` | 否 | 每镜像备份开关。 |
| `digest_pin` | `bool` | 否 | 每镜像摘要锁定开关。 |
| `current` | `object` | 是 | 如何找到当前部署的版本。 |
| `discovery` | `object` 或 `string` | 是 | 发现配置或对命名 `discovery_sources` 条目的引用。 |
| `filter` | `object` | 条件 | 版本过滤器。除非发现模式为 `digest`，否则必填。 |

### `current`

必须指定以下来源之一：

**静态标签：**

```yaml
current:
  tag: "v1.2.3"
```

**环境文件：**

```yaml
current:
  env:
    file: .env
    key: APP_VERSION
```

`file` 路径相对于服务目录。Composia 读取文件，查找 `KEY=VALUE` 行并提取值。

**YAML 文件：**

```yaml
current:
  yaml:
    file: values.yaml
    path: app.image.tag
```

`path` 是进入 YAML 文档树的点分隔路径。该路径处的值必须为标量。

### 发现

发现源可以是：

**命名引用**，指向 `discovery_sources` 条目：

```yaml
discovery: upstream-gh
```

**内联定义：**

```yaml
discovery:
  sources:
    - type: probe
  combine: first_success
  include_prerelease: false
```

发现源类型：

| 类型 | 必填键 | 行为 |
|------|---------------|----------|
| `probe` | 无 | 语义化版本探测：通过探测注册表清单搜索更高版本。需要 `semver` 过滤器。 |
| `registry` | 无 | 列出镜像注册表中的所有标签。 |
| `auto` | 无（可选 `repo_url`） | 作为合并发现，先尝试 `probe` 再尝试 `registry`。必须是发现配置中的唯一源。 |
| `digest` | 无 | 仅比较远程摘要与本地摘要。不进行标签比较。必须省略 `filter`。必须是唯一源。 |
| `github` | `repo`（`owner/repo`） | 查询 GitHub 发布。在控制器端处理。 |
| `gitlab` | `project` | 查询 GitLab 发布。在控制器端处理。 |
| `forgejo` | `repo`（`owner/repo`） | 查询 Forgejo 发布。在控制器端处理。 |

`combine` 接受 `merge`（所有源结果的并集）或 `first_success`（第一个返回结果的源胜出）。

`include_prerelease` 在 GitHub、GitLab 和 Forgejo 发布查询中包含预发布版本。

### 过滤器

| 类型 | 必填键 | 行为 |
|------|---------------|----------|
| `semver` | 无 | 按语义化版本过滤。`allow` 可包含 `patch`、`minor`、`major`。 |
| `date` | `format` | 使用给定的格式将标签解析为日期。 |
| `regex` | `pattern`、`order` | 按正则表达式过滤。`order` 必须为 `numeric` 或 `lexicographic`。 |
| `latest` | 无 | 取最新标签，不进行过滤。 |

#### 语义化版本探测

使用 `type: probe` 和 `semver` 过滤器时，Composia 通过构造版本号并检查相应注册表清单是否存在来搜索候选标签。它根据 `allow` 列表探测补丁（patch）、次版本（minor）和主版本（major）的升级，使用指数搜索配合二分法精化来找到最高可用版本。

## 摘要模式

当配置中的所有发现源类型均为 `digest` 时，不执行标签比较。Composia 仅将远程镜像摘要与本地摘要进行比较：

```yaml
discovery:
  sources:
    - type: digest
```

当发现模式设置为 `digest` 时，必须省略 `filter`。如果摘要不同，则认为存在可用更新。

## 镜像观测

在部署和更新任务期间，agent 还会为所有 compose 服务收集镜像观测信息。这些信息包括本地和远程摘要，无论是否配置了 `update.images` 都会上报给控制器。这提供了 Web UI 和 CLI 中的镜像状态可见性。
