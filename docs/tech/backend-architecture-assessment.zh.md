# CSGClaw 后端架构评估

> - 评估日期：2026-07-24
> - 代码基线：`f6559fed`（`upstream/main`）
> - 范围：Go CLI、HTTP Server、API、领域服务、Runtime、Sandbox、持久化与装配链路
> - 结论性质：模块化单体演进建议，不建议拆成微服务

关联文档：[前端架构评估](web/architecture-assessment.zh.md)

## 1. 总结

### 1.1 一句话结论

CSGClaw 后端的领域能力已经比较完整，当前最需要的不是增加更多 package，也不是拆微服务，而是建立真正的应用层，让 API、CLI、Domain、Runtime 和基础设施回到各自边界。

> 保持单进程模块化单体，补齐 `internal/app`，统一任务真相，解除 Runtime 对具体 Channel 的反向依赖。

当前职责主要向三个中心泄漏：

- `internal/api.Handler` 同时承担 HTTP Adapter、应用服务和业务状态容器。
- `cli/serve` 同时承担 CLI 命令和整个应用的 Composition Root。
- `internal/runtime` / `internal/agent` 直接依赖 Feishu 具体类型。

此外，`internal/team` 与 `internal/taskcore` 各自实现任务状态、审批、事件、Presence 和持久化，形成两套任务真相。

### 1.2 核心决策

| 决策            | 处理方式                                                   |
| --------------- | ---------------------------------------------------------- |
| 部署模型        | 保持单进程，不拆微服务                                     |
| 应用装配        | 从 `cli/serve` 移入 `internal/app`                         |
| HTTP API        | 只解析请求、调用 Use Case、映射错误和返回响应              |
| 领域状态        | 不保存在 API Handler 中                                    |
| Task            | 以 `taskcore` 为唯一状态机和持久化真相                     |
| Team            | 保留 metadata、成员、planning 和 projection                |
| Runtime/Channel | 通过中立 `ChannelBinding` contract 解耦                    |
| API Contract    | CLI 与 Server 通过 `apitypes` 通讯，不共享内部领域 command |
| Migration       | 建立版本化、幂等、可审计的 migration runner                |
| 大 Service      | 先在现有领域包内按能力拆分，不急于增加 package             |

### 1.3 优先级

1. **P1：把业务状态和持久化从 API Handler 移出。**
2. **P1：统一 Team Task 与 Task Core。**
3. **P1：解除 Runtime/Agent 对 Feishu 具体实现的依赖。**
4. **P2：把应用 Bootstrap 和生命周期从 `cli/serve` 移入 `internal/app`。**
5. **P2：净化 API Contract、Migration 和依赖注入。**
6. **P2：在领域包内部拆分过大的 Service。**

## 2. 当前架构

### 2.1 图的阅读方式

本文采用 ASCII 的组件边界图：

- 横向表示不同入口或领域能力。
- 纵向表示从 Adapter 到业务再到 Runtime/Infrastructure 的调用方向。
- `(!)` 表示职责错位、重复真相或反向依赖。
- 图只表达关键边界和主数据流；具体文件证据放在后面的详细分析。

这种形式比完整 package 依赖图更适合人类阅读。package 级依赖数量会作为证据，但不会全部画入主图。

### 2.2 当前后端架构图

