# 备份与迁移

本文档介绍 Composia 的数据备份和迁移功能。

## 备份功能

### 概述

Composia 的备份功能基于以下组件：

- **rustic**: 备份引擎，支持增量备份、加密、压缩
- **data_protect**: 定义可备份的数据项和策略
- **backup**: 定义哪些数据项参与备份

### 1. 部署 rustic 基础设施

创建 rustic 基础设施服务：

```yaml
# infra-backup/composia-meta.yaml
name: infra-backup
nodes:
  - main

infra:
  rustic:
    compose_service: rustic
    profile: default
```

```yaml
# infra-backup/docker-compose.yaml
services:
  rustic:
    image: rustic:latest
    volumes:
      - ./config:/config
      - ./repo:/repo
      - /var/lib/composia:/data:ro  # 挂载 Composia 数据
    command: rustic -c /config/rustic.toml
```

```toml
# infra-backup/config/rustic.toml
[repository]
repository = "/repo"
password = "your-backup-password"

[backup]
exclude-if-present = [".nobackup"]
```

### 2. Controller 配置

```yaml
controller:
  rustic:
    main_nodes:
      - "main"    # 指定哪些节点可以执行备份
```

### 3. 业务服务配置

配置数据保护策略：

```yaml
# my-app/composia-meta.yaml
name: my-app
nodes:
  - main

data_protect:
  data:
    - name: uploads
      backup:
        strategy: files.copy
        include:
          - ./data/uploads
        exclude:
          - ./data/uploads/temp
      restore:
        strategy: files.copy
        include:
          - ./data/uploads
    
    - name: database
      backup:
        strategy: database.pgdumpall
        service: postgres      # Compose 服务名
      restore:
        strategy: files.copy
        include:
          - ./restore/

backup:
  data:
    - name: uploads
      provider: rustic
    - name: database
      provider: rustic
```

### 备份策略

| 策略 | 说明 | 使用场景 |
|------|------|----------|
| `files.copy` | 直接复制文件 | 静态文件、上传目录 |
| `files.tar_after_stop` | 停止服务后打包 | 需要一致性的数据 |
| `database.pgdumpall` | PostgreSQL 全量导出 | PostgreSQL 数据库 |

### 执行备份

**Web UI：**
1. 进入「服务」页面
2. 找到目标服务
3. 点击「备份」按钮
4. 触发该服务当前已配置数据项的备份

**API：**

当前 Controller 暴露的是 ConnectRPC 方法，而不是 `/api/v1/...` 形式的 REST 接口。
备份任务请使用 `composia.controller.v1.ServiceCommandService/RunServiceAction`。

### 查看备份

备份完成后，可以在「备份」页面查看：

| 字段 | 说明 |
|------|------|
| 服务 | 备份所属服务 |
| 数据项 | 备份的数据项名称 |
| 任务 ID | 生成该备份记录的任务 |
| 时间 | 备份时间 |
| 状态 | 成功/失败 |

### 备份最佳实践

1. **定期备份**
   - 重要数据每日备份
   - 数据库建议在低峰期备份

2. **备份验证**
   - 定期测试恢复流程
   - 验证备份完整性

3. **保留策略**
   - 配置 rustic 的 forget 策略
   - 保留最近 7 天每日快照
   - 保留每月和每年快照

4. **异地备份**
   - 配置 rustic 的 rclone 后端
   - 同步到对象存储（S3, B2 等）

## 迁移功能

### 概述

迁移功能允许将服务实例从一个节点移动到另一个节点，同时保持数据完整性。

### 迁移流程

```
源节点                      目标节点
   │                           │
   ▼                           │
导出数据 ◄─────────────────────┤
   │                           │
停止实例 ◄─────────────────────┤
   │                           │
卸载配置                      │
   │                           │
   ├──────────────────────────►│
   │                          导入数据
   │                           │
   ├──────────────────────────►│
   │                          启动实例
   │                           │
更新 DNS ◄─────────────────────┤
   │                           │
更新 nodes 配置               │
```

### 配置迁移

在 `composia-meta.yaml` 中配置：

