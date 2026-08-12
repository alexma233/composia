---
title: "本地开发"
date: '2026-07-06T00:00:00+08:00'
weight: 10
---

使用 `mise` 作为本地开发的统一入口。

## 设置

```bash
mise install
mise run setup
```

`mise run setup` 会准备 `dev/` 下的本地文件，创建开发令牌和 age 密钥文件，并在需要时初始化控制器仓库。现有本地文件不会被覆盖。

## 应用堆栈

启动控制器、agent 和 Web UI：

```bash
mise run dev
```

- 控制器：<http://127.0.0.1:7001>
- Web UI：<http://127.0.0.1:5173>

查看日志或停止堆栈：

```bash
mise run dev:logs
mise run dev:down
```

## 文档

默认情况下，文档不会随应用堆栈启动。

```bash
mise run dev:docs # 仅文档，监听 :5174
mise run dev:all  # 应用和文档
```

## 检查

提交前运行标准本地检查：

```bash
mise run check
```

需要时运行较慢的后端检查：

```bash
mise run check:full
```

## Protobuf 生成

修改 `proto/**` 后，重新生成已纳入版本控制的 Protobuf 输出和 API 文档：

```bash
mise run gen
```

提交 `gen/go/`、`web/src/lib/gen/` 和 `site/content/docs/developer-guide/api/` 下的生成文件。

## E2E

使用真实的控制器和 agent 测试堆栈运行全部本地 E2E 测试：

```bash
mise run e2e
```
