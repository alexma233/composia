---
title: "密钥"
date: '2026-05-26T00:00:00+08:00'
weight: 50
---

Composia 使用 age 加密在期望状态仓库中管理加密的密钥文件。加密和解密在控制器上发生。Agent 永远不会访问 age 私钥。

## 配置

密钥需要一个 age 密钥对。在控制器配置中设置：

```yaml
controller:
  secrets:
    provider: age
    identity_file: "/app/configs/age-identity.key"
```

| 键 | 类型 | 必填 | 描述 |
|-----|------|----------|-------------|
| `provider` | `string` | 是 | 必须为 `age`。 |
| `identity_file` | `string` | 是 | age 私钥文件的路径。 |
| `recipient_file` | `string` | 否 | 包含 age 接收者（公钥）的文件路径。如果省略，接收者从私钥派生。 |
| `armor` | `bool` | 否 | 使用 ASCII 封装输出。默认为 `true`。 |

生成密钥对：

```bash
age-keygen -o age-identity.key
```

可选：提取公钥作为接收者：

```bash
age-keygen -y age-identity.key > age-recipients.txt
```

## 密钥的存储方式

仓库中的密钥文件按惯例带有 `.enc` 扩展名。它们以 age 加密的密文形式存储：

```
my-app/
├── docker-compose.yaml
├── composia-meta.yaml
└── .secret.env.enc        （使用 age 加密）
```

控制器在写入时加密明文，在读取时解密。仓库中只包含密文。密钥从不会以明文形式出现在仓库、任务日志或传输给 agent 的数据中。

## 密钥如何到达 agent

在部署或更新任务的渲染步骤中，控制器：

1. 从仓库中的服务目录读取加密文件。
2. 使用 age 私钥解密每个文件。
3. 将解密后的内容作为 `.composia-secret.env` 注入服务包。

服务包通过 agent 报告连接流式传输到 agent。Agent 将包写入磁盘并继续执行 `docker compose up`。解密后的密钥环境变量可供 Compose 服务使用，而 agent 永远不会看到私钥。

## CLI 用法

写入加密的密钥文件：

```bash
composia secret update my-app .secret.env.enc --file ./local-plain.env
```

读取并解密密钥文件：

```bash
composia secret get my-app .secret.env.enc
```

就地编辑密钥（打开编辑器）：

```bash
composia secret edit my-app .secret.env.enc
```

所有密钥写入操作都包含基准版本检查，以防止与并发更改冲突。

## 文件路径规则

密钥文件路径必须：

- 相对于服务目录（不能是绝对路径）。
- 不包含路径穿越序列，如 `../`。
- 指向服务目录内的文件。

控制器定位服务，相对于服务目录解析文件路径，并对仓库文件进行操作。

## 错误情况

- **密钥未配置**: 当 `controller.secrets` 未设置时，`GetSecret` 和 `UpdateSecret` 返回 `FailedPrecondition`。
- **文件未找到**: 当文件不存在时，`GetSecret` 返回空内容响应而非错误。这使客户端能区分文件缺失和解密失败。
- **基准版本冲突**: `UpdateSecret` 使用 CAS（比较并交换）机制针对仓库 HEAD。如果自上次读取以来仓库发生了变化，写入将因版本冲突而失败。
