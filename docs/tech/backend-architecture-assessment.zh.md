# CSGClaw 后端架构评估

> - 评估日期：2026-07-24
> - 代码基线：`f6559fed`（`upstream/main`）
> - 范围：Go CLI、HTTP Server、API、领域服务、Runtime、Sandbox、持久化与装配链路
> - 结论：继续演进模块化单体，不拆微服务

关联文档：

- [Agent / Channel / Runtime 架构改造方案](agent-channel-runtime-refactor-plan.zh.md)：
  细化本文 P1 Channel Binding 问题，并定义执行模式、接口、协议和迁移步骤。
- [历史架构图](architecture.md)

本文给出后端整体职责图和演进顺序；Agent、Channel、Runtime 纵向切面的实现细节以
对应改造方案评审后的结论为准。本文基线早于该方案，开始实施前仍需用当前代码复核
依赖数量、文件行数等易变化数据。

## 1. 结论

CSGClaw 后端已经具备主要业务能力，当前问题不是功能不足，而是部分实现放错了位置：

- `internal/api.Handler` 同时处理 HTTP、业务编排、业务状态和文件持久化。
- `cli/serve` 同时处理命令行和整个应用的构造、恢复与启动。
- `internal/team` 和 `internal/taskcore` 各自维护一套任务状态机。
- `internal/runtime` / `internal/agent` 直接依赖 Feishu 类型。
- CLI Client 使用 Server 内部领域类型。
- `internal/localstore` 知道过多上层迁移规则。

下一步应保持单进程，渐进完成以下调整：

1. 跨领域流程放入 `internal/app`。
2. `internal/api` 和 `cli/serve` 只保留入口职责。
3. `taskcore` 成为任务状态、审批、事件和持久化的唯一真相。
4. Runtime 通过中立的 `ParticipantBinding` 或 Runtime 专用配置输入获取 Channel 配置。
5. CLI 和 Server 只通过 `apitypes` 传输数据。
6. 建立版本化、幂等、可审计的 Migration Runner。

## 2. 各层职责

这些名称表示职责，不要求为每层新建一个顶级目录。

```text
Adapter         怎么进来
Application     要完成什么
Domain          什么是合法的
Infrastructure  具体怎么执行
API Contract    网络上怎么表示
```

| 概念 | 一句话说明 | CSGClaw 中的职责 |
| --- | --- | --- |
| Adapter | 对接外部入口 | 解析 HTTP、CLI、Channel 输入，调用用例并转换输出 |
| Application | 执行一个完整用例 | 组织业务流程，协调多个领域能力和基础设施 |
| Domain | 维护业务事实和规则 | 定义实体、状态迁移、业务规则和领域错误 |
| Infrastructure | 实现外部能力 | 操作文件、SDK、Runtime、Sandbox、LLM 和进程 |
| API Contract | 定义通信协议 | 定义跨进程请求和响应，不承载业务规则 |

API Contract 不是独立的业务层，而是 Adapter 边界使用的数据协议。

对应当前目录：

```text
HTTP JSON、路由、状态码             -> internal/api
CLI 参数、daemon、PID、输出         -> cli/serve
跨多个领域完成一次操作              -> internal/app
任务状态转换和业务规则              -> internal/taskcore
Team metadata、成员和 planning      -> internal/team
Channel SDK                         -> internal/channel/*
Runtime / Sandbox                   -> internal/runtime、internal/sandbox
文件操作                            -> internal/localstore
版本化迁移                          -> internal/migration（目标）
跨进程请求和响应                    -> internal/apitypes
HTTP Client                         -> internal/apiclient
```

以“创建任务”为例：

```text
HTTP Handler                         # Adapter：读取 JSON
  -> CreateTask Use Case             # Application：检查上下文并组织流程
     -> taskcore                     # Domain：创建任务并检查状态规则
     -> Task Store                   # Infrastructure：保存任务和事件
  -> TaskResponse                    # API Contract：转换成稳定 JSON
```

简单判断：

1. 处理 HTTP、CLI 或 Channel 格式：Adapter。
2. 组织一次完整用户操作：Application。
3. 不依赖 HTTP 和存储也成立的规则：Domain。
4. 读写文件、启动进程或调用 SDK：Infrastructure。
5. 定义网络 JSON：API Contract。

## 3. 当前实现

`(!)` 表示当前主要问题。