```text
+-------------------------------- External --------------------------------------+
|                                                                                |
|  Web UI        csgclaw CLI       csgclaw-cli       Feishu / CSGClaw Channel   |
|    |                |                 |                       |                 |
+----+----------------+-----------------+-----------------------+-----------------+
     |                |                 |                       |
     v                v                 v                       v
+------------------------------- Entrypoints ------------------------------------+
|                                                                                |
| internal/server   (!) cli/serve       internal/apiclient      channel adapters |
|      |            flags + daemon             |                       |          |
|      |            + app bootstrap            |                       |          |
|      |            + lifecycle                |                       |          |
|      |            + recovery                 |                       |          |
|      v                                                                         |
| (!) internal/api.Handler                                                       |
|     HTTP routing + use-case orchestration + business state + file persistence  |
+-------------------------------+------------------------------------------------+
                                |
              +-----------------+------------------+-------------------+
              |                                    |                   |
              v                                    v                   v
+---------------------------+  +---------------------------+  +-------------------+
| Collaboration / Task      |  | Agent / Runtime           |  | Resources / Model |
|                           |  |                           |  |                   |
| participant -> im         |  | agent -> runtime registry |  | hub / skill / MCP |
| channelbridge             |  |    -> codex               |  | connectors        |
|                           |  |    -> sandboxgateway      |  | model providers   |
| (!) team task state       |  |       -> Pico/OpenClaw    |  | LLM / CLIProxy    |
| (!) taskcore task state   |  |                           |  |                   |
+-------------+-------------+  +-------------+-------------+  +---------+---------+
              |                              |                          |
              |                              | (!) runtime -> Feishu    |
              |                              v                          |
              |                    +----------------------+             |
              |                    | Runtime/Sandbox      |             |
              |                    | BoxLite / Docker     |             |
              |                    | Codex app-server     |             |
              |                    +----------+-----------+             |
              |                               |                         |
+-------------+-------------------------------+-------------------------+---------+
| Infrastructure / Persistence                                                  |
|                                                                               |
| local stores / config / session files / migration / provider SDK / filesystem |
+-------------------------------------------------------------------------------+
```

### 2.3 当前四条主链路

历史架构图主要表达：

```text
channel -> room -> participant -> agent -> runtime -> sandbox
```

这条关系仍然正确，但当前系统实际至少有四条链路。

#### 控制面

```text
Web / CLI
  -> HTTP API
  -> Handler 内编排多个 Service
  -> Config / Agent / Connector / Upgrade / Hub 等能力
```

#### 协作数据面

```text
Channel event
  -> Channel Adapter / Bridge
  -> Room / Participant / IM
  -> Agent Runtime Session
  -> Channel response
```

#### 任务数据面

```text
Team Task -----------------> team.Service 状态机
Agent/Scheduled Task ------> agenttask -> taskcore 状态机
                                      |
                                      v
                              API 层合并两套结果
```

#### 模型数据面

```text
Agent Runtime
  -> bot LLM bridge
  -> llm.Service
  -> Provider / CLIProxy / remote API / CSGHub-lite
```

### 2.4 当前架构的关键事实

在本次代码基线上：

- 约 87 个 Go package。
- 333 个非测试 Go 文件，约 94,312 行生产代码。
- 187 个测试文件，约 79,120 行测试代码。
- `internal/api` 直接导入约 37 个本地 package。
- `cli/serve` 直接导入约 36 个本地 package。
- `internal/api/handler.go` 约 3,274 行。
- `cli/serve/serve.go` 约 1,373 行。
- `internal/team/service.go` 与 `internal/taskcore/service.go` 各自维护任务状态机。

测试基础较好，因此适合做渐进重构，不需要重写。

### 2.5 当前做得合理的部分

- `cmd/csgclaw` 与 `cmd/csgclaw-cli` 入口保持很薄。
- Participant、IM、Team、Agent、Runtime、Sandbox、Skill、MCP 等领域包已经可识别。
- Runtime 和 Sandbox 已有正式接口。
- `internal/apitypes` 与 `internal/apiclient` 已开始分离 API Contract。
- `internal/app/channelwiring` 和 `internal/app/runtimewiring` 已经建立了正确方向的基础。
- 主 Server 使用 chi，gin 仅存在于 CLIProxy SDK 边界；无需为了统一框架改写第三方 SDK。

## 3. 目标架构

### 3.1 目标后端架构图

