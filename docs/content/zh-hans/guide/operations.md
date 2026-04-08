# 日常运维

本文档介绍 Composia 的任务系统、资源管理和常见运维操作。

## 任务系统

### 概述

Composia 使用任务队列管理所有异步操作：

- Controller 负责任务创建和状态管理
- Agent 通过长轮询主动拉取属于自己的任务
- 任务按步骤上报状态、日志和结果

### 任务类型

| 任务类型 | 说明 | 触发方式 |
|----------|------|----------|
| `deploy` | 部署服务 | 手动/API |
| `update` | 更新服务 | 手动/API |
| `stop` | 停止服务 | 手动/API |
| `restart` | 重启服务 | 手动/API |
| `backup` | 执行备份 | 手动/API |
| `dns_update` | 更新 DNS 记录 | 自动/手动 |
| `caddy_sync` | 同步 Caddy 配置 | 自动 |
| `caddy_reload` | 重载 Caddy | 自动 |
| `prune` | 清理资源 | 手动/API |
| `migrate` | 迁移服务 | 手动/API |

### 任务生命周期

```
Created → Pending → Running → Completed
                    │
                    └─► Failed
                    │
                    └─► Cancelled
```

### 查看任务

**Web UI：**
- 「任务」页面显示所有任务列表
- 可按服务、节点、类型、状态筛选
- 点击任务查看详细日志

**任务状态：**

| 状态 | 说明 |
|------|------|
| Pending | 等待 Agent 拉取 |
| Running | 正在执行 |
| Success | 执行成功 |
| Failed | 执行失败 |
| Cancelled | 已取消 |

### 任务日志

任务执行过程中会实时输出日志：

```
[2024-01-15 10:30:00] 开始部署服务 my-app 到节点 main
[2024-01-15 10:30:01] 下载服务 bundle...
[2024-01-15 10:30:05] 渲染运行目录...
[2024-01-15 10:30:06] 执行 docker compose up -d
[2024-01-15 10:30:15] 容器启动成功
[2024-01-15 10:30:16] 同步 Caddy 配置...
[2024-01-15 10:30:18] 部署完成
```

## Docker 资源管理

### 容器管理

Agent 定期上报节点上的 Docker 容器信息，Controller 提供统一的浏览界面。

**查看容器：**
1. 进入「容器」页面
2. 按节点筛选容器
3. 查看容器状态、镜像、端口等信息

**容器操作：**

| 操作 | 说明 |
|------|------|
| 查看日志 | 实时查看容器日志 |
| 启动 | 启动已停止的容器 |
| 停止 | 停止运行中的容器 |
| 重启 | 重启容器 |
| 终端 | 进入容器执行命令 |

**查看容器日志：**

```
# 在 Web UI 中
1. 找到目标容器
2. 点击「日志」按钮
3. 实时查看或搜索历史日志
```

**容器终端：**

```bash
# Web UI 提供基础终端功能
1. 点击容器「终端」按钮
2. 选择 shell（bash/sh）
3. 执行命令
```

### 镜像管理

**查看镜像：**
- 「镜像」页面显示所有节点上的镜像
- 显示镜像标签、大小、创建时间

**清理镜像：**
```bash
# 手动清理未使用镜像
curl -X POST http://localhost:7001/api/v1/nodes/main/prune \
  -H "Authorization: Bearer YOUR_TOKEN" \
  -d '{"type": "images"}'
```

### 网络管理

**查看网络：**
- 「网络」页面显示 Docker 网络
- 查看网络驱动、子网、容器连接

### 卷管理

**查看卷：**
- 「卷」页面显示 Docker 卷
- 查看卷大小、挂载点

## 节点管理

### 节点状态

Agent 每 5 秒发送心跳，包含以下信息：

| 信息 | 说明 |
|------|------|
| 在线状态 | 是否连接到 Controller |
| Docker 版本 | 节点 Docker 版本 |
| 容器数量 | 运行中的容器数 |
| 资源使用 | CPU、内存、磁盘使用率 |
| 服务实例 | 该节点上的服务实例列表 |

### 节点视图

**Web UI 提供以下视图：**

- **节点列表**: 查看所有节点概览
- **节点详情**: 单个节点的详细信息和资源使用
- **服务实例**: 节点上的服务部署情况
- **Dashboard**: 整体资源使用趋势

### 节点操作

**重新连接 Agent：**

如果 Agent 断开连接：
1. 检查 Agent 容器日志
2. 检查网络连通性
3. 重启 Agent 容器

```bash
docker compose restart agent
```

## 资源清理

### 清理任务

执行 `prune` 任务清理未使用资源：