```text
+----------------------------- External ------------------------------+
| Web UI | csgclaw CLI | csgclaw-cli | Feishu / CSGClaw Channel      |
+----+-----------+-------------+------------------+--------------------+
     |           |             |                  |
     v           v             v                  v
+----------------------------- Entrypoints ----------------------------+
| internal/server                                                    |
|      |                                                              |
|      v                                                              |
| (!) internal/api.Handler                                            |
|     HTTP + 业务编排 + 业务状态 + 部分文件持久化                     |
|                                                                     |
| (!) cli/serve                                                       |
|     flags + daemon + 应用装配 + migration + recovery + lifecycle    |
+-------------------------------+--------------------------------------+
                                |
              +-----------------+------------------+
              |                                    |
              v                                    v
+-------------------------------+  +-----------------------------------+
| Collaboration / Task          |  | Agent / Runtime                   |
| participant / im / team       |  | agent -> runtime -> sandbox       |
| (!) team task + taskcore task |  | (!) runtime/agent -> Feishu      |
+-------------------------------+  +-----------------------------------+
              |                                    |
              +------------------+-----------------+
                                 v
+-------------------------- Infrastructure ----------------------------+
| local stores / config / SDK / filesystem / processes                |
+----------------------------------------------------------------------+
```

当前主要数据流：

```text
控制面：Web / CLI -> HTTP API -> Handler -> 各领域 Service
协作面：Channel -> Bridge -> Room / Participant / IM -> Runtime
任务面：Team Task -> team；Agent / Schedule Task -> taskcore
模型面：Runtime -> LLM Bridge -> Provider / CLIProxy / CSGHub-lite
```

本次基线上，`internal/api` 直接导入约 37 个本地 package，`cli/serve` 直接导入约 36 个；`handler.go` 约 3,274 行，`serve.go` 约 1,373 行。测试基础较好，因此适合逐步迁移，不需要重写。

当前已有可继续利用的基础：

- `cmd/csgclaw` 与 `cmd/csgclaw-cli` 入口较薄。
- 各主要领域已经有可识别的 package。
- Runtime 和 Sandbox 已有接口。
- `apitypes`、`apiclient` 已开始分离网络协议。
- `app/channelwiring`、`app/runtimewiring` 已经存在。

## 4. 主要问题和实现方式

### 4.1 P1：API Handler 保存业务状态

当前 Handler 持有大量具体服务，构造后还需要多个 Setter。以下长期状态也在 API 层：

```text
teamPlanJobs               Team planning job 去重
participantActivityTurns   Activity turn 生命周期
feishuRegistrationStateDir Feishu 注册记录目录
```

API 中还直接执行注册文件的创建、JSON 编解码、临时文件写入、Rename、扫描和删除。

这会导致：

- HTTP 生命周期和业务状态生命周期绑定。
- 同一业务难以被 CLI、Channel 或后台任务复用。
- Handler 依赖越来越多。
- 缺失依赖只能在运行期通过 503 暴露。

目标实现：

```text
internal/api
  -> 解析请求
  -> 调用 Use Case
  -> 映射错误和响应

internal/app
  -> 保存工作流状态
  -> 协调领域 Service
  -> 调用 Store / Runtime

Infrastructure
  -> 读写注册文件和其他持久化数据
```

API 使用显式依赖构造；必需依赖启动时检查，可选能力通过 Feature Set 决定是否注册路由。

### 4.2 P1：存在两套 Task

当前重复定义：

```text
team.TeamTask       <-> taskcore.Task
team.TeamApproval   <-> taskcore.TaskApproval
team.TeamEvent      <-> taskcore.TaskEvent
team.Presence       <-> taskcore.Presence
```

两个 Service 都实现创建、Claim、完成、失败、阻塞、审批、事件和持久化。结果是状态规则可能漂移，API 及其调用方还要合并、归一化两套数据。

目标逻辑：

```text
Team planning ------+
Agent request ------+--> TaskUseCases -> taskcore -> task store / events
Schedule trigger ---+

team      = metadata + members + planning + room projection
taskcore  = task state machine + approval + events + persistence
```

实现顺序：

1. 建立两套状态迁移的等价测试。
2. 建立 Team Task 到 `taskcore` 的兼容 Adapter。
3. 新 Team Task 只写入 `taskcore`。
4. 迁移存量 Team Task。
5. 缩减 `team.Service`。
6. 删除 API 的两套任务合并逻辑。

存量数据迁移完成前不能直接删除旧实现。

### 4.3 P1：Runtime 和 Agent 依赖 Feishu

当前依赖：

```text
runtime/sandboxgateway -> channel/feishu
openclawsandbox / picoclawsandbox -> feishu.AgentCredentialProvider
agent -> Feishu Provider
```

增加新 Channel 时需要修改 Runtime 和 Agent。

