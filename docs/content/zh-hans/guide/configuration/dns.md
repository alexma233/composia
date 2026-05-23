# DNS 配置

本文档说明 `controller.dns` 配置。

## 配置示例

```yaml
controller:
  dns:
    cloudflare:
      api_token_file: "/app/configs/cloudflare-token.txt"
    alidns:
      access_key_id_file: "/app/configs/alidns-access-key-id.txt"
      access_key_secret_file: "/app/configs/alidns-access-key-secret.txt"
      zones: ["example.com"]
    dnspod:
      secret_id_file: "/app/configs/dnspod-secret-id.txt"
      secret_key_file: "/app/configs/dnspod-secret-key.txt"
      zones: ["example.com"]
    route53:
      access_key_id_file: "/app/configs/aws-access-key-id.txt"
      secret_access_key_file: "/app/configs/aws-secret-access-key.txt"
      region: "us-east-1"
      zones: ["example.com"]
    huaweicloud:
      access_key_id_file: "/app/configs/huaweicloud-access-key-id.txt"
      secret_access_key_file: "/app/configs/huaweicloud-secret-access-key.txt"
      region_id: "cn-south-1"
      zones: ["example.com"]
```

也可以直接内联：

```yaml
controller:
  dns:
    cloudflare:
      api_token: "cloudflare-token"
```

支持的 provider：
- `cloudflare`
- `alidns`
- `dnspod`
- `route53`
- `huaweicloud`

除 `cloudflare` 外，其他 provider 需要配置 `zones`。`zones` 用于把服务 `network.dns.hostname` 匹配到实际 DNS zone。

`route53` 也可以使用 AWS 环境变量、共享配置或实例角色；如果不内联凭据，仍然需要配置 `zones`。

当前文档只覆盖平台侧配置。

服务侧 DNS 规则、自动推导逻辑和记录写入行为，请参考 [DNS 配置](../dns)。