**Web UI：**
1. 进入「节点」页面
2. 选择目标节点
3. 点击「清理」按钮
4. 选择要清理的资源类型

**API：**

```bash
# 清理未使用容器
curl -X POST http://localhost:7001/api/v1/nodes/main/prune \
  -H "Authorization: Bearer YOUR_TOKEN" \
  -d '{"type": "containers"}'

# 清理未使用镜像
curl -X POST http://localhost:7001/api/v1/nodes/main/prune \
  -H "Authorization: Bearer YOUR_TOKEN" \
  -d '{"type": "images"}'

# 清理未使用卷
curl -X POST http://localhost:7001/api/v1/nodes/main/prune \
  -H "Authorization: Bearer YOUR_TOKEN" \
  -d '{"type": "volumes"}'

# 清理所有
curl -X POST http://localhost:7001/api/v1/nodes/main/prune \
  -H "Authorization: Bearer YOUR_TOKEN" \
  -d '{"type": "all"}'
```

### 自动清理建议

设置定时任务定期清理：

```bash
# cron 示例（每天凌晨清理）
0 3 * * * curl -X POST http://localhost:7001/api/v1/nodes/main/prune \
  -H "Authorization: Bearer YOUR_TOKEN" \
  -d '{"type": "images"}'
```

## 日志管理

### 任务日志

任务日志存储在 Controller 的 `log_dir`：

```
log_dir/
├── tasks/
│   ├── 2024-01-15/
│   │   ├── task-001.log
│   │   └── task-002.log
│   └── 2024-01-16/
│       └── task-003.log
```

### 容器日志

容器日志通过 Docker API 实时获取，历史日志由 Docker 管理。

### 日志保留策略

建议配置日志轮转：

```yaml
# docker-compose.yaml
services:
  controller:
    logging:
      driver: "json-file"
      options:
        max-size: "100m"
        max-file: "5"
```

## 监控和告警

### 当前监控能力

- **实时状态**: Web UI 实时显示服务、容器、节点状态
- **资源使用**: 节点 CPU、内存、磁盘使用率
- **日志查看**: 实时查看容器和任务日志

### 建议的监控方案

**集成 Prometheus + Grafana：**

在需要监控的节点上部署 node-exporter 和 cadvisor：

```yaml
# monitoring/docker-compose.yaml
services:
  node-exporter:
    image: prom/node-exporter
    volumes:
      - /proc:/host/proc:ro
      - /sys:/host/sys:ro
      - /:/rootfs:ro

  cadvisor:
    image: gcr.io/cadvisor/cadvisor
    volumes:
      - /:/rootfs:ro
      - /var/run:/var/run:ro
      - /sys:/sys:ro
      - /var/lib/docker:/var/lib/docker:ro
```

**自定义告警：**

使用 Composia API 查询状态，配合外部告警系统：

```bash
# 检查服务状态
curl http://localhost:7001/api/v1/services/my-app/status \
  -H "Authorization: Bearer YOUR_TOKEN"
```

## 故障排查

### 常见问题

**1. Agent 无法连接 Controller**

检查：
- Controller 地址是否正确
- Token 是否匹配
- 网络连通性
- 防火墙设置

**2. 部署失败**

检查：
- 任务日志中的错误信息
- Docker Compose 文件语法
- 镜像是否可拉取
- 端口是否冲突

**3. 服务状态不一致**

检查：
- Agent 是否在线
- 容器是否实际运行
- 标签是否正确设置

**4. Caddy 配置未生效**

检查：
- Caddy 基础设施服务状态
- 配置片段语法
- Agent 目录挂载

### 调试模式

启用调试日志：

```bash
# Controller
LOG_LEVEL=debug go run ./cmd/composia controller ...

# Agent
LOG_LEVEL=debug go run ./cmd/composia agent ...
```

### 获取支持

- 查看 [GitHub Issues](https://github.com/alexma233/composia/issues)
- 查阅 [开发文档](./development)
- 检查日志文件

## 性能优化

### Controller 优化

- 使用 SSD 存储 `state_dir`
- 定期清理旧任务日志
- 合理设置 `pull_interval`

### Agent 优化

- 确保 Docker socket 访问顺畅
- 监控 Agent 资源使用
- 定期清理未使用资源

### 数据库优化

SQLite 性能优化：

```sql
-- 定期执行 VACUUM
VACUUM;

-- 检查数据库完整性
PRAGMA integrity_check;
```

## 相关文档

- [部署管理](./deployment) —— 服务部署操作
- [备份与迁移](./backup-migrate) —— 数据保护操作
- [网络配置](./networking) —— DNS 和代理配置
