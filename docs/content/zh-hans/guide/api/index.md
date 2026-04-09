# API 参考

Composia 的 RPC 接口定义位于 `proto/`，参考文档直接由 protobuf 注释生成。

## 协议

- 传输协议：`ConnectRPC`
- 默认请求体编码：`application/json`
- 鉴权：面向 controller 的接口使用 `Authorization: Bearer <token>`
- 使用 JSON 调用 Connect 接口时需要携带 `Connect-Protocol-Version: 1`

## 参考页

- [Controller API Reference](/guide/api/controller-reference)
- [Agent Internal API Reference](/guide/api/agent-internal-reference)

## 重新生成

在仓库根目录运行：

```bash
bun run docs:api:generate
```

生成后的 Markdown 文件会提交到 `docs/content/en-us/guide/api/`，由 VitePress 直接发布。
