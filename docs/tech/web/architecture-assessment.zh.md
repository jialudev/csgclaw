# CSGClaw 前端架构评估

> - 评估日期：2026-07-24
> - 代码基线：`f6559fed`（`upstream/main`）
> - 范围：`web/app`、前端构建链路及其与 Go API 的边界
> - 结论性质：架构演进建议，不要求一次性重写

关联文档：

- [后端架构评估](../backend-architecture-assessment.zh.md)
- [Multica 分层架构参考分析](../multica-architecture-reference.zh.md)
- [Mantine 采用与渐进迁移方案](mantine-adoption-plan.zh.md)

## 1. 总结

### 1.1 一句话结论

CSGClaw 前端仍应保持一个 Vite + React 单体应用，但需要把业务所有权从全局 `Workspace` 归还给各个路由页面。

> 保留 Workspace 的视觉 Shell，删除 Workspace 的全局业务 Controller。

当前的主要问题不是目录数量，而是实际运行时边界与目录表达不一致：

- 表面上已经有 Conversation、Agent、Team、Tasks、Hub 等独立 Page。
- 实际上这些 Page 多数只是从 `WorkspaceControllerContext` 获取组装好的 View Props。
- 数据加载、mutation、实时同步、跨页面协调和 Overlay 仍集中在 `useWorkspaceController`。
- 因此，页面被拆开了，业务所有权并没有真正拆开。

### 1.2 核心决策

| 决策                         | 处理方式                                                 |
| ---------------------------- | -------------------------------------------------------- |
| `WorkspacePage`              | 保留父路由能力，改造成精简的 `AppShellRoute`             |
| `WorkspaceLayout`            | 保留，只负责 Sidebar、Outlet、布局和可访问性             |
| `useWorkspaceController`     | 逐步删除                                                 |
| `WorkspaceControllerContext` | 逐步删除，禁止替换成另一个大 `AppContext`                |
| 页面数据和 mutation          | 移入对应 Route Page Controller                           |
| 共享服务端数据               | 由 React Query Cache 负责共享和去重                      |
| URL 可表达的选择状态         | 继续以 URL 为唯一真相                                    |
| Realtime                     | 只更新 Query Cache 或专用 Event Store                    |
| 全局 UI 状态                 | 仅保留 theme、locale、sidebar、floating overlay 等窄状态 |
| 微前端                       | 不引入；当前问题是模块边界，不是部署边界                 |

### 1.3 优先级

1. **P1：拆除全局 Workspace 业务中心。**
2. **P1：消除 Page 之间的私有模块引用和依赖环。**
3. **P1：把浏览器副作用移出纯 Model。**
4. **P1：拆分 Agent、Conversation、Tasks 等大 Controller/View。**
5. **P2：统一 mutation、storage、realtime 和错误处理边界。**
6. **P2：增加 import 规则和关键状态机测试，防止架构继续退化。**

## 2. 当前架构

### 2.1 图的阅读方式

本文使用适合代码仓库长期维护的 ASCII 组件图：

- 大框表示稳定的系统或运行时边界。
- 框内表示同一生命周期中的组件。
- 实线箭头表示主要调用或数据流。
- `(!)` 表示当前不合理的依赖或职责集中点。
- 文件名和函数级证据不塞进主图，统一放在后面的详细分析。

这种图比目录树更能表现“谁拥有状态、谁调用谁”，也比类图更适合当前 React 函数组件和 Hook 架构。

### 2.2 当前前端架构图