```text
+-------------------------------- External --------------------------------------+
|                                                                                |
| Web UI        csgclaw CLI       csgclaw-cli       Feishu / other Channels      |
+----+----------------+-----------------+-----------------------+-----------------+
     |                |                 |                       |
     v                v                 v                       v
+-------------------------------- Adapters --------------------------------------+
|                                                                                |
| HTTP API        CLI command       API client/contracts       Channel adapters  |
| decode/encode   flags/output      apitypes only              SDK conversion    |
+-------------------------------+------------------------------------------------+
                                |
                                v
+--------------------------- Application Layer ---------------------------------+
| internal/app                                                                   |
|                                                                                |
| Bootstrap / Dependencies / Lifecycle / Recovery                                |
|                                                                                |
| AgentUseCases          ParticipantUseCases          TeamUseCases               |
| TaskUseCases           RegistrationUseCases         RuntimeProvisioning        |
| ConnectorUseCases      ModelUseCases                Scheduling                 |
+-------------------------------+------------------------------------------------+
                                |
               +----------------+-------------------+
               |                                    |
               v                                    v
+----------------------------- Domain ------------------------------------------+
|                                                                                |
| participant / im       taskcore / team       agent / template / worklease      |
| skill / MCP            activity              connector + channel contracts      |
|                                                                                |
| Rules, state transitions, entities, domain errors and ports                    |
+-------------------------------+------------------------------------------------+
                                |
                                v
+----------------------- Infrastructure Implementations -------------------------+
|                                                                                |
| stores + migration       runtime implementations       sandbox providers       |
| Feishu SDK               LLM providers / CLIProxy      filesystem / processes  |
| Codex / PicoClaw / OpenClaw                         BoxLite / Docker            |
+-------------------------------------------------------------------------------+
```

### 3.2 目标依赖方向

```text
adapter -> application -> domain
                         -> infrastructure ports

infrastructure implementation -> domain/application port
```

禁止：

```text
runtime -> channel/feishu concrete type
domain  -> internal/api
HTTP Handler -> filesystem persistence
CLI HTTP client -> server internal domain command
localstore primitive -> all domain schemas
```

### 3.3 目标主流程

#### 应用启动

```text
cli/serve
  -> 解析 flags 和前后台模式
  -> internal/app.Bootstrap(config)
       -> 构造 Dependencies
       -> 执行 migration/recovery
       -> 装配 Channel/Runtime
       -> 启动 Server 和后台生命周期
  -> 返回 App + Shutdown + Diagnostics
```

#### HTTP 请求

```text
HTTP request
  -> API Adapter 解码
  -> Application Use Case
  -> Domain rule
  -> Port / Infrastructure
  -> Application result
  -> API Adapter 映射 response/error
```

#### Runtime 获取 Channel 配置

```text
Feishu/Other Channel Adapter
  -> 生成中立 ChannelBinding
  -> RuntimeProvisioning Use Case
  -> Runtime-specific renderer
  -> PicoClaw/OpenClaw config
```

Runtime 不再知道 `feishu.AgentCredentialProvider`。

#### Task

```text
Team planning ------+
Agent request ------+--> TaskUseCases -> taskcore -> task store/events
Schedule trigger ---+

team      = metadata + members + planning + projection
taskcore  = task state machine + approval + events + persistence
```

### 3.4 目标职责边界

| 层               | 应负责                                   | 不应负责                           |
| ---------------- | ---------------------------------------- | ---------------------------------- |
| API Adapter      | HTTP 解码、Use Case 调用、错误映射、响应 | 业务状态、文件持久化、Planner 构造 |
| CLI Adapter      | flags、daemon、PID、输出、打开浏览器     | 构造所有领域 Service               |
| Application      | 用例编排、事务边界、生命周期、恢复       | HTTP 或 Channel SDK 细节           |
| Domain           | 状态机、规则、实体、领域错误和 Port      | CLI、HTTP、文件路径                |
| Runtime Contract | 中立运行时输入和能力接口                 | Feishu 等具体 Channel 类型         |
| Infrastructure   | Store、SDK、进程、Runtime/Sandbox 实现   | 反向控制上层用例                   |
| API Contract     | 稳定 wire request/response               | Server 内部 Service command        |

## 4. 当前与目标的差距

