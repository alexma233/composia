# 为什么选择 Composia？

Composia 面向这样一类用户：他们需要一个真正的 Docker Compose 控制平面，但又不想失去对文件、CLI 工作流和基础设施本身的直接掌控。

## 核心宗旨

Composia 围绕一条原则构建：

**平台应该帮助你管理 Docker Compose 基础设施，而不是把你锁进平台私有抽象里。**

这也是为什么 Composia 坚持用纯文件保存期望状态，尽量贴近标准 Docker Compose 工作流，并把控制平面定位成增强层，而不是唯一入口。

这在实际中意味着：

- 只看仓库本身，你仍然应该能理解系统的期望状态
- 你仍然应该能使用相应的 CLI 工具直接操作服务
- 即使未来不再使用 Composia，你也不需要先反向理解一套私有平台模型
- 控制平面负责协调、校验、执行和汇总，而不是夺走操作者对系统的主权

## Composia 是什么

Composia 是一个面向自托管场景的 Docker Compose 控制平面，核心围绕这些能力展开：

- Git-backed 的服务定义
- 单一控制平面
- 一个或多个执行代理
- 多节点服务部署
- 任务执行、repo 写入、secret、备份、恢复和运行态可见性

它不只是一个 `compose.yaml` 的 Web 编辑器，也不是一个自托管版云 PaaS。

## 为什么不直接用 Compose 管理器？

像 Dockge、Dockman 这样的项目，在“更舒服地管理 Compose 文件”这件事上做得很好。

它们通常更关注：

- 在浏览器里方便地编辑 `compose.yaml`
- 快速启动、停止、更新 stack
- 保留对 Compose 文件的直接访问
- 以较轻量的方式管理单机或少量主机

这些都很有价值，但 Composia 解决的是更高一层的问题。

Composia 面向的是这样一些场景：你需要的不只是 stack UI，而是：

- controller-agent 架构，而不是直接在本机改 stack
- 一个可以把同一个 service 定向到多个 node 的模型
- 结构化的任务执行和日志体系
- 带 revision 校验的 Git-backed desired state 变更
- 覆盖 service、instance、container、node 的统一系统视图

可以简化成一句话：

- Dockge 和 Dockman 让你更舒服地管理 Compose
- Composia 让你把 Compose 当成受控基础设施来运行

## 为什么不直接用自托管 PaaS？

像 Dokploy、Coolify 解决的是另一类问题。

它们通常把自己定位为 Heroku、Netlify、Vercel 一类平台的自托管替代品，核心价值一般在于：

- 从 Git 仓库部署应用
- 统一管理应用、数据库、域名、证书、模板
- 提供更强的平台自动化和更接近 PaaS 的体验
- 让平台承担更多应用部署模型本身的复杂度

当你的目标是“拥有一个部署平台”时，这种取向是合理的。

但 Composia 刻意不采用这套产品哲学。

Composia 不假设“平台应该成为应用模型的唯一主入口”。相反，它从这些前提出发：

- Docker Compose 仍然是运行层的基础
- 文件仍然是期望状态的真相来源
- 操作者应该保有对底层系统的直接控制
- 控制平面应该是可移除的

也可以直接总结成：

- Dokploy 和 Coolify 是自托管 PaaS 平台
- Composia 是面向自托管基础设施的 Compose 原生控制平面

## Composia 处在什么位置

Composia 处在两类常见方案之间：

- 更轻量的 Compose 管理器
- 更高层的自托管 PaaS 平台

它保留了很多人真正想要的 Compose 原生、文件优先、操作者可控模型，同时补上了轻量 stack 管理器通常不提供的控制平面能力。

这正是 Composia 想站的位置。

## 如果你想要这些，Composia 更合适

- 一个多节点 Docker Compose 控制平面
- Git-backed 的期望状态，而不是不透明的平台内部状态
- 继续使用正常 CLI 和纯文件工作流
- 清晰的 service、instance、container、node 边界
- 在不失去底层控制权的前提下，获得协调、可观测性和运维能力

## 总结

如果说 Dockge 或 Dockman 更像“更好的 Compose 操作界面”，Dokploy 或 Coolify 更像“自托管 PaaS 平台”，那么 Composia 想做的是另一件事：

**一个平台无关的 Docker Compose 控制平面，在增强协调与运维能力的同时，不拿走你对基础设施本身的控制权。**
