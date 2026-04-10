# 配置验证

如果是在本地直接跑源码，建议使用开发配置进行验证：

```bash
# 使用 dev 配置启动 Controller
go run ./cmd/composia controller -config ./dev/config.controller.yaml

# 使用共享的 dev 配置启动 main Agent
go run ./cmd/composia agent -config ./dev/config.controller.yaml
```

本地开发配置建议放在 `./dev/` 下，并且不要提交本地 token 或密钥文件。