```text
+---------------------------------- Browser ------------------------------------+
|                                                                                |
|  main.tsx                                                                      |
|     |                                                                          |
|     v                                                                          |
|  AppProviders                  AppRouter                                        |
|  QueryClient / ErrorBoundary      |                                            |
|                                   v                                            |
|  +------------------------- WorkspacePage ----------------------------------+  |
|  |                                                                         |  |
|  |  WorkspaceLayout                                                        |  |
|  |  +-------------+  +----------------------+  +------------------------+  |  |
|  |  | Sidebar     |  | Outlet               |  | WorkspaceOverlays      |  |  |
|  |  |             |  |                      |  |                        |  |  |
|  |  | messages    |  | ConversationPage     |  | floating chat          |  |  |
|  |  | agents      |  | AgentPage            |  | profile preview        |  |  |
|  |  | teams       |  | Human / Team         |  | create dialogs         |  |  |
|  |  | tasks       |  | Tasks / Hub          |  | auth / upgrade notice  |  |  |
|  |  | hub/models  |  | Settings / Models    |  |                        |  |  |
|  |  +------+------+  +----------+-----------+  +-----------+------------+  |  |
|  |         |                    |                          |               |  |
|  |         +--------------------+--------------------------+               |  |
|  |                              |                                          |  |
|  |                              v                                          |  |
|  |            (!) WorkspaceControllerContext                               |  |
|  |                              |                                          |  |
|  |                              v                                          |  |
|  |            (!) useWorkspaceController                                   |  |
|  |                - 8 组根 Query                                            |  |
|  |                - Agent / Conversation / Task / Hub Controller            |  |
|  |                - Auth / Connector / Config / Upgrade                     |  |
|  |                - Human / Provider mutation                              |  |
|  |                - Realtime dispatch                                      |  |
|  |                - 跨面板协调与 View Props 组装                            |  |
|  +------------------------------+------------------------------------------+  |
|                                 |                                             |
|                 +---------------+----------------+                            |
|                 |                                |                            |
|                 v                                v                            |
|         React Query + api/*              Zustand + browser storage            |
+-----------------+--------------------------------------------------------------+
                  |
                  | HTTP JSON / SSE / WebSocket
                  v
+---------------------------- CSGClaw Go API -----------------------------------+
```

### 2.3 当前主数据流

```text
页面加载
  -> WorkspacePage 启动全部根 Query
  -> useWorkspaceController 组装各业务 Controller
  -> Context 发布一个大型 Controller 对象
  -> Route Page 读取对应 View Props
  -> View 渲染

用户操作
  -> View callback
  -> Workspace/Feature Controller
  -> api/*
  -> Go API
  -> invalidate Query 或手工更新多个状态

实时事件
  -> SSE/WebSocket
  -> useWorkspaceRealtime
  -> 调用多个 Controller callback
  -> Context value 变化
  -> 消费 Context 的页面重新观察状态
```

### 2.4 当前架构的关键事实

在本次代码基线上：

- `useWorkspaceController.ts` 约 960 行。
- 根部预先启动 8 组 React Query。
- Controller 对外返回约 28 个顶层字段。
- `sidebarProps` 包含 84 个字段。
- 约 14 个模块直接消费 `WorkspaceControllerContext`。
- 主 Conversation 和 Floating Chat 各自创建一套 Conversation Controller。
- 几乎所有业务路由都位于 `WorkspacePage` 下。

因此，`Workspace` 已不是一个普通页面，而是：

```text
Application bootstrap
+ global query aggregator
+ feature controller registry
+ cross-feature coordinator
+ realtime dispatcher
+ overlay manager
+ view-props assembler
```

这就是“很多逻辑都在 Workspace”的直接原因。

### 2.5 为什么会演化成这样

这不是单次错误，而是一条自然但需要被纠正的演化路径：

```text
共享布局
  -> 共享父路由
     -> 为方便复用而建立共享 Context
        -> 数据被提升到父 Controller
           -> mutation 和 realtime 也被提升
              -> 新功能默认继续进入 Workspace
```

其中“共享布局”和“共享父路由”是合理的；后面的全局业务聚合并不是必然结果。

Git 历史也显示，`WorkspacePage` 和 `useWorkspaceController` 在 Vite/React 现代化阶段一起建立，后续多个功能持续接入这个入口。后来虽然出现了独立 Page 目录，但主要移动的是 View，而不是状态和用例所有权。

## 3. 目标架构

### 3.1 目标前端架构图

