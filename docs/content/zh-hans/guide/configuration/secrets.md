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
| `recipient_file` | age 公钥文件路径 |
| `armor` | 是否使用 ASCII Armor 格式 |

如果配置了 `secrets` 段，则 `provider`、`identity_file` 和 `recipient_file` 都是必填项，且 `provider` 必须是 `age`。

## 生成 age 密钥

```bash
# 生成 age 密钥对
age-keygen -o key.txt

# 提取公钥
cat key.txt | grep "public key" > recipients.txt
```

挂载到容器时：

- `key.txt` 作为 `identity_file`（私钥）
- `recipients.txt` 作为 `recipient_file`（公钥）
