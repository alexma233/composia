# 核心概念

理解 Composia 的四个核心对象，有助于更好地使用平台进行服务管理。

## 四个核心对象

Composia 围绕以下四个一等对象进行设计：

| 对象 | 说明 | 示例 |
|------|------|------|
| **Service** | 逻辑服务定义 | `my-app` 服务配置 |
| **ServiceInstance** | 服务在某个节点上的部署实例 | `my-app` 在 `node-1` 上的实例 |
| **Container** | 实际的 Docker 容器 | `my-app-web-1` |
| **Node** | 执行代理对应的 Docker 主机 | `node-1`, `node-2` |

## 对象关系

```
┌─────────────────────────────────────┐
│           Service                   │
│  (Git 仓库中的服务定义)              │
└──────────────┬──────────────────────┘
               │
       ┌───────┴───────┐
       ▼               ▼
┌──────────────┐ ┌──────────────┐
│ServiceInstance│ │ServiceInstance│
│  on Node A   │ │  on Node B   │
└──────┬───────┘ └──────┬───────┘
       │                │
       ▼                ▼
┌──────────────┐ ┌──────────────┐
│  Container   │ │  Container   │
│  (Docker)    │ │  (Docker)    │
└──────────────┘ └──────────────┘
```

### Service（服务）

服务是逻辑层面的定义，来源于 Git 仓库中的一个目录：

```
repo/
└── my-service/
    ├── composia-meta.yaml    # 服务元数据
    └── docker-compose.yaml   # Compose 配置
```

**特点：**
- 使用 `composia-meta.yaml` 定义服务属性
- 包含 Docker Compose 配置
- 可以部署到多个节点
- 控制平面保存的是期望状态

### ServiceInstance（服务实例）

服务实例是服务在某个特定节点上的部署表现：

**特点：**
- 由 Service + Node 唯一确定
- 代表期望在该节点上部署的服务
- Agent 负责把实际状态拉向期望状态
- 每个实例可以有独立的部署状态

**示例：**

```yaml
# my-service/composia-meta.yaml
name: my-service
nodes:
  - main
  - edge
```

这将创建两个 ServiceInstance：
- `my-service` on `main`
- `my-service` on `edge`

### Container（容器）

容器是 Docker 实际运行的进程：

**特点：**
- 通过 Compose labels 归属到某个 ServiceInstance
- 可以被独立管理（查看日志、重启等）
- 可能在 ServiceInstance 生命周期外独立存在
- Agent 定期上报容器状态

**标签关联：**

```yaml
# docker-compose.yaml
services:
  web:
    labels:
      - "composia.service=my-service"
      - "composia.instance=my-service-main"
```

### Node（节点）

节点是执行代理（Agent）运行的 Docker 主机：

**特点：**
- 在 Controller 配置中预先声明
- 每个节点有唯一的 ID 和认证 Token
- Agent 通过 Token 连接到 Controller
- 可以有自定义属性（如公网 IP）

**Controller 配置示例：**

```yaml
controller:
  nodes:
    - id: "main"
      display_name: "Main Server"
      enabled: true
      token: "main-agent-token"
      public_ipv4: "203.0.113.10"
    
    - id: "edge"
      display_name: "Edge Node"
      enabled: true
      token: "edge-agent-token"
```

## 分层视角

Web UI 中不同页面对应不同对象层级：

| 页面 | 对象层级 | 功能 |
|------|----------|------|
| 服务列表 | Service | 管理所有服务定义 |
| 服务详情 | Service + ServiceInstance | 查看服务的节点分布 |
| 实例详情 | ServiceInstance + Container | 管理特定实例的容器 |
| 容器列表 | Container | 浏览所有 Docker 容器 |
| 节点列表 | Node | 查看所有代理节点 |

## 状态流向

```
Git Repo (期望状态)
       │
       ▼
Controller (协调)
       │
       ▼
ServiceInstance (展开)
       │
       ▼
Agent (拉取)
       │
       ▼
Docker (实际状态)
```

1. **定义阶段**: 用户在 Git 仓库中定义 Service
2. **协调阶段**: Controller 扫描并解析 Service，创建 ServiceInstance
3. **部署阶段**: Agent 获取任务，创建实际的 Container
4. **同步阶段**: Agent 定期上报 Container 状态

## 典型使用场景

### 场景一：单节点部署

```yaml
# composia-meta.yaml
name: my-app
nodes:
  - main
```

- 1 个 Service
- 1 个 ServiceInstance (on main)
- N 个 Containers

### 场景二：多节点部署

```yaml
# composia-meta.yaml
name: my-app
nodes:
  - main
  - edge-1
  - edge-2
```

- 1 个 Service
- 3 个 ServiceInstance (每个节点一个)
- 每个节点 N 个 Containers

### 场景三：多服务多节点

```
Services:
  - web (nodes: [main, edge-1])
  - api (nodes: [main])
  - db (nodes: [main])

ServiceInstances:
  - web-main
  - web-edge-1
  - api-main
  - db-main
```

## 相关文档

- [服务定义](./service-definition) —— 如何定义 Service
- [配置指南](./configuration) —— 如何配置 Node
- [部署管理](./deployment) —— 管理 ServiceInstance 生命周期