```text
+---------------------------------- Browser ------------------------------------+
|                                                                                |
|  AppProviders                                                                  |
|  QueryClient / ErrorBoundary / Theme                                           |
|       |                                                                        |
|       v                                                                        |
|  +-------------------------- AppShellRoute ---------------------------------+  |
|  |                                                                         |  |
|  |  +----------------+    +----------------------+    +-----------------+  |  |
|  |  | Sidebar        |    | Outlet               |    | Shell Features  |  |  |
|  |  |                |    |                      |    |                 |  |  |
|  |  | section query  |    | Route Page           |    | floating chat   |  |  |
|  |  | selectors      |    | owns its use cases   |    | upgrade notice  |  |  |
|  |  +-------+--------+    +----------+-----------+    | preview         |  |  |
|  |          |                        |                +--------+--------+  |  |
|  |          |                        |                         |           |  |
|  |          +------------------------+-------------------------+           |  |
|  |                                   |                                     |  |
|  |  RealtimeSync --------------------+----> QueryClient / Event Store       |  |
|  +-----------------------------------+-------------------------------------+  |
|                                      |                                        |
|                  +-------------------+-------------------+                    |
|                  |                                       |                    |
|                  v                                       v                    |
|  +-------------------------------+       +-------------------------------+    |
|  | Route-owned features          |       | Shared frontend foundations   |    |
|  |                               |       |                               |    |
|  | ConversationPageController    |       | React Query cache             |    |
|  | AgentPageController           |       | narrow UI stores/contexts     |    |
|  | TeamPageController            |       | pure models                   |    |
|  | TasksPageController           |       | business components           |    |
|  | HubPageController             |       | UI primitives                 |    |
|  +---------------+---------------+       +-------------------------------+    |
|                  |                                                             |
|                  v                                                             |
|           domain query hooks -> api/*                                           |
+------------------+-------------------------------------------------------------+
                   |
                   | HTTP JSON / SSE / WebSocket
                   v
+----------------------------- CSGClaw Go API -----------------------------------+
```

### 3.2 目标主数据流

```text
页面加载
  -> Router 只加载当前 Route Page
  -> Page Controller 调用所属领域 Query Hook
  -> React Query 命中缓存或发起请求
  -> Page 观察 Query 结果并渲染

Sidebar 加载
  -> 每个 Sidebar Section 读取自己的摘要 Query
  -> 与 Route Page 使用同一个 Query Key
  -> React Query 自动去重，不复制领域状态

用户操作
  -> Page/Feature mutation
  -> api/*
  -> 更新或失效精确 Query Key
  -> 所有观察者自动获得一致结果

实时事件
  -> RealtimeSync
  -> 解析领域事件
  -> queryClient.setQueryData / invalidateQueries
  -> 当前页面和 Sidebar 自动更新
```

### 3.3 目标职责边界

| 层                   | 应负责                                       | 不应负责                              |
| -------------------- | -------------------------------------------- | ------------------------------------- |
| `AppProviders`       | Query Client、Error Boundary、Theme          | 具体业务 query 和 mutation            |
| `AppShellRoute`      | Sidebar、Outlet、Shell Overlay、RealtimeSync | Agent/Task/Hub 等业务编排             |
| Route Page           | 当前路由的数据、用例、页面状态               | 其他 Page 的私有实现                  |
| Domain Query Hook    | Query key、请求、缓存转换                    | 页面布局                              |
| Model                | 纯转换、规则、展示模型                       | `window`、storage、API、React runtime |
| Business Component   | 稳定的跨页面业务 UI                          | 隐式拥有不透明的全局 mutation         |
| UI Primitive         | 无业务含义的基础交互                         | API、Page、Controller                 |
| Narrow Store/Context | 少量真正全局或瞬时 UI 状态                   | 全站业务 Controller 对象              |

### 3.4 Workspace 最终如何处理

可保留：

- `WorkspaceLayout` 的布局能力。
- Sidebar、Outlet、响应式布局和 Shell 可访问性。
- Theme、locale、sidebar collapsed 等 Shell preference。
- 独立的 Floating Chat、Upgrade Indicator、Profile Preview。

应删除或拆走：

