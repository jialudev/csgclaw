# Multica 分层架构参考分析

> - 分析日期：2026-07-24
> - Multica 基线：`ecce589867b949a5a1751a69685a0a61b76e606f`
> - 最新提交：`MUL-5265: GitHub API-snapshot PR cards — CI status + mergeability (#5889)`
> - 范围：Monorepo 前端、Go Server、Daemon/Runtime 及代码级依赖边界
> - 用途：帮助前端开发者理解分层，并作为 CSGClaw 架构演进参考

关联文档：

- [CSGClaw 前端架构评估](web/architecture-assessment.zh.md)
- [CSGClaw 后端架构评估](backend-architecture-assessment.zh.md)

## 1. 总结

### 1.1 一句话结论

Multica 的前端分层值得重点参考；Go 后端是务实的模块化单体，但不是教科书式 Clean Architecture。

最值得 CSGClaw 学习的部分：

1. Web 和 Desktop 的路由入口很薄，业务页面放在共享 `views`。
2. `core`、`ui`、`views` 的依赖方向明确。
3. Navigation、Storage 等平台差异通过 Adapter 注入。
4. React Query、Zustand、URL 和 Context 的状态职责写得很清楚。
5. WebSocket 更新 Query Cache，不建立全局页面 Controller 回调总线。

不能直接当作标准答案的部分：

1. Go Server 没有独立、完整的 Application/Domain/Infrastructure 目录。
2. `cmd/server/main.go` 和 `router.go` 仍承担大量装配。
3. Handler 既调用 Service，也大量直接调用 sqlc Queries。
4. `handler`、`service/task.go`、前端 `core/api/client.ts` 和 Realtime 已出现新的大型中心文件。
5. 文档声明的少数硬边界与当前代码存在偏差。

因此，应把 Multica 看成：

```text
前端：较成熟的 Package Boundary 案例
后端：大型 Go 单体的真实演化案例
而不是：所有目录都应照抄的项目模板
```

### 1.2 分层到底是什么

分层不是先建七个目录，而是回答四个问题：

1. 这个模块为什么会变化？
2. 谁拥有业务规则和状态？
3. 谁可以 import 谁？
4. 外部框架或 SDK 被替换时，哪些业务代码不应跟着变化？

当这些答案稳定后，才决定用文件、目录、Go Package、Workspace Package 或 Interface 来表达。

## 2. 用前端概念理解后端分层

### 2.1 名词翻译

| 后端层           | 前端类比                                       | 直观理解                         |
| ---------------- | ---------------------------------------------- | -------------------------------- |
| API Adapter      | Route Component、表单提交入口、Response Mapper | 把外部输入转换成内部调用         |
| CLI Adapter      | Web、Desktop、Mobile 这些不同入口              | 同一业务的另一套交互入口         |
| Application      | Page Controller、Use Case Hook                 | 编排一次完整用户操作             |
| Domain           | 纯 Model、Reducer、状态机和校验规则            | 与 React、HTTP、数据库无关的业务 |
| Runtime Contract | `NavigationAdapter`、`StorageAdapter`          | 上层声明“我需要什么能力”         |
| Infrastructure   | `fetch`、WebSocket、localStorage、SDK          | 真正连接浏览器、数据库和外部服务 |
| API Contract     | DTO、Zod Schema、OpenAPI Types                 | 跨网络传输的数据协议             |

### 2.2 一个完整类比

前端创建 issue：

```text
CreateIssueDialog                  # View
  -> useCreateIssue()              # Application/use-case hook
     -> validateIssueDraft()       # Domain rule
     -> api.createIssue(dto)       # API/infrastructure adapter
        -> fetch("/api/issues")    # 外部 I/O
  -> IssueResponseSchema.parse()   # API contract
  -> Query Cache 更新              # Server state projection
```

后端创建 issue：

```text
CreateIssue Handler               # API Adapter
  -> CreateIssue Use Case         # Application
     -> Issue rule/state          # Domain
     -> IssueRepository.Create    # Port
        -> PostgreSQL/sqlc        # Infrastructure
  -> CreateIssueResponse          # API Contract
```

