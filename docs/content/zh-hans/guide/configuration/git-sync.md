# Git 远端同步

本文档说明 `controller.git` 配置。

## 配置示例

```yaml
controller:
  git:
    remote_url: "https://github.com/example/composia-services.git"
    branch: "main"
    pull_interval: "30s"
    author_name: "Composia"
    author_email: "composia@example.com"
    auth:
      username: "git"
      token_file: "/app/configs/git-token.txt"
```

## 字段说明

| 字段 | 说明 |
|------|------|
| `remote_url` | 远端 Git 仓库地址 |
| `branch` | 跟踪的分支；未填写时沿用当前本地分支 |
| `pull_interval` | 自动拉取间隔，如 `30s`、`5m`；设置 `remote_url` 后必填 |
| `author_name` | Git 提交者名称 |
| `author_email` | Git 提交者邮箱 |
| `auth.username` | 可选。配置后使用 Basic Auth，作为认证用户名 |
| `auth.token_file` | 访问令牌文件路径 |

## 认证行为

- 未配置 `auth.username` 时，Composia 会在 `git fetch` / `git push` 请求里发送 `Authorization: Bearer <token>`
- 配置 `auth.username` 后，Composia 会改为 Basic Auth，使用 `username:token` 作为凭据

## 适用场景

启用后，Controller 会把服务定义工作树和远端 Git 仓库保持同步。