- `useWorkspaceController`。
- `WorkspaceControllerContext`。
- 聚合式 `useWorkspaceData`。
- `hooks/workspace/types.ts` 中跨领域的总类型。
- Workspace 中的 Human、Model Provider、Agent、Task、Hub mutation。
- 通过 realtime callback 直接操作多个 Page Controller 的机制。

建议最终命名：

```text
WorkspacePage       -> AppShellRoute
WorkspaceLayout     -> AppShellLayout
WorkspaceSidebar    -> AppSidebar
WorkspaceOverlays   -> ShellOverlays
WorkspaceRealtime   -> RealtimeSync
```

这里的改名不是第一步。应先移动所有权，最后再改名；否则只是对旧结构进行表面包装。

### 3.5 UI 基础组件如何演进

目标架构中的 `components/ui` 继续作为 CSGClaw 稳定的 UI Contract。Mantine 可以逐步替换其内部的 Radix/原生实现，但 Page 和 Business Component 不应因此直接依赖 Mantine。

推荐顺序：

```text
先拆 Workspace 业务所有权
  + 同期迁移低风险 UI Primitive
  -> AppShellRoute 职责稳定
  -> 最后评估 Mantine AppShell/Sidebar
```

主题映射、CSS Layer、Button/Select/Dialog 兼容风险及详细迁移阶段见：[Mantine 采用与渐进迁移方案](mantine-adoption-plan.zh.md)。

## 4. 当前与目标的差距

| 当前状态                          | 目标状态                          | 主要收益               |
| --------------------------------- | --------------------------------- | ---------------------- |
| 所有 Route 依赖大 Context         | Route 自己拥有 Page Controller    | 页面可独立加载和测试   |
| 根部预取多个领域                  | 当前页面和 Sidebar 按需 Query     | 减少无关请求和启动耦合 |
| Sidebar 接收 84 个 Props          | Section 使用 Query Selector       | 降低父子接口复杂度     |
| Realtime 调用 Controller callback | Realtime 更新 Cache/Store         | 消除隐式回调总线       |
| Page A 引用 Page B 私有模块       | 跨页 UI 提升为 Business Component | 消除依赖环             |
| Model 读取 localStorage           | Storage Adapter 调用纯 Model      | Model 可独立测试       |
| mutation 分散在 Controller 和组件 | mutation 归所属 Page/Feature      | 缓存和错误策略一致     |
| 大 View 内嵌状态机和纯计算        | Page 组合子组件与纯 Model         | 降低变更影响范围       |

## 5. 详细问题与代码归属

### 5.1 P1：Workspace 是全局 Front Controller

涉及：

- `pages/WorkspacePage/WorkspacePage.tsx`
- `hooks/workspace/useWorkspaceController.ts`
- `hooks/workspace/WorkspaceControllerContext.tsx`
- `hooks/workspace/useWorkspaceData.ts`
- `hooks/workspace/useWorkspaceRealtime.ts`

问题：

- 页面生命周期与整个应用生命周期绑定。
- 一个领域的变更会扩大 Context value 和测试装配范围。
- Settings、Models 等页面也受到 IM bootstrap 和其他领域请求影响。
- Route lazy loading 只延迟了组件代码，没有延迟数据和 Controller。

目标位置：

- 业务 Controller 移入 `pages/<Page>/`。
- 可复用领域 Query 移入 `hooks/<domain>/`。
- Realtime 移入 `shared/realtime/` 或精简 Shell。
- Shell preference 留在窄 Store。

### 5.2 P1：Agent Controller 和 Model 聚合过多职责

证据：

- `useAgentController.ts` 约 2,500 行，并使用多个 API 模块。
- 同时处理 Agent 生命周期、Profile 草稿、Manager 重建、模型认证、Feishu 注册、Skills、MCP、Team 创建和模板发布。
- `models/agents.ts` 约 2,255 行，混合 runtime、profile、notification、template、environment 和 presentation 规则。

建议拆分：

