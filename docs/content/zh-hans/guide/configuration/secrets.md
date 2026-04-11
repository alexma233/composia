# Secrets 配置

本文档说明 `controller.secrets` 配置。

## 配置示例

```yaml
controller:
  secrets:
    provider: age
    identity_file: "/app/configs/age-identity.key"
    recipient_file: "/app/configs/age-recipients.txt"
    armor: true
```

## 字段说明

| 字段 | 说明 |
|------|------|
| `provider` | 加密提供方，当前仅支持 `age` |
| `identity_file` | age 私钥文件路径 |
| `recipient_file` | age 公钥文件路径，可选；未配置时会从 `identity_file` 推导 |
| `armor` | 是否使用 ASCII Armor 格式 |

如果配置了 `secrets` 段，则 `provider` 和 `identity_file` 是必填项，且 `provider` 必须是 `age`。`recipient_file` 可选。

## 生成 age 密钥

```bash
# 生成 age 密钥对
age-keygen -o key.txt

# 提取公钥
cat key.txt | grep "public key" > recipients.txt
```

挂载到容器时：

- `key.txt` 作为 `identity_file`（私钥）
- `recipients.txt` 可作为 `recipient_file`（公钥）

## 运行时语义

启用 `controller.secrets` 后，Composia 的 secret 写入与下发遵循以下规则：

- Controller 使用配置的 age 身份解密和重新加密 secret 文件
- Git 仓库中保存的是加密后的 secret，而不是明文
- 明文 secret 不会持久化写入 `controller.repo_dir` 工作树
- Agent 仅在任务 bundle 中拿到运行时所需的解密结果

这意味着：

- secret 的仓库写入会复用普通 repo 写事务的并发与同步检查
- 运行时明文只存在于 agent 侧的任务执行上下文，不应当作为长期文件依赖