两边结构是同一种思想：View/HTTP 只负责边界，业务流程由 Use Case 组织，纯规则不依赖框架，外部 I/O 放在 Adapter。

### 2.3 是否必须体现在代码目录

分三种强度：

| 强度       | 表达方式                                 | 约束效果                       |
| ---------- | ---------------------------------------- | ------------------------------ |
| 概念分层   | 只写在文档里                             | 最弱，容易随时间失效           |
| 文件/目录  | `handler/`、`service/`、`storage/`       | 人能看懂，但仍可能反向 import  |
| 编译级边界 | Go Package、Workspace Package、Interface | 最强，可以被编译器和 Lint 检查 |

最佳实践不是“每一层一个目录”，而是关键边界能够被代码工具检查。

例如，一个小型 Task 领域可以先写成：

```text
internal/task/
  model.go        # Domain
  service.go      # Application
  ports.go        # Repository/Notifier contract
  errors.go
```

等 PostgreSQL、本地文件等实现需要独立变化时，再拆：

```text
internal/task/
internal/taskstore/postgres/
internal/taskstore/local/
```

## 3. Multica 当前系统架构

### 3.1 系统级架构图

```text
+-------------------------------- Client Surfaces -------------------------------+
|                                                                                |
| Next.js Web        Electron Desktop        Expo Mobile        multica CLI      |
+---------+------------------+--------------------+-------------------+------------+
          |                  |                    |                   |
          | shared           | shared             | independent UI    |
          v                  v                    |                   |
+------------------- Frontend Shared Packages ----------------------+             |
| @multica/views -> @multica/core + @multica/ui                    |             |
+-------------------------------+-----------------------------------+             |
                                | HTTP / WebSocket                                 |
                                v                                                  |
+----------------------------- Go Server ----------------------------------------+
| cmd/server -> router -> handlers -> services/sqlc -> PostgreSQL                 |
|                         |          |                                             |
|                         |          +-> Realtime / Redis                          |
|                         +-> Lark / Slack / GitHub / VCS / Composio              |
+-------------------------------+------------------------------------------------+
                                |
                                | task claim / heartbeat / result
                                v
+--------------------------- Local Execution -----------------------------------+
| multica daemon -> runtime registry -> Claude/Codex/OpenClaw/... -> workspace    |
+-------------------------------------------------------------------------------+
```

### 3.2 三个运行时边界

| 边界         | 主要职责                                      |
| ------------ | --------------------------------------------- |
| Client       | 展示、交互、缓存、客户端偏好和平台导航        |
| Go Server    | 工作区、issue、task、权限、协作数据和实时广播 |
| Local Daemon | 在用户机器上领取任务并调用真实的 AI CLI       |

Server 保存协作和任务真相，但不直接执行编码 Agent。Daemon 才拥有本地目录、CLI 登录态和实际进程。

## 4. Multica 前端分层

### 4.1 当前前端架构图

```text
+---------------------------- Platform Apps ------------------------------------+
|                                                                                |
| apps/web                     apps/desktop                      apps/mobile      |
| Next.js routing              Electron/router                  Expo/RN          |
| cookies/server APIs          IPC/window/tabs                   own UI/state     |
+------------+-----------------------+-------------------------------------------+
             | web + desktop share   |
             v                       v
+----------------------------- Shared Views ------------------------------------+
| packages/views                                                                 |
| Business pages, forms, modals and shared layouts                               |
| No Next.js or React Router dependency                                          |
+----------------------+-------------------------------+-------------------------+
                       |                               |
                       v                               v
+-----------------------------+      +-------------------------------------------+
| packages/core               |      | packages/ui                               |
|                             |      |                                           |
| API client + Zod schemas    |      | Atomic components                         |
| React Query hooks/cache     |      | Tokens, styles and UI primitives          |
| Zustand client state        |      | No business/core dependency               |
| Realtime cache sync         |      |                                           |
| Platform contracts          |      |                                           |
+--------------+--------------+      +-------------------------------------------+
               |
               | HTTP / WebSocket
               v
+----------------------------- Go Server ----------------------------------------+
```