```text
pages/AgentPage/
  useAgentPageController.ts
  useAgentLifecycle.ts
  useAgentProfileEditor.ts
  useAgentResources.ts
  useAgentFeishuRegistration.ts
  useAgentTemplateActions.ts

models/
  agentRuntime.ts
  agentProfile.ts
  agentNotification.ts
  agentTemplates.ts
  agentPresentation.ts
```

旧 `useAgentController` 可在迁移期转调新 Hook，但最终组合入口必须归 `AgentPage`。

### 5.3 P1：Page 之间出现私有依赖和依赖环

当前路径：

```text
AgentPage
  -> ConversationPage
     -> ConversationPane
        -> AgentPage/components/AgentView

hooks/workspace/types
  -> AgentPage/components/AgentDetailPaneProps
```

问题：

- Page 不再是依赖树叶节点。
- Controller 公共类型被具体页面组件反向约束。
- Lazy loading 和模块初始化更容易出现隐性问题。

处理：

- 将稳定复用的 Agent Detail 提升到 `components/business/AgentDetail/`。
- Props/Handle 类型跟随该公共组件。
- Agent 路由显示 Conversation 的逻辑改为路由重定向或 Outlet 决策。

### 5.4 P1：纯 Model 包含浏览器副作用

证据：

- `models/routing.ts` 的 workspace group 逻辑读取 `window.localStorage`。
- `models/workspace.ts` 使用 React 展示类型。

处理：

- 浏览器读写移入 `shared/storage/` 或 Shell Hook。
- Model 只接受数据并返回标准化结果。
- UI 样式类型留在组件层。

### 5.5 P1：复杂 View 混合规则、状态和渲染

| 文件                     |   约规模 | 混合职责                                           |
| ------------------------ | -------: | -------------------------------------------------- |
| `TasksView.tsx`          | 2,143 行 | 表单、Board、Timeline、依赖图、任务计算            |
| `AgentDetailPane.tsx`    | 1,902 行 | Profile、runtime、模型、通知、Skills、MCP、Channel |
| `AgentActivityPanel.tsx` | 1,587 行 | 活动请求、过滤、状态和交互                         |
| `HubDetailPane.tsx`      | 1,388 行 | Hub 详情、文件、编辑和操作                         |
| `WorkspaceTabPanels.tsx` | 1,211 行 | 多 Sidebar Section、排序和存储                     |

拆分顺序应是：

1. 先提取纯规则和展示模型。
2. 再按用户能力提取独立子组件。
3. 最后迁移对应 CSS。

不要只按行数机械切文件。

### 5.6 P2：mutation 所有权不一致

当前有些 mutation 在 Workspace Controller，有些直接位于共享业务组件，例如：

- Conversation question response。
- Agent activity decision。
- Runtime directory picker。

默认规则：

- Page 或 Feature Container 拥有 mutation。
- 纯展示组件只接收 `onRespond`、`onDecide` 等回调。
- 确需自管理网络请求时，显式命名为 `*Container`，并记录缓存和错误策略。

### 5.7 P2：Storage 访问策略分散

直接 storage 访问分布在 Workspace、Agent、Auth、Connector、Floating Chat、Conversation 和多个 View 中。

建议：

- `shared/storage` 提供安全的基础读写。
- 每个 Feature 自己定义 key、schema、默认值和版本。
- 跨刷新业务草稿使用独立 Adapter，不放回 Workspace Controller。

### 5.8 P2：样式和测试边界不足

较大的样式岛包括：

- `WorkspaceComponents.css`：约 3,971 行。
- `ConversationPane.css`：约 2,487 行。
- `AgentDetailPane.css`：约 2,391 行。
- `WorkspaceModals.css`：约 2,389 行。

当前约 304 个 TS/TSX 文件对应 15 个测试文件。测试数量不是唯一指标，但关键边界缺少自动保护：

- 没有规则阻止 Page 私有模块互相导入。
- 没有规则阻止 Model 读取浏览器环境。
- Agent、Task 等大状态机缺少足够的纯函数测试。

## 6. 详细实施计划

### 阶段 0：建立护栏

目标：先阻止问题继续扩大。