目标是引入中立的派生输入，具体字段与生命周期由
[Agent / Channel / Runtime 架构改造方案](agent-channel-runtime-refactor-plan.zh.md)
统一定义：

```go
type ParticipantBinding struct {
    ParticipantID string
    Channel       string
    AgentID       string
    ChannelUser   ChannelUserRef
    Metadata      map[string]string
}
```

目标调用：

```text
Feishu / Other Channel Adapter
  -> app/channelwiring 从 Participant 派生 ParticipantBinding
  -> RuntimeProvisioning Use Case
  -> 短生命周期 Secret Resolver
  -> Runtime Renderer
  -> PicoClaw / OpenClaw 配置
```

Participant Store 仍是 Binding 的唯一事实源，不新建包含 Participant、Agent、Runtime
和 Credential 副本的 Binding Store。Runtime 只消费中立 Binding 或 Runtime 专用配置
输入，不再知道 Feishu 类型。Secret 使用只读 Value Object/Handle 按需解析，不进入
`Metadata`、日志或跨进程 Event Envelope。

### 4.4 其他问题

| 问题 | 当前实现 | 应该怎么做 |
| --- | --- | --- |
| `cli/serve` 过重 | 构造 Service、执行迁移和恢复、注册 Runtime、启动 Agent 和 Server | CLI 只处理 flags、daemon、PID、输出；其余移入 `app.Bootstrap` |
| Migration 位置错误 | `localstore/migrate_ids.go` 知道大量上层 Schema | 新建版本化 Migration Runner；`localstore` 只保留文件原语 |
| CLI 泄漏内部类型 | CLI 直接使用 Agent、Team、Task、Feishu 等领域类型 | 请求响应统一使用 `apitypes`，API 再转成 Application Command |
| Service 文件过大 | IM、Agent、Feishu、LLM Service 承担多类能力 | 先在原 package 内按能力拆文件，不急于增加 package |
| 兼容逻辑无退出条件 | 旧 ID、配置和协议兼容散落在热路径 | 记录版本、命中统计、迁移标记、最后支持版本和删除条件 |

大 Service 建议先按以下方式拆文件：

```text
im/                         agent/                     llm/
|-- users.go                |-- lifecycle.go           |-- routing.go
|-- rooms.go                |-- manager.go             |-- streaming.go
|-- messages.go             |-- worker.go              |-- providers.go
|-- threads.go              |-- profiles.go            `-- errors.go
`-- store.go                |-- resources.go
                            `-- logs.go
```

## 5. 目标目录结构

这是一份目标职责图：包含本次调整涉及的目录，也保留其他主要后端目录。不要求一次性创建所有目标文件，也不要求把现有包全部搬入新的 `domain/` 或 `infrastructure/` 顶级目录。

```text
csgclaw/
|
|-- cmd/
|   |-- csgclaw/                    # 薄入口
|   `-- csgclaw-cli/                # 薄入口
|
|-- cli/
|   |-- serve/
|   |   |-- command.go              # 调用 app.Bootstrap
|   |   |-- flags.go                # 参数
|   |   |-- daemon.go               # daemon / PID
|   |   `-- output.go               # 输出、打开浏览器
|   `-- ...                         # 其他 CLI Adapter
|
`-- internal/
    |
    |-- server/                      # HTTP Server 和静态资源
    |
    |-- api/                         # HTTP Adapter
    |   |-- router.go
    |   |-- errors.go                # 业务错误 -> HTTP
    |   |-- agents/
    |   |-- participants/
    |   |-- teams/
    |   `-- tasks/
    |
    |-- apitypes/                    # Wire request / response
    |-- apiclient/                   # CLI 使用的 HTTP Client
    |
    |-- app/                         # 用例和应用装配
    |   |-- bootstrap.go
    |   |-- dependencies.go
    |   |-- lifecycle.go
    |   |-- recovery.go
    |   |-- agent_usecases.go
    |   |-- participant_usecases.go
    |   |-- team_usecases.go
    |   |-- task_usecases.go
    |   |-- registration/            # Registration Use Case 和 Store Port
    |   |-- channelwiring/           # Channel -> Binding
    |   `-- runtimewiring/           # Runtime 装配
    |
    |-- participant/                 # Participant 规则
    |   `-- feishubind/              # Participant 与 Feishu 绑定
    |-- im/                          # User / Room / Message / Thread
    |-- team/                        # metadata / members / planning
    |-- taskcore/                    # 唯一 Task 状态机
    |-- agenttask/                   # Agent Task Use Case Adapter
    |-- scheduledtask/               # Schedule Task Use Case Adapter
    |-- agent/                       # Agent lifecycle 和配置
    |-- agentmanager/                # Manager 相关能力
    |-- agentworkspace/              # Agent 工作目录
    |-- activity/
    |-- worklease/
    |-- template/
    |-- templates/                   # 内置模板资源
    |-- skill/
    |-- mcp/
    |-- slashcommand/
    |
    |-- channel/                     # Channel Adapter / SDK
    |   |-- feishu/
    |   `-- csgclaw/
    |-- channelbridge/
    |
    |-- runtime/                     # Runtime contract 和实现
    |   |-- codex/
    |   |-- picoclawsandbox/
    |   |-- openclawsandbox/
    |   `-- sandboxgateway/
    |-- runtimeassets/               # Runtime 内置资源
    |-- runtimecatalog/              # Runtime 类型目录
    |
    |-- sandbox/                     # Sandbox contract 和实现
    |   |-- boxlitecli/
    |   |-- dockercli/
    |   `-- csghub/
    |-- sandboxproviders/            # Sandbox Provider 装配
    |
    |-- llm/
    |-- modelprovider/
    |-- modelcap/
    |-- codexmodel/
    |-- codexcli/
    |-- cliproxy/
    |-- connectors/
    |
    |-- migration/                   # 版本化迁移
    |   |-- runner.go
    |   |-- registry.go
    |   `-- v1_typed_ids/
    |
    |-- registrationstore/           # Registration 文件 Store 实现
    |-- localstore/                  # 路径、原子写、备份原语
    |-- config/
    |-- auth/
    |-- identity/
    |-- onboard/
    |-- upgrade/
    |-- desktop/
    |-- assets/
    |-- utils/
    `-- version/
