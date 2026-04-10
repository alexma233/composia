# 配置安全

本文档整理配置文件、token 和密钥文件的基本安全建议。

## Token 管理

- 使用强随机字符串作为 Controller 和节点 token
- 生产环境不要复用开发环境 token
- 定期轮换 token

## 配置文件挂载

建议对配置目录使用只读挂载：

```yaml
# docker-compose.yaml
volumes:
  - ./config:/app/configs:ro
```

## 密钥文件存放

- 不要把本地 token 或密钥文件提交到仓库
- 将 age 私钥与公钥分别挂载到 `identity_file` 和 `recipient_file`
- 仅在需要的容器中挂载对应密钥文件
