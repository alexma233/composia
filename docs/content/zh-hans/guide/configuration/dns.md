# DNS 配置

本文档说明 `controller.dns` 配置。

## 配置示例

```yaml
controller:
  dns:
    cloudflare:
      api_token_file: "/app/configs/cloudflare-token.txt"
```

当前文档只覆盖平台侧配置。

服务侧 DNS 规则、自动推导逻辑和记录写入行为，请参考 [网络配置](../networking)。