依赖方向：

```text
apps/web --------+
apps/desktop ----+-> views -> core
                 |         -> ui
                 +-------> core + ui

ui   -X-> core
core -X-> ui
views -X-> next/react-router
```

### 4.2 各目录实际职责

| 目录             | 角色                      | 典型内容                                      |
| ---------------- | ------------------------- | --------------------------------------------- |
| `apps/web`       | Next.js Platform Adapter  | App Router、Cookie、Server API、Web Provider  |
| `apps/desktop`   | Electron Platform Adapter | IPC、Window、Tab、React Router                |
| `apps/mobile`    | 独立 Mobile Client        | React Native UI、State、Release               |
| `packages/views` | 共享业务 View             | IssuesPage、AgentsPage、DashboardLayout       |
| `packages/core`  | Headless Application      | API、Schema、Query、Mutation、Store、Realtime |
| `packages/ui`    | UI Primitive              | Button、Dialog、Table、Token、Markdown        |

### 4.3 一条页面数据流

Multica Web 的 Issues 路由非常薄：

```text
apps/web/.../issues/page.tsx
  -> <IssuesPage /> from packages/views
     -> useIssueQuery/useIssueMutation from packages/core
        -> ApiClient + Zod Schema
           -> Go REST API

WebSocket
  -> packages/core/realtime
  -> setQueryData / invalidateQueries
  -> IssuesPage 自动观察新缓存
```

这与 CSGClaw 当前全局 Workspace Controller 的差别是：

```text
Multica:
Route -> Feature View -> Feature Query/Mutation -> Shared Cache

当前 CSGClaw:
Route -> Global Workspace Context -> All Feature Controllers -> View Props
```

### 4.4 Multica 如何处理 Workspace

Multica 仍有 Workspace Layout，但它没有成为所有 Feature 的业务 Controller。

Web Workspace Layout 主要负责：

- 从 URL 获取 `workspaceSlug`。
- Auth 和 Onboarding Gate。
- 用 React Query 解析 Workspace。
- 向子树提供 Workspace Slug。
- 同步 Platform Header/Cookie 等 Web 特有信息。

Issues、Agents、Projects、Chat 等 Page 分别调用自己的 Query 和 Mutation。

这说明：

> 共享 Workspace Layout 不等于必须共享 Workspace Business Controller。

### 4.5 Adapter 的真实例子

Navigation 是最容易理解的例子：

```text
packages/views
  -> NavigationAdapter
       |- apps/web/platform/navigation.tsx      -> Next.js Router
       `- apps/desktop/.../platform/navigation  -> React Router + Electron Tabs
```

共享 View 只知道：

```ts
navigation.push(path);
```

它不知道底层是 Next.js 还是 Electron。这就是 Runtime/Infrastructure Contract 在前端的实际形式。

Storage 同理：

```text
packages/core Store
  -> StorageAdapter
     -> Browser/Electron storage implementation
```

### 4.6 状态所有权

Multica 的状态规则值得直接借鉴：

| 状态                 | 所有者                   |
| -------------------- | ------------------------ |
| Issue/Agent/User     | React Query              |
| Workspace Identity   | URL，Platform 层只做镜像 |
| Filter/Draft/Modal   | Zustand                  |
| Navigation           | Navigation Adapter       |
| Realtime Server Data | React Query Cache        |
| Platform Plumbing    | 小型 Context             |

它避免把全部状态塞入一个 Global Context。

### 4.7 前端分层并不完美

代码已经出现新的集中风险：

- `packages/core/api/client.ts` 约 2,952 行。
- `packages/core/realtime/use-realtime-sync.ts` 约 1,467 行。
- `packages/core/issues/mutations.ts` 超过 1,100 行。
- 多个 Issues/Agent View 超过 1,000 行。
- `packages/views/search/search-store.ts` 直接创建 Zustand Store。
- 个别 View 直接导入 Zustand，和“Views 不拥有 Store”的书面规则存在偏差。
- `packages/core/platform/storage.ts` 提供默认 localStorage Adapter，说明“Core 零 localStorage”在代码中实际存在一个受控平台例外。

这说明 Package Boundary 能防止跨层污染，但不能自动解决包内部越来越大的问题。

## 5. Multica Go 后端分层

### 5.1 当前后端架构图

```text
+------------------------------- External --------------------------------------+
|                                                                                |
| Web / Desktop / Mobile / CLI             Lark / Slack / GitHub / VCS           |
+--------------------+--------------------------------------+--------------------+
                     | HTTP / WebSocket                     | webhook / SDK
                     v                                      v
