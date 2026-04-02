### 1. 项目目标（一句话定位）

**Composia = 服务声明 + 部署控制面 + 运行节点执行面**

核心目标：做一个**以服务为核心**的自建服务管理器，支持单节点/多节点，重点解决实际自建运维中的备份、迁移、更新、监控等痛点，而不是做一个普通的 Docker Compose GUI。

### 2. 核心设计原则（已全部确定）

- **以服务为主**：所有操作（部署、备份、迁移、监控）都围绕“服务”（如 stalwart、vaultwarden）而不是文件类型。
- **一个服务只部署在一个节点**：v1 不支持多副本/高可用（后续 v2 再单独设计）。
- **单主控制面**：永远只有 1 个 active main + N 个 agent，不做 active-active。
- **同一二进制不同角色**：`composia` 一个程序，通过启动参数切换 `main` / `agent` 模式。
- **Git 只存声明**：Git 负责服务定义、meta.yaml、模板、历史；运行时状态全部进 SQLite。
- **main 持全量，agent 只持本地**：agent 只保存自己需要运行的服务包，减少暴露和同步量。
- **Caddy 每个节点独立**：每个 agent 节点独立运行一个 Caddy 实例，只提供一个 startup 模板（cf 指 Cloudflare Origin 模式），用户自己手写配置。
- **备份默认内置 rustic**：其他 provider 通过 PR 扩展。
- **前端**：SvelteKit + shadcn-svelte（Tailwind）。
- **网页编辑器**：CodeMirror 6（支持 YAML 高亮 + 基本 lint，可后续加自定义关键词检查）。
- **Mobile Support**：Web UI 响应式（手机浏览器可用），不需要 PWA。
- **Image update**：同时支持手动触发 + meta.yaml 配置自动 schedule。
- **Podman**：v1 只支持 Docker，provider 抽象已留好扩展位。
- **i18n**：只留接口，主要英文，只做 Web UI（CLI 不需要）。
- **DNS**：自动帮服务更新 DNS 记录（v1 只支持 Cloudflare）。
- **权限**：单用户模式。
- **数据库**：SQLite（keep it simple）。
- **后端语言**：Go。

---

### 3. 总体架构（已确定）

- **Main**（控制面）：Git 拉取、配置渲染、任务调度、备份编排、Caddy 聚合（生成最终片段）、状态数据库、API、Web UI、CLI 入口。
- **Agent**（执行面）：接收 main 下发的服务包、本地落盘、执行 docker compose up/down/pull、执行备份 hook、上报状态、健康检查。
- **通信方式**：ConnectRPC

---

### 4. 目录结构（推荐结构，已基本确定）

```text
composia/                  # 程序安装目录（main 和 agent 都一样）
  config.yaml
  .secrets.env               # 或 secrets.enc（取决于后面 Secrets 选择）
  state/composia.db

repo/                      # Git 仓库根目录（main 全量，agent 只本地服务）
  stalwart/
    docker-compose.yaml
    .env.template
    composia-meta.yaml
    config/
    site_config.caddy
  vaultwarden/
    ...
  caddy/                     # 全局 Caddy 模板（每个节点独立使用）
    docker-compose.yaml
    config/
      Caddyfile
      snippet/
      site/
```

agent 本地只会保留自己需要运行的服务 + caddy。

---

### 5. 服务定义模型（meta.yaml）（大部分已确定，还缺部分字段）

已确定字段：

```yaml
name: vaultwarden
node: node-2
project_name: vaultwarden
enabled: true

network:
  caddy:
  	enabled: true
  	source: ./site_config.caddy
  dns:
  	provider: cloudflare
	hostname: vaultwarden.alexma.top

update:
  strategy: pull_and_recreate
  backup_before_update: true
  healthcheck_after_update: true
  rollback_on_failure: true

backup:
	database:
		enabled: true
		provider: rustic
		strategy: pgdump
		schedule: "0 2 * * *" # 每天凌晨 2 点 (cron)
		retain: 30d
		pg_addr: http://pg:8080
	config:
		enabled: true
		provider: rustic
		strategy: copy
		schedule: "0 2 * * *" # 每天凌晨 2 点 (cron)
		retain: 30d
		include:
			- ./config
			- vaultwarden_config # Docker Volumes
```

---

### 6. 备份系统（已确定大部分）

- 统一抽象（pre-hook / dump / copy / snapshot / restore / retention）。
- 默认 provider：rustic（内置），其他后续 PR。
- PostgreSQL 默认用 pg_dump / pg_dumpall（不强制 pg_basebackup）。
- 备份完成后**必须上报 main** 存入 SQLite。
- 迁移时**强制备份**（即使 meta.yaml 里关闭备份）。

---

### 7. 服务迁移流程（已确定）

1. 修改 meta.yaml 中的 node。
2. main 自动备份旧节点。
3. 停止旧节点服务。
4. rsync 数据包到新节点。
5. 新节点启动。
6. 更新 Caddy 配置 + DNS（Cloudflare）。
7. 健康检查 + 验证。

---

### 8. Secrets 管理

加密后进 Git（.secrets.enc.yaml，用户自行用 age/sops 加密）。

---

### 9. AI 助手 / MCP / Skills

CLI Skill 或者直接使用普通文档替代

---

### 10. 节点注册与 Main 高可用

- 选项 A：完全手动（手动维护节点列表，main 挂了就手动在 agent config.yaml 改地址（**未确定**））。

### 11. Web UI 页面

核心页面已确定：服务列表 / 服务详情 / 节点列表 / 备份状态 / 任务历史 / AI 助手页 / 设置页。

need to design

---

### 12. CLI 命令（**未确定**，请二选或补充）

已确认核心：`service list/deploy/backup/restore/migrate`、`node list`、`prune`、`status`、`caddy reload`、`dns update`。

need to design and add more

---

### 13. 其他已确定但细节待细化的

- Git 同步：main 检测到更新后**自动拉取**
- 失败恢复：main 挂了就接受“手动操作 compose”的现实
- 状态数据库表：节点/服务/任务/备份/心跳（具体字段等你写代码时再定）