- 冻结 `useWorkspaceController`，不再加入新业务能力。
- 增加跨层 import 限制。
- 禁止 `pages/A` 导入 `pages/B` 私有目录。
- 禁止 `models` 导入 API、storage 和 React runtime。
- 为 route helper、storage adapter 和关键状态转换补最小测试。

完成标志：

- 新功能必须声明 Route/Feature owner。
- CI 可以发现新增的反向依赖。

### 阶段 1：独立路由脱离 Workspace

目标：验证 Route-owned 架构。

建议顺序：

1. Human。
2. Model Provider。
3. Hub。
4. Tasks。
5. Team。

每个 Page：

- 自己读取 Route Params。
- 自己调用领域 Query Hook。
- 自己拥有 mutation 和错误展示。
- 不再读取完整 `WorkspaceControllerContext`。

旧 Controller 暂时保留为兼容桥，但已迁移页面不得继续消费它。

### 阶段 2：清理确定错位的依赖

- 移走 `models/routing.ts` 的 storage 访问。
- 提升 Agent Detail 公共组件。
- 删除 Page A 到 Page B 私有模块的引用。
- 建立统一 storage primitives。
- 将共享组件内部 mutation 改成显式 Container 或 callback。

### 阶段 3：迁移 Agent 和 Conversation

- 先拆 Agent Profile、Feishu、Skills/MCP 和 Template 子能力。
- 再拆 `models/agents.ts`。
- `AgentPage` 接管 `useAgentPageController`。
- `ConversationPage` 接管 Conversation Controller。
- Conversation 内 Agent Drawer 使用 URL 或独立 Agent Query。
- Floating Chat 保留为 Shell Feature，但拥有独立、窄化的数据入口。

### 阶段 4：拆分 Sidebar 和 Realtime

- Sidebar Section 使用独立 Query Selector。
- 删除 84 字段的聚合 `sidebarProps`。
- RealtimeSync 只更新 Query Cache/Event Store。
- Profile Preview、Upgrade、Floating Chat 分别成为窄 Shell Feature。

### 阶段 5：删除 Workspace 业务中心

- 删除 `WorkspaceControllerContext`。
- 删除 `useWorkspaceController`。
- 删除聚合式 `useWorkspaceData`。
- 将残留 Hook 移入 Page、`hooks/<domain>`、`shared/realtime` 或 Shell。
- 将 `WorkspacePage` 改名为 `AppShellRoute`。

### 阶段 6：持续拆分高变更 View

- 优先处理 TasksView 和 AgentDetailPane。
- 一次只围绕一个用户能力拆分。
- 组件移动时同步迁移其 CSS。
- 不把未知职责重新堆入 `utils/` 或 `common/`。

## 7. 架构验收标准

- `WorkspaceControllerContext` 和 `useWorkspaceController` 已删除。
- App Shell 不导入 Agent、Conversation、Task、Hub 的 mutation Controller。
- Route Page 可以独立加载和测试。
- Sidebar 不接收 80+ 字段的聚合 Props。
- Realtime 只更新 Cache/Store，不调用多个 Page Controller callback。
- Page 私有目录之间没有横向依赖。
- Model 中没有 `window`、storage、API 或 React runtime。
- UI Primitive 保持零业务和零 API 依赖。
- 新增边界规则、typecheck 和 Vitest 全部通过。
- 拆分前后的路由、SSE、Profile 保存和任务操作行为一致。

## 8. 与历史架构图的关系

[历史前端架构图](architecture.md) 仍可作为早期页面分层参考，但它没有覆盖当前的 Human、Team、Tasks、Model Provider、Settings，也没有表达全局 Workspace Controller 的真实运行时地位。

建议：

- 保留历史文档，不用当前技术债覆盖历史背景。
- 本文作为当前评估和目标设计。
- 第一轮 Workspace 拆分完成后，再把目标图升级为正式主架构图。

最终判断：Workspace 可以拆。应保留共享 Shell，删除它作为全站业务中心的角色，并把数据、用例和 mutation 归还给真正拥有它们的 Route Page。