```yaml
name: my-app
nodes:
  - main      # 当前部署节点

# 数据保护（backup 和 restore 都必须配置）
data_protect:
  data:
    - name: uploads
      backup:
        strategy: files.copy
        include:
          - ./data/uploads
      restore:
        strategy: files.copy
        include:
          - ./data/uploads

# 迁移时带走的数据
migrate:
  data:
    - name: uploads
```

### 执行迁移

**Web UI：**
1. 进入服务详情页
2. 使用服务详情页中的迁移控件
3. 选择源节点和目标节点
4. 点击「迁移」按钮

**API：**

请使用 `composia.controller.v1.ServiceCommandService/MigrateService`。

### 迁移步骤详解

1. **源节点数据导出**
   - 执行备份任务
   - 将数据打包传输

2. **源实例停止**
   - 停止 Docker Compose 服务
   - 卸载 Caddy 配置

3. **源节点 Caddy reload**
   - 移除代理配置

4. **目标节点数据恢复**
   - 解压数据到目标路径
   - 执行 restore 策略

5. **目标实例启动**
   - 部署服务到目标节点
   - 挂载 Caddy 配置

6. **目标节点 Caddy reload**
   - 加载新代理配置

7. **DNS 更新**
   - 更新 DNS 记录指向新节点

8. **回写配置**
   - 更新 `composia-meta.yaml` 中的 `nodes`
   - 提交到 Git 仓库

### 迁移注意事项

**前提条件：**
- 服务必须已在源节点部署
- 目标节点必须在线且可用
- 数据项必须配置 `backup` 和 `restore` 策略

**中断时间：**
- 迁移过程会导致服务短暂中断
- 中断时间取决于数据大小和网络速度
- 建议在低峰期执行迁移

**数据一致性：**
- 迁移前会停止源实例
- 确保没有正在写入的数据
- 对于数据库，建议使用导出策略

### 迁移失败处理

如果迁移失败：
1. 检查任务日志定位问题
2. 根据日志修复失败步骤
3. 如有需要，可手动回滚：
   - 在源节点重新部署
   - 在目标节点停止并清理
   - 恢复 DNS 记录

## 恢复功能

### 当前状态

- 迁移流程中已使用恢复能力
- 独立的完整恢复工作流仍在完善中
- 可以通过手动方式恢复数据

### 手动恢复

1. 在 Web UI 中找到备份记录
2. 结合 rustic 仓库中的备份元数据确定要恢复的快照
3. 在 rustic 容器中执行：

```bash
rustic restore <snapshot-id>:/path/to/backup /path/to/restore
```

4. 重启服务应用恢复的数据

## 定时备份与 rustic 维护

Composia 现在支持由 controller 内置 scheduler 触发自动备份与 rustic 维护任务。

### backup 定时

`backup` 是 service/data 级任务：

- controller 可通过 `controller.backup.default_schedule` 提供默认定时
- service 可在 `backup.data[].schedule` 中覆盖默认值
- `schedule: none` 表示该数据项永不自动备份

示例：

```yaml
backup:
  data:
    - name: uploads
      provider: rustic
      schedule: "0 */6 * * *"
    - name: cache
      provider: rustic
      schedule: none
```

### rustic forget / prune 定时

`rustic_forget` 和 `rustic_prune` 是整个 rustic 仓库级维护任务：

- 只能在 controller 配置中设置 schedule
- 不按 service 过滤
- 不按 data 过滤
- 实际行为仍由 rustic 自身配置文件决定

示例：

```yaml
controller:
  rustic:
    main_nodes:
      - "main"
    maintenance:
      forget_schedule: "15 3 * * *"
      prune_schedule: "45 3 * * *"
```

### 触发语义

- scheduler 创建的任务来源为 `schedule`
- `backup` 会按 service data 配置生成任务
- `rustic_forget` 与 `rustic_prune` 会在一个可用的 `rustic.main_nodes` 节点上运行整个 rustic repo 的维护命令

## 相关文档

- [服务定义](./service-definition) —— 数据保护配置详解
- [部署管理](./deployment) —— 服务部署流程
- [rustic 文档](https://rustic.cli.rs/) —— 备份引擎参考