+-------------------------- Server Entrypoint ----------------------------------+
| cmd/server/main.go + router.go                                                 |
|                                                                                |
| (!) Composition Root: config / DB / Redis / middleware / realtime /            |
|     integrations / services / handlers / routes / lifecycle                    |
+----------------------------------+---------------------------------------------+
                                   |
                                   v
+-------------------------- Middleware + Handler -------------------------------+
| internal/middleware + internal/handler                                         |
|                                                                                |
| HTTP decode/encode / auth / validation / response                              |
| (!) 部分用例编排、权限和业务判断                                                |
+----------+-----------------------+----------------------+----------------------+
           |                       |                      |
           | path A                | path B (!)           | side effects
           v                       |                      v
+------------------------------+   |     +---------------------------------------+
| internal/service             |   |     | integrations / realtime / storage     |
|                              |   |     |                                       |
| Task / Issue / Autopilot     |   |     | Lark / Slack / GitHub / VCS           |
| Application + Domain 混合    |   |     | WebSocket Hub / Redis Relay / Files    |
+---------------+--------------+   |     +-------------------+-------------------+
                |                  |                         |
                |                  v                         |
                |       +--------------------------+         |
                +------>| pkg/db + generated sqlc  |<--------+
                        | SQL queries / DB models  |
                        +------------+-------------+
                                     |
                                     v
                              +--------------+
                              | PostgreSQL   |
                              +--------------+


+---------------------------- Local Execution ----------------------------------+
|                                                                                |
| cmd/multica                                                                   |
|      |                                                                         |
|      v                                                                         |
| internal/daemon <---- REST / WebSocket task protocol ----> Go Server           |
|      |                                                                         |
|      v                                                                         |
| dispatch / runtimeapps -> pkg/agent                                             |
|                              |                                                  |
|                              +-> Claude / OpenClaw / other CLI                  |
|                              |                                                  |
|                              `-> codex app-server --listen stdio://             |
|                                      <-> JSON-RPC 2.0 over stdin/stdout         |
|                                      -> local workspace / child processes      |
+--------------------------------------------------------------------------------+
```

图中的两条数据库路径都真实存在：

```text
path A: Handler -> Service -> sqlc -> PostgreSQL
path B: Handler ----------> sqlc -> PostgreSQL
```

`(!)` 不是表示代码错误，而是表示职责已经超过该边界的理想范围。Server 负责协作数据和任务状态，本地 Daemon 通过协议领取任务，再调用具体 Agent CLI；因此 Agent 进程不属于 HTTP Server 的内部调用层。

Codex 的具体调用链是：