```

目标请求链路：

```text
HTTP / CLI / Channel
  -> Adapter
  -> Application Use Case
  -> Domain Rule
  -> Store / Runtime / SDK
  -> Application Result
  -> API Contract / Adapter Response
```

目标启动链路：

```text
cli/serve
  -> 解析 flags
  -> app.Bootstrap(config)
       -> 验证 Dependencies
       -> migration / recovery
       -> Channel / Runtime wiring
       -> 启动 Server 和后台任务
  -> App.Shutdown()
```

## 6. 实施顺序

### 阶段 0：依赖护栏

- CI 禁止新增 `runtime -> channel/feishu`。
- 新业务流程必须明确 Application Use Case owner。

### 阶段 1：瘦身 API

- 移出 Registration 文件操作。
- 移出 Team planning job 和 Participant activity turn 状态。
- API 使用显式 Dependencies。

### 阶段 2：建立 Application Bootstrap

- 将 Service 构造、Migration、Recovery、Runtime 注册和生命周期移入 `internal/app`。
- `cli/serve` 只保留 CLI 和操作系统行为。

### 阶段 3：Channel Binding

- 定义从 Participant 派生的中立 `ParticipantBinding`，不新增持久化 Binding Store。
- 在 `channelwiring` 选择执行模式，并将 Binding 与短生命周期 Secret 转换为 Runtime
  专用配置输入。
- 删除 Runtime 和 Agent 对 Feishu 的 import。
- 用第二种虚拟 Channel 做 Contract Test。

### 阶段 4：统一 Task Core

- 先做等价测试和兼容 Adapter。
- 再迁移新旧 Team Task。
- 最后删除 API 合并逻辑。

### 阶段 5：Contract 和 Migration

- CLI 请求响应统一使用 `apitypes`。
- 建立版本化 Migration Runner。
- 为兼容路径增加命中统计和删除条件。

### 阶段 6：拆分大 Service

- 在原 package 内按能力拆文件。
- 只有新边界稳定后才提取 package。

## 7. 验收标准

- API 不保存领域状态，不直接读写 Registration 文件。
- API 必需依赖在启动时验证。
- `cli/serve` 不再构造全部 Service。
- Runtime 和 Agent 不导入 `channel/feishu`。
- Task 状态、审批、事件和持久化只有一套定义。
- CLI HTTP 请求响应只依赖 `apitypes`。
- Migration 有版本、幂等测试和审计结果。
- `go test ./...`、启动流程、Runtime 和 Web API 行为保持兼容。

## 8. 不做什么

- 不拆微服务。
- 不一次性重写中心模块。
- 不为了减少行数创建大量小 package。
- 不在迁移数据前删除旧 Team Task。
- 不使用全局 Service Locator。
- 不把所有领域对象都改成接口。

最终目标：

```text
入口只处理输入输出
应用层组织完整操作
领域层维护业务规则
基础设施负责真实执行
API Contract 保持跨进程协议稳定
```