| 当前状态                      | 目标状态                           | 主要收益                         |
| ----------------------------- | ---------------------------------- | -------------------------------- |
| Handler 保存业务状态          | Application Service 保存或协调状态 | HTTP 生命周期与业务生命周期解耦  |
| Handler 直接写注册文件        | Registration Store/Service         | 可复用、可测试、可替换           |
| `cli/serve` 装配整个应用      | `internal/app.Bootstrap`           | 启动逻辑可复用、可验证           |
| Team 与 Task Core 两套状态机  | Task Core 单一真相                 | 消除规则漂移和 API 合并          |
| Runtime 依赖 Feishu Provider  | Runtime 依赖 ChannelBinding        | 新 Channel 不修改 Runtime 构造器 |
| CLI 使用领域类型              | CLI 只使用 API Contract            | Client 与 Server 内部可独立演进  |
| localstore 了解所有 schema    | Versioned Migration Runner         | 迁移可审计、可演进               |
| Setter 注入可构造残缺 Handler | 显式 Dependencies 构造             | 启动时发现缺失依赖               |

## 5. 详细问题与代码归属

### 5.1 P1：HTTP Handler 同时是应用服务和状态容器

证据：

- `internal/api` 导入约 37 个本地 package。
- `handler.go` 约 3,274 行。
- Handler 持有 Agent、Participant、IM、Feishu、LLM、Hub、MCP、Team、Scheduled Task、Connector、Upgrade、Work Lease 等具体服务。
- 构造后仍需十多个 `Set*Service` / `Set*Manager`。

不应属于 HTTP Adapter 的状态：

```text
teamPlanJobs               # Team planning job 去重
participantActivityTurns   # Activity turn 生命周期
feishuRegistrationStateDir # Feishu 注册持久化目录
```

API 内还执行注册记录的目录创建、JSON 编解码、临时文件写入、Rename、扫描和删除。

目标位置：

```text
internal/app/
  agentusecase/
  participantusecase/
  teamusecase/
  registration/

internal/api/
  agents/
  participants/
  teams/
  router.go
```

API 子 Handler 只做四件事：

1. 解析输入。
2. 调用应用用例。
3. 映射领域错误。
4. 序列化响应。

### 5.2 P1：Team 与 Task Core 存在两套任务真相

当前重复：

```text
team.TeamTask / TeamApproval / TeamEvent / Presence
taskcore.Task / TaskApproval / TaskEvent / Presence

team.TaskStatusInProgress
taskcore.StatusInProgress
```

`team.Service` 和 `taskcore.Service` 都实现创建、Claim、完成、失败、阻塞、审批、事件和持久化。API 再把 `teamSvc` 与 `agentTaskSvc` 的结果合并。

风险：

- 状态迁移规则会逐渐不一致。
- 修复任务行为时难以判断权威位置。
- Scheduled Task 和 Team Task 走不同 Core。
- API 和前端被迫重新归一化。

目标：

- `taskcore` 成为唯一任务状态机。
- `assignment_type=team|agent` 表达分配目标。
- `team` 只保留 metadata、planning、成员和 Room projection。
- `agenttask`、`scheduledtask` 成为 Task Use Case 的 Adapter。

迁移前必须建立状态迁移等价测试和存量数据 Adapter，不能直接删除 `team.Service`。

### 5.3 P1：Runtime/Agent 反向依赖 Feishu

当前依赖：

```text
internal/runtime/sandboxgateway -> internal/channel/feishu
openclawsandbox/picoclawsandbox -> feishu.AgentCredentialProvider
internal/agent                 -> Feishu Provider
```

目标中立 contract：

```go
type ChannelBinding struct {
    Channel       string
    ParticipantID string
    Config        map[string]string
}

type ChannelBindingProvider interface {
    BindingsForAgent(agentID string) []ChannelBinding
}
```

生产实现应对 secret 使用只读 Value Object，避免任意 map 长期成为公开契约。

Feishu Adapter 在 `internal/app/channelwiring` 中生成 Binding；Runtime 只把 Binding 渲染为对应运行时配置。

### 5.4 P2：`cli/serve` 是过重的 Composition Root

`cli/serve/serve.go` 约 1,373 行，除命令行为外还负责：

- Local store migration。
- Bootstrap state。
- Agent/IM/Participant/Team/Task/Scheduled Task 构造。
- Channel bridge 与 Codex bridge。
- Model Provider preflight。
- Interrupted user-input 恢复。
- Runtime 注册、启动 Agent 和 Server 装配。

目标位置：