```text
Multica Daemon
  -> pkg/agent/codex.go
  -> spawn: codex app-server --listen stdio://
  -> JSON-RPC initialize
  -> initialized notification
  -> thread/resume（存在历史 thread 时）
       `-> 失败后回退 thread/start
  -> turn/start(prompt, model, reasoning effort, service tier)
  <- item/*、turn/* notifications
  <- turn/completed + usage
  -> 关闭 stdin / 清理 Codex 进程组
```

Multica 没有重新实现 Codex Agent，也不是直接调用 OpenAI Chat Completions API。它实现的是一个 Codex App Server Host：负责进程生命周期、任务级 `CODEX_HOME`、MCP 配置、会话恢复、超时、事件转换和审批响应。Daemon 模式下没有人工审批界面，因此当前代码会自动接受命令执行、文件修改和已识别的权限请求。

### 5.2 Multica 与 CSGClaw 的 Codex 并发模型

两者都能同时运行多个 Codex App Server，但并发单位不同：

```text
Multica：Task 是并发单位

Server Task Queue
        |
        | batch claim: 可用 slots
        v
Daemon Semaphore (max_concurrent_tasks，默认 20)
        |
        +-- goroutine: Task A -> task workspace A -> Codex App Server A
        |
        +-- goroutine: Task B -> task workspace B -> Codex App Server B
        |
        `-- goroutine: Task C -> task workspace C -> Codex App Server C
```

Multica 每个已领取 Task 独立执行，Daemon 使用显式 Semaphore 控制整台机器的任务并发数。任务级工作目录、`CODEX_HOME` 和 App Server 生命周期使 Task 之间形成隔离；如果多个任务指向同一个本地目录，则额外使用路径锁避免同时写入。

```text
CSGClaw：Agent / Participant 是并发单位

                         +-> Bot Worker A queue（串行）-> Runtime A -> App Server A
Room / Message Dispatch -+-> Bot Worker B queue（串行）-> Runtime B -> App Server B
                         `-> Bot Worker C queue（串行）-> Runtime C -> App Server C
```

CSGClaw 的 `appServerManager` 用 `RuntimeID` 保存多个 App Server Session，因此不同 Codex Agent 可以同时运行。每个 Codex Bridge Bot 则只有一个消费 goroutine：

```text
同一个 Agent:
Message A -> running
Message B -> queue
Message C -> queue

不同 Agent:
Agent A / Message A -> running
Agent B / Message B -> running
Agent C / Message C -> running
```

此外，CSGClaw App Server 内部可以为不同 Room/Thread 建立多个 Codex Thread，但 Bridge 仍按 Bot 串行消费消息；底层能力没有直接转换成“同一个 Agent 多 Turn 并行”。

因此，CSGClaw 只使用一个 manager 时，确实不能像 Multica 一样自动把多个 Task 扇出为多个 Codex 进程。要并发，需要预先创建多个 Codex Worker，并把工作分别派发给不同 Participant。当前缺少 Multica 那种统一的 Task Queue、批量领取、全局并发槽和任务级进程隔离。

两边默认都关闭单个 Codex 内部的原生 `features.multi_agent`，避免父 Turn 完成时子 Agent 生命周期尚未收敛：

- Multica 默认关闭，但允许通过 `MULTICA_CODEX_MULTI_AGENT=1` 显式开启。
- CSGClaw 当前会在运行时配置中固定写入 `features.multi_agent = false`，没有对应的用户级逃生开关。

### 5.3 实际目录职责

| 目录                           | 实际角色                                     |
| ------------------------------ | -------------------------------------------- |
| `server/cmd/*`                 | 可执行入口和大量应用装配                     |
| `server/internal/handler`      | HTTP Adapter，但也包含部分应用逻辑和 DB 调用 |
| `server/internal/service`      | Task/Issue/Autopilot 核心流程                |
| `server/internal/integrations` | Lark、Slack、VCS、GitHub、Composio Adapter   |
| `server/internal/realtime`     | WebSocket Hub 与 Redis Relay                 |
| `server/internal/daemon`       | 本地任务领取和 Runtime 管理                  |
| `server/pkg/db`                | SQL、sqlc 生成代码和 DB Model                |
| `server/pkg/agent`             | 多种 Agent CLI 的统一执行实现                |

### 5.4 它不是严格 Clean Architecture

代码证据：

- `cmd/server/main.go` 约 556 行。
- `cmd/server/router.go` 约 1,622 行，负责大量 Service、Handler 和 Integration 装配。
- `internal/handler` 有约 86 个非测试 Go 文件。
- 约 62 个生产 Handler 文件直接调用 `h.Queries`。
- `internal/service` 只有约 10 个非测试文件，覆盖最复杂的跨步骤流程。
- `internal/service/task.go` 约 4,493 行。
- Handler Struct 持有 Queries、多个 Service、Storage、Realtime、Integration 和可选 Store。

真实调用既有：

```text
Handler -> Service -> sqlc
```

也有：

```text
Handler -> sqlc
```

所以 Multica 后端更准确的描述是：

```text
Handler + Service + sqlc 的模块化单体
```

而不是：

```text
Adapter -> Application -> Domain -> Port -> Infrastructure
```

### 5.5 后端值得借鉴的部分

- 使用 `cmd` 区分 Server、CLI、Migration 和 Backfill。
- 使用 `internal` 保护应用私有包。
- sqlc 让 SQL 与生成代码边界清楚。
- Integration 按 Lark、Slack、VCS 等能力分包。
- Task 的复杂生命周期集中在 Service，而不是散落于所有 Handler。
- Realtime、Events、Daemon 和 Agent Runtime 有可识别边界。
- 测试与代码同目录，复杂 Handler/Service 有大量回归测试。

### 5.6 不应照搬的部分

- 不应把 1,600 行 Router 当成理想 Composition Root。
- 不应让新 Handler 默认直接持有所有 DB Queries。
- 不应以一个 4,000 行 Task Service 作为最终领域边界。
- 不应因为 Multica 使用 `pkg/agent`，就把所有 Runtime 代码都放入公共 `pkg`。
- Multica 的“数据库不使用 Foreign Key”是其项目决策，不是通用 Go 最佳实践。

## 6. GitHub 项目中的常见实践

### 6.1 Go 官方只规定少数结构

Go 官方的 [Organizing a Go module](https://go.dev/doc/modules/layout) 建议：

- 大型 Server 可以使用 `cmd/` 放多个可执行入口。
- 应用私有代码优先放入 `internal/`。
- Go Package 是最基本的代码和编译边界。
- Server 项目内部怎样继续分层，官方没有规定唯一答案。

因此，`Application`、`Domain`、`Infrastructure` 不是 Go 语言要求，而是复杂业务项目常用的设计方法。

### 6.2 不存在“官方标准目录模板”

[golang-standards/project-layout](https://github.com/golang-standards/project-layout) 很流行，但它自己的 README 明确说明：

- 它不是 Go 核心团队定义的官方标准。
- 它只收集常见目录模式。
- 小项目照搬全部目录会过度设计。
- 它不规定 Clean Architecture 内部应怎样组织。

所以不能根据 GitHub Star 数量，把一个目录模板当成语言规范。

### 6.3 严格分层参考

[ThreeDotsLabs/wild-workouts-go-ddd-example](https://github.com/ThreeDotsLabs/wild-workouts-go-ddd-example) 是专门演示 DDD、Repository、Ports & Adapters、CQRS 和 Clean Architecture 的教学项目。

它适合学习：

- Interface 应由哪一层声明。
- Application Command/Query 如何组织。
- Domain 怎样保持纯净。
- Database Adapter 如何实现 Repository Port。

但它是教学项目，不应把全部 DDD/CQRS 形式机械复制到 CSGClaw。

### 6.4 大型真实项目通常更务实

- [Grafana 的 Go Services](https://github.com/grafana/grafana/tree/main/pkg/services) 更偏按业务能力组织。
- [Kubernetes Commands](https://github.com/kubernetes/kubernetes/tree/master/cmd) 使用 `cmd` 区分可执行入口，但内部结构由自身领域复杂度决定。
- Multica 同样采用 `cmd + internal + service + integrations`，没有强制每个领域都套七层目录。

现实项目的共同点不是目录完全一致，而是入口、领域能力、外部适配和依赖方向能够被团队理解并长期维护。

## 7. 对 CSGClaw 的参考价值

### 7.1 前端可以直接借鉴

| Multica 做法                  | CSGClaw 对应演进                                      |
| ----------------------------- | ----------------------------------------------------- |
| App Route 很薄                | Route Page 自己拥有 Page Controller，不依赖大 Context |
| Views 调用 Feature Query      | Agent/Task/Hub Page 调用自己的 Query Hook             |
| React Query 共享 Server State | Sidebar 与 Page 使用同一 Query Key                    |
| Realtime 更新 Query Cache     | 删除 Workspace Realtime Callback Bus                  |
| Navigation Adapter            | 多平台出现后再抽象，当前先保持 Router Model           |
| UI 与 Core 独立               | `components/ui` 保持零业务/API 依赖                   |
| Context 只做 Plumbing         | 只保留 Auth、Workspace Identity 等窄 Context          |

CSGClaw 当前只有一个 Web App，不需要为了模仿 Multica 立刻拆成 pnpm Monorepo。通过目录、Alias 和 ESLint Import Rule 也能获得大部分收益。

### 7.2 后端应借鉴原则，不照搬现状

建议 CSGClaw 继续采用：

```text
cmd/                    # 很薄的可执行入口
cli/                    # CLI Adapter
internal/api/           # HTTP Adapter
internal/app/           # Bootstrap + 跨领域 Use Cases
internal/<domain>/      # Model、Rule、Service、Port
internal/<integration>/ # Channel/SDK Adapter
internal/<store>/       # Local/File/Database 实现
```

但不要求每个领域都有七个子目录。以 Task 为例：

```text
internal/taskcore/
  model.go
  transitions.go
  service.go
  repository.go

internal/taskstore/
  local.go

internal/app/teamusecase/
  create_team_task.go
```

其中：

- Task 状态机留在 `taskcore`。
- Team Task 跨 Participant/Room/Task 的流程进入 Application Use Case。
- Local Store 实现 Task Repository。
- API 只做 HTTP 转换。

### 7.3 判断是否需要拆层

出现以下信号时才值得拆：

- 同一业务流程需要被 HTTP、CLI、后台任务共同调用。
- Handler 开始保存跨请求状态。
- Domain Test 必须启动 HTTP 或数据库。
- Runtime 构造器持续增加具体 Channel/SDK 参数。
- 修改一个业务规则需要同时改多个入口。
- Package 因 import cycle 被迫使用 `map[string]any` 或全局变量。

如果只是一个简单 CRUD，`Handler -> Repository` 可以是合理的，不需要人为增加空 Service。

## 8. 分层检查清单

### 8.1 对人类

- 能否用一张图说明一次请求经过哪些边界？
- 每个业务状态是否只有一个权威所有者？
- 新功能应该放哪里是否容易判断？
- 替换数据库、Router 或 Channel SDK 时，影响范围是否明确？

### 8.2 对代码

- Import 方向能否通过编译器或 Lint 检查？
- API/CLI 是否依赖稳定 Contract，而非内部 Service Struct？
- Domain Test 是否可以不启动 HTTP、文件系统和数据库？
- Realtime 是否更新数据 Cache，而不是操作多个 Page Controller？
- Composition Root 是否只负责构造和生命周期？
- 可选依赖是否显式，必需依赖是否在启动时验证？

### 8.3 对团队

- 架构规则是否写入仓库级开发规范？
- 测试是否跟随真正拥有规则的 Package？
- 例外是否记录原因和退出条件？
- 文档描述是否与实际 Import 保持一致？

## 9. 最终判断

对前端开发者来说，可以先记住：

```text
Adapter        = 输入输出边界
Application    = 完成一次用户操作的编排
Domain         = 不依赖框架的业务规则
Contract/Port  = 上层需要的能力接口
Infrastructure = 外部系统的具体实现
```

Multica 前端把这些边界较好地落实成了 `pnpm` workspace package；Multica 后端则展示了大型业务快速增长后，Handler、Service 和 Composition Root 会如何重新变重。

CSGClaw 应学习 Multica 前端的依赖约束和 Adapter 方法，同时避免复制其后端已经出现的中心化问题。
