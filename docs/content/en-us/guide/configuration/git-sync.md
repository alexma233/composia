# Git Remote Sync

This page documents the `controller.git` configuration.

## Example

```yaml
controller:
  git:
    remote_url: "https://github.com/example/composia-services.git"
    branch: "main"
    pull_interval: "30s"
    author_name: "Composia"
    author_email: "composia@example.com"
    auth:
      token_file: "/app/configs/git-token.txt"
```

## Fields

| Field | Description |
|-------|-------------|
| `remote_url` | Remote Git repository URL |
| `branch` | Branch to track; if omitted, the controller keeps using the current local branch |
| `pull_interval` | Auto-pull interval such as `30s` or `5m`; required when `remote_url` is set |
| `author_name` | Git committer name |
| `author_email` | Git committer email |
| `auth.token_file` | Path to the access token file |

## Behavior

When enabled, the Controller keeps the service definition working tree synchronized with the remote Git repository.