```text
cli/serve/
  flags.go
  daemon.go
  output.go
  command.go

internal/app/
  bootstrap.go
  dependencies.go
  lifecycle.go
  recovery.go
  channelwiring/
  runtimewiring/
```

CLI 只负责操作系统和命令行体验；应用启动顺序由 `internal/app` 拥有。

### 5.5 P2：Migration 知识位于通用 localstore

`internal/localstore/migrate_ids.go` 约 2,720 行，了解 Agent、Participant、IM、Team、Task、Provider、Auth、目录名和 mention tag 等大量上层 schema。

它使用 `map[string]any` 避免 import cycle，但 schema 变化无法由编译器发现，也缺少统一版本和执行记录。

目标位置：

```text
internal/migration/
  runner.go
  registry.go
  v1_typed_ids/
    plan.go
    agents.go
    participants.go
    im.go
    teams.go
    auth.go
    backup.go
```

`localstore` 只保留 Root Path、原子文件操作和 Backup Primitive。

### 5.6 P2：CLI HTTP Client 泄漏领域实现类型

普通 CLI 仍直接使用 Agent、Participant、Team、Task Core、Feishu 和 CLIProxy 的内部类型。

处理：

- Wire request/response 移入 `internal/apitypes` 或独立 `api/contracts`。
- CLI flags 在 CLI 内归一化。
- API Adapter 把 wire request 转换为领域 command。
- 必须本地调用 CLIProxy 的 operator command 作为明确例外，不与远程 API Client 混合。

### 5.7 P2：大 Service 的领域位置基本正确，但对象粒度过大

| 文件                                 |   约规模 | 当前职责                                         |
| ------------------------------------ | -------: | ------------------------------------------------ |
| `internal/im/service.go`             | 3,195 行 | User、Room、Message、Thread、mention、持久化     |
| `internal/agent/service.go`          | 2,800 行 | Manager、Worker、runtime、sandbox、logs、startup |
| `internal/agent/service_profiles.go` | 1,347 行 | Profile、MCP、模型、reconcile、upgrade           |
| `internal/channel/feishu/service.go` | 2,190 行 | User、Room、Message、Event、SDK                  |
| `internal/llm/service.go`            | 1,569 行 | Provider 路由、协议代理、stream、错误            |

这些代码多数位于正确领域，不应因为文件大就移入其他层。

建议先在原 package 内按能力拆分：

```text
im/
  users.go
  rooms.go
  messages.go
  threads.go
  presentation.go
  store.go

agent/
  lifecycle.go
  manager.go
  worker.go
  profiles.go
  resources.go
  logs.go
```

只有能力能够独立构造、依赖方向稳定后，才考虑提取子 package。

### 5.8 P2：API 允许“部分可用 Handler”

当前 Handler 先构造，再通过多个 Setter 补依赖；所有路由默认注册，缺少服务时运行期返回 503。

建议：

```go
type api.Dependencies struct {
    Agents       AgentUseCases
    Participants ParticipantUseCases
    Teams        TeamUseCases
}

func New(deps Dependencies) (*Handler, error)
```

- 必需依赖在构造时验证。
- 可选功能通过显式 Feature Set 决定是否注册路由。
- 不依赖每个 Handler 自己检查 nil。

### 5.9 P3：兼容逻辑缺少退出策略

Agent ID、Participant ID、旧 Runtime Kind、旧配置字段、旧消息格式和旧 Codex Protocol 的兼容代码分布在多个包。

需要统一记录：

- Compatibility Version。
- 命中统计。
- Migration 完成标记。
- 最后支持版本。
- 删除条件。

已经被 Migration 消除的数据格式，不应永久保留在热路径。

## 6. 详细实施计划

### 阶段 0：建立依赖护栏

目标：阻止架构继续向中心包泄漏。

- 记录 `internal/api`、`cli/serve`、Runtime 的当前本地依赖。
- 增加 package import 检查。
- 禁止新增 `runtime -> channel/feishu`。
- 新业务流程必须声明 Application Use Case owner。

完成标志：

- CI 能发现新增反向依赖。
- 新增 API 不再把业务状态放入 Handler。

### 阶段 1：移动确定错位的状态和持久化

