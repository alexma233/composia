# 部署管理

本文档介绍如何使用 Composia 部署、更新、停止和重启服务。

## 部署流程

### 1. 服务发现

Controller 会定期扫描 `repo_dir`，查找所有包含 `composia-meta.yaml` 的目录：

```
repo/
├── service-a/
│   ├── composia-meta.yaml    ← 发现
│   └── docker-compose.yaml
├── service-b/
│   ├── composia-meta.yaml    ← 发现
│   └── docker-compose.yaml
└── README.md
```

### 2. 实例生成

每个 Service 根据 `nodes` 配置生成对应的 ServiceInstance：

```yaml
# service-a/composia-meta.yaml
name: service-a
nodes:
  - main
  - edge
```

生成：
- `service-a` on `main`
- `service-a` on `edge`

### 3. 触发部署

当用户通过 Web UI 或 API 触发部署时：

1. Controller 验证服务定义
2. 为每个目标节点创建 `deploy` 任务
3. Agent 拉取任务并执行
4. 下载服务 bundle（包含 Compose 文件和配置）
5. 渲染运行目录
6. 执行 `docker compose up -d`
7. 如需 Caddy，触发 `caddy_sync` 和 `caddy_reload`
8. 上报执行结果

## 可用操作

### 部署 (Deploy)

首次部署服务到节点。

**适用场景：**
- 新服务首次部署
- 服务从 Git 仓库加载后首次部署

**行为：**
- 下载服务 bundle
- 渲染运行目录
- 执行 `docker compose up -d`
- 触发 Caddy 同步（如果配置了 `network.caddy`）

### 更新 (Update)

更新已部署的服务。

**适用场景：**
- 更新了 `docker-compose.yaml`
- 更新了镜像版本
- 更新了环境变量

**行为：**
- 拉取最新 bundle
- 重新渲染运行目录
- 执行 `docker compose up -d`（自动处理变更）
- 触发 Caddy 刷新

**注意事项：**
- Compose 会自动判断哪些容器需要重建
- 数据卷会保留
- 环境变量变更会触发重建

### 停止 (Stop)

停止服务实例。

**适用场景：**
- 暂时下线服务
- 释放节点资源
- 准备迁移服务

**行为：**
- 执行 `docker compose down`
- 删除生成的 Caddy 片段
- 触发 Caddy reload

**注意事项：**
- 数据卷会保留（除非使用 `down -v`）
- 容器会被删除
- 服务定义会保留在 Git 仓库

### 重启 (Restart)

重启服务实例。

**适用场景：**
- 应用配置变更需要重启
- 内存泄漏等临时问题

**行为：**
- 依次执行停止和启动
- 不会重新拉取 bundle（如需更新，使用 Update）

## 使用 Web UI 操作

### 部署服务

1. 进入「服务」页面
2. 点击目标服务
3. 在「实例」标签页找到目标节点
4. 点击「部署」按钮

### 批量操作

在「服务」列表页，可以对多个服务进行批量操作：
- 批量部署
- 批量更新
- 批量停止

### 查看部署状态

部署过程中，可以在「任务」页面实时查看进度：

| 状态 | 说明 |
|------|------|
| `pending` | 等待开始 |
| `running` | 正在执行 |
| `awaiting_confirmation` | 等待外部确认步骤完成 |
| `succeeded` | 执行成功 |
| `failed` | 执行失败 |
| `cancelled` | 已取消 |

## 使用 API 操作

当前 Controller 暴露的是 ConnectRPC 服务，而不是 `/api/v1/...` 形式的 REST 接口。

部署相关操作请使用以下 RPC 方法：

- `composia.controller.v1.ServiceCommandService/RunServiceAction`：deploy、update、stop、restart、backup、dns_update、caddy_sync
- `composia.controller.v1.ServiceCommandService/MigrateService`：迁移服务
- `composia.controller.v1.ServiceInstanceService/RunServiceInstanceAction`：对单个实例执行操作

## 多节点部署策略

### 同一服务多节点部署

```yaml
# composia-meta.yaml
name: my-app
nodes:
  - main
  - edge-1
  - edge-2
```

部署后会在三个节点各创建一个实例。

### 按环境分离

```yaml
# my-app-prod/composia-meta.yaml
name: my-app-prod
nodes:
  - main

---

# my-app-staging/composia-meta.yaml
name: my-app-staging
nodes:
  - edge-1
```

### 滚动更新

当前 Composia 会同时对所有目标节点执行更新。如需滚动更新，建议：

1. 先更新 `nodes` 配置，移除部分节点
2. 等待更新完成
3. 重新添加节点
4. 再次更新

## 部署最佳实践

### 1. 使用健康检查

```yaml
# docker-compose.yaml
services:
  app:
    image: myapp:latest
    healthcheck:
      test: ["CMD", "curl", "-f", "http://localhost:8080/health"]
      interval: 30s
      timeout: 10s
      retries: 3
      start_period: 40s
```

### 2. 配置重启策略

```yaml
services:
  app:
    image: myapp:latest
    restart: unless-stopped
```

### 3. 资源限制

```yaml
services:
  app:
    image: myapp:latest
    deploy:
      resources:
        limits:
          cpus: '2.0'
          memory: 2G
        reservations:
          cpus: '0.5'
          memory: 512M
```

### 4. 环境分离

使用不同的服务名称区分环境：

```yaml
# 生产环境
name: my-app-prod
project_name: my-app-prod

# 测试环境
name: my-app-staging
project_name: my-app-staging
```

### 5. 版本控制

在镜像标签中明确版本：

```yaml
services:
  app:
    image: myapp:1.2.3  # 明确版本
    # 避免使用 latest
```

## 故障排查

### 部署失败

检查任务日志：
1. 进入「任务」页面
2. 找到失败的部署任务
3. 查看详细日志

常见问题：
- 镜像拉取失败：检查镜像名称和网络
- 端口冲突：检查端口占用情况
- 环境变量缺失：检查 `.env` 文件

### 容器无法启动

在「容器」页面：
1. 找到目标容器
2. 查看日志
3. 检查环境变量和卷挂载

### Caddy 配置未生效

检查：
1. `network.caddy.enabled` 是否为 `true`
2. `Caddyfile.fragment` 路径是否正确
3. Caddy 基础设施服务是否运行

## 相关文档

- [服务定义](./service-definition) —— 如何定义服务
- [任务系统](./operations#任务系统) —— 了解任务执行机制
- [网络配置](./networking) —— 配置反向代理