- Feishu Registration Service/Store 移出 API。
- Team planning job 状态移入 Team Application Workflow。
- Participant activity turn 状态移入 Bridge/Application Service。
- Directory picker 建立 Platform Adapter。
- API 只保留 request-scoped 状态。

### 阶段 2：建立正式的 Application Bootstrap

- 在 `internal/app` 建立 `Bootstrap`、`Dependencies`、`Lifecycle` 和 `Recovery`。
- 移入 Service 构造顺序、Migration、Provider Preflight 和 Runtime 注册。
- API 使用显式 Dependencies。
- `cli/serve` 保留 flags、daemon、PID、输出和浏览器打开。

### 阶段 3：解除 Runtime 与 Channel 耦合

- 定义中立 `ChannelBinding` Port。
- 在 `channelwiring` 完成 Feishu 到 Binding 的转换。
- PicoClaw/OpenClaw Renderer 消费 Binding。
- 删除 Runtime 和 Agent 对 `internal/channel/feishu` 的 import。
- 增加第二种虚拟 Channel 的 Contract Test，验证抽象没有 Feishu 假设。

### 阶段 4：统一 Task Core

1. 建立 Team Service 与 Task Core 的状态迁移等价测试。
2. 建立 Team Task 到 Task Core 的兼容 Adapter。
3. 新 Team Task 写入 Task Core。
4. 迁移存量 Team Task Store。
5. Team Service 退化为 metadata、planning 和 projection。
6. API 删除两套任务的合并逻辑。

### 阶段 5：净化 Contract 和 Migration

- CLI 请求响应统一使用 `apitypes`。
- 建立版本化 Migration Runner。
- 迁移 Typed ID 等现有 Localstore Migration。
- 为兼容路径建立 Registry、命中统计和移除条件。

### 阶段 6：拆分领域包内部的大 Service

- 按 IM User/Room/Message/Thread 拆文件和能力接口。
- 按 Agent Lifecycle/Profile/Resource/Log 拆分。
- 按 LLM Routing/Streaming/Provider Adapter 拆分。
- 保持 package 不变，直到出现明确、稳定的新边界。

## 7. 架构验收标准

- `internal/api` 不保存领域状态，不直接读写 Registration 文件。
- API Handler 必需依赖在启动时完成验证。
- `cli/serve` 不再构造全部具体 Service。
- `internal/runtime/**` 和 `internal/agent` 不导入 `internal/channel/feishu`。
- Task 状态、审批、事件和持久化只有一套权威定义。
- `/api/v1/tasks` 不在 Adapter 层合并两套状态机。
- CLI HTTP 请求响应只依赖 API Contract。
- Migration 有版本、幂等测试和可审计结果。
- 现有 `go test ./...`、启动流程、Runtime 和 Web API 行为保持兼容。

## 8. 与历史架构图的关系

[历史架构图](architecture.md) 对以下概念关系仍然有效：

```text
channel -> room -> participant -> agent -> runtime -> sandbox
```

但它没有表达：

- Team、Task 和 Scheduled Task。
- Hub、Skill、MCP 和 Connector。
- Model Provider、CLIProxy 和 LLM Bridge。
- Upgrade、Config、Migration 和 Work Lease。
- Codex Runtime 与 Sandbox Runtime 的差异。
- Application Layer 和四条并行业务链路。

建议保留历史文档作为核心概念关系图，本文作为当前组件架构和目标架构。第一阶段应用层重构稳定后，再升级正式主架构图。

## 9. 不建议做的事情

- 不拆微服务。
- 不一次性重写 `internal/api` 或 `internal/agent`。
- 不为了减少行数创建大量只有少数函数的 package。
- 不在迁移存量数据前直接删除 Team Task 旧实现。
- 不用全局 Service Locator 替代显式 Dependencies。
- 不把所有领域对象都改成接口；只在跨层和可替换基础设施边界定义 Port。

最终判断：CSGClaw 后端适合继续做模块化单体。下一阶段的关键不是目录重排，而是把应用编排从 API/CLI 抽回 `internal/app`，再沿着明确的依赖方向逐步统一 Task、Channel Binding、Contract 和 Migration。
