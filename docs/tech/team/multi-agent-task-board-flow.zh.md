# CSGClaw 多 Agent 协作与任务看板流程（会议讨论稿）

> 本文用于讨论 CSGClaw 多 Agent 协作和任务看板的核心产品流程。重点不是提前固定答案，而是把当前问题、候选模型、参考案例和需要会议拍板的决策放在同一份材料中。

---

## 1. 当前产品中的主要概念

为方便后续讨论，先列出当前产品中已经使用的一些主要概念：

```text
主 Task  = 用户希望持续追踪和最终验收的目标
子 Task  = 为完成主 Task 拆出的执行工作项
Run      = Agent 对某个 Task 的一次执行尝试
Team     = 可复用的成员与协作规则，本身不执行任务
Leader   = Team 的规划、协调、汇总负责人
Manager  = 系统默认给用户创建的主管 Agent / 主 Agent，当前定位是全知全能的系统级 Agent；创建 Team 时会被默认设置为 Leader Agent
```

---

## 2. 当前流程

当前已经存在三条容易交叉的工作路径：

```text
Manager 直接执行
单 Agent Task
Team 主 Task + Planner + 子 Task
```

### 当前主流程图

```text
┌───────────────────────────────────────────────────────────────┐
│                         用户提出目标                           │
└──────────────────────────────┬────────────────────────────────┘
                               │
                 ┌─────────────┴─────────────┐
                 │                           │
                 ▼                           ▼
          在 Web 创建 Task              告诉 Manager
                 │                           │
          选择 Agent / Team       Manager 自行判断如何处理
                 │                           │
          ┌──────┴──────┐       ┌───────────┼────────────┐
          │             │       │           │            │
          ▼             ▼       ▼           ▼            ▼
     单 Agent Task   Team Task  自己完成   创建 Agent   创建 Team
          │             │                   │            │
          │             │                   └─────┬──────┘
          │             │                         │
          │             │              ┌──────────┴──────────┐
          │             │              │                     │
          │             │              ▼                     ▼
          │             │       在房间直接发消息         创建 Team Task
          │             │       可能没有正式 Task              │
          │             │                                       │
          │             └──────────────────┬────────────────────┘
          │                                │
          │                         创建执行房间
          │                                │
          │                         Planner 拆分任务
          │                         并选择 assignee
          │                                │
          └────────────────┬───────────────┘
                           ▼
                    Worker 收到通知
                           │
                           ▼
                    claim 并执行 Task
                           │
              ┌────────────┼────────────┐
              │            │            │
              ▼            ▼            ▼
          completed      blocked      failed
              │
              ▼
       所有子 Task completed？
              │
         ┌────┴────┐
         │         │
        否         是
         │         │
         │         ▼
         │   主 Task 自动 completed
         │   系统聚合子任务结果
         │         │
         └─────────┴───────────┐
                               ▼
                         用户看到结果
```

---

## 3. 待讨论问题：会议需要对齐的 9 个决策

建议先讨论 1～2，先确定 Task 与用户之间的交互形式，再讨论角色和治理规则。

### A. Task 与用户之间的交互形式（P0）

1. **面向 Team 的 Task 应该如何新建？**
   - 当前方式：先创建 Team，再进入任务页面创建 Task，最后把 Task 指派给 Team；流程较长，用户也不容易知道正确顺序；
   - 候选方式一：取消 Team，由 Room 直接承担成员组织和协作；在当前 Room 中创建 Task。由于一个 Room 会承接多个 Task，需要用 Thread / 任务讨论串隔离每个 Task 的上下文，否则消息会混在一起；
   - 候选方式二：直接在 Task / Issue 中 `@Team` 启动协作，聊天和执行记录都保留在 Task 中，不再创建对应 Room。

2. **用户如何向已有 Task 追加要求？**
   1. 当前方式：只能进入 Task 对应的 Room，在 Room 中继续追加用户要求；
   2. 候选方式：直接在 Task 详情中追加用户要求，由系统绑定当前 Task 并投递给对应执行者。

### B. 谁来组织协作（P1）

3. **Manager 是什么？**
   - 当前定位：Manager 是系统入口和内置执行 Agent，也是所有 Team 的默认 Leader / 上层协调者，需要承担两级调度：
     1. 跨 Team：判断任务应该分配给哪个 Team，并协调各 Team Leader；
     2. Team 内：任务进入 Team 后，继续安排具体成员和执行方式。
   - 候选定位：Manager 是默认 Worker + 产品功能指引助手，可以执行普通任务并指导用户使用产品，但不再默认承担 Team 内或跨 Team 的协调。
   - 如果保留当前定位，还需要分别明确“选择 Team”和“Team 内指派成员”的规则。

4. **Team 是否保留？**
   - 保留 Team，或由 Room 替代；
   - 若同时保留：Team 管成员和规则，Room 管沟通。

5. **Leader 固定由 Manager 担任，还是允许用户指定其他 Agent？**
   - 方案一：所有 Team 固定由 Manager 担任 Leader，用户无需选择。Manager 按“全知全能”的系统级 Agent 定位负责规划、协调和验收，产品流程最简单；
   - 方案二：用户可以指定其他 Agent 担任 Leader，以满足更细粒度的需求。例如选择具备多模态能力、能够验收图片生成结果的 Agent，或选择速度更快、价格更低的模型，让它以低成本 PMO 的角色持续跟进和协调 Worker。

6. **Planner、Leader 与 Reviewer 的关系是什么，规划和验收规则是否允许用户配置？**
   - 角色关系：三者可以由同一个 Agent 承担，也可以由 Planner 生成方案、Leader 负责执行协调、Reviewer 独立验收；
   - 方案一：不提供单独配置，系统使用内置的规划和验收规则，用户只描述任务目标；
   - 方案二：允许用户提供规则，包括单个 Task 的验收标准，以及 Workspace / Team 级的长期规划标准。例如“面向 500 人规模团队，所有系统方案默认满足 500 人并发”；
   - 配置方式可以是完全自定义 Prompt，也可以是结构化模板；如果第一阶段暂不提供，也应为后续扩展保留入口。

### C. 自然语言执行、Team 指派与完成规则（P1）

7. **用户通过自然语言让 Manager 工作时，什么时候创建 Task，什么时候直接执行？**
   - 方案一：只有用户明确说“创建任务”“放到看板”或明确派单时，Manager 才创建 Task；其他请求直接完成；
   - 方案二：由 Manager 自动判断。可以在当前交互中完成的请求直接执行；需要持续跟踪、异步执行或多 Agent 协作时，先创建 Task 再执行；
   - 如果选择自动判断，需要明确触发规则，并让用户知道本次工作是否进入任务看板。

8. **Team 接到 Task 后，Leader 如何把工作指派给成员？**
   - 用户已经指定成员时，以用户选择为准；
   - 用户未指定时，需要明确 Leader 是否根据 Agent 的名称、Profile 描述和实际能力进行匹配。

9. **什么时候 Task 才算完成，是否允许用户设置完成规则？**
   - Agent 执行结束是直接完成 Task，还是只表示已提交结果、需要验收通过后才完成；
   - 完成规则可以使用系统默认标准，也可以复用第 6 点的配置，让用户提供 Task 级验收标准。

---

## 4. 当前最需要解决的问题

当前最重要的是：**用户能否在不理解 Task、Room、Agent 内部关系的情况下，继续推进一个已有 Task。**

### 4.1 P0：已有 Task 的二次调整路径过长，而且入口不明显

```text
用户在看板看到 Task
  → 找对应 Room
  → 找正确的 Leader / Worker
  → @ 下达补充要求
```

- 路径长：同一个任务要跨 Task、Room、Agent 三个对象；
- 不易发现：用户不知道要从看板进入 Room 再操作。

Multica 直接在 Issue 中追加指令；Raft 以 Channel 为中心，没有 Task → Room 的跳转。CSGClaw 同时保留看板、Team 和 Room，就必须主动解决这个断点。

### 4.2 P0：追加要求与原 Task 的对象关系不清晰

```text
继续优化 → 新 Run / 重开 Task / 子 Task / 新 Task / 只发消息？
```

一律建子 Task 会让看板膨胀；只发消息又无法追踪。需要先确定默认语义，再设计入口。

### 4.3 P0：Manager 私聊中的跨任务定位存在串任务风险

Manager 即使能看到所有 Room，也无法保证“把之前那个任务再优化一下”指向正确 Task。要么让 Manager 绑定并确认目标 Task，要么明确要求用户回到 Task；不能让 Manager 静默猜测。

### 4.4 P0：Room 数量会持续增长，但当前缺少组织和归档层级

如果每个 Team Task 都创建 Room，Room 会快速增多。至少需要区分私聊、Team Room、Task Room，并展示所属 Task / Team、活跃状态、双向入口和归档状态。

### 4.5 P1：角色与任务生命周期问题会放大上面的断点

- Manager 身份过多，Manager 与 Team Leader 语义混用；
- Planner 指派规则不稳定；
- Room 消息可能绕过任务系统；
- Worker 执行结束被当作任务已验收。

---

## 5. 一种可能的目标角色模型（待讨论）

这个候选模型成立的前提是：保留 Team、Team 有唯一 Leader、Task 是状态事实源。前提变化时，角色也要调整。

### 5.1 CSGClaw 候选角色

| 角色 | 候选职责 |
| --- | --- |
| Manager | 系统入口和平台操作；判断是否进入任务流程；不自动成为所有 Team 的 Leader |
| Team Leader | 计划、拆分、指派、协调、子任务 Review 和主任务汇总 |
| Worker | 执行、报告进度 / 阻塞、提交交付物、按反馈修正 |
| Reviewer / Acceptor | Reviewer 验收子任务；Acceptor 最终验收主任务 |

### 5.2 Multica 的参考模型

Multica 只作为参考，不代表 CSGClaw 必须照搬。注意：**Multica 的 Issue 更接近本文的 Task，Multica 底层的 Task 更接近本文的 Run。**

| CSGClaw 候选概念 | Multica 中的近似对象 | 关键差异 |
| --- | --- | --- |
| Task | Issue | Issue 同时承载状态、评论、活动时间线和多次 Agent 执行 |
| Run | `agent_task_queue` 中的一次 Task | 每次指派、评论触发或 `@Agent` 都可能产生新的执行 Run |
| Team | Squad | Squad 是路由与协调对象，不是可直接运行的 Agent |
| Team Leader | Squad 的 `leader_id` Agent | 指派给 Squad 时先路由给 Leader，不会自动 fan-out 全体成员 |
| Worker | Squad 中的 Agent 成员 | Leader 可通过子 Issue 或评论中的 `@Agent` 委派 |
| 执行 Room | 没有完全等价对象 | Issue 评论和时间线承担了与任务绑定的协作入口 |
| Manager | 没有完全等价对象 | Multica 的独立 Chat 与 Issue 上下文分开，不负责模糊地跨 Issue 续接任务 |
| Reviewer / Acceptor | `in_review` / `done` 流程中的人或 Agent | 没有完全相同的固定 Acceptor 对象 |

一个可能的 Multica 协作例子：

```text
用户创建 Issue：“优化登录体验”
  → 指派给 Frontend Squad
  → 系统把执行路由给 Squad Leader
  → Leader 将父 Issue 设为 in_progress
  → Leader 创建子 Issue，分别指派给 UI Agent、Test Agent
  → Worker 在各自子 Issue 中执行和回报
  → 用户直接在父 Issue 评论：“移动端间距还要再调整”
      ├─ 评论唤醒当前 Squad Leader
      └─ 或显式 @某个 Agent / @Squad 触发对应路由
  → 系统在同一个 Issue 下创建新的底层执行 Task（相当于 Run）
  → 同一 (Agent, Issue) 可恢复之前的 session_id 和工作目录
  → 子 Issue 的阶段完成后，系统提醒并再次唤醒父 Issue 的 Leader
  → Leader 汇总并把父 Issue 推进到 in_review
  → 人类确认后进入 done；若需修改，继续在该 Issue 下追加指令
```

可借鉴的重点：

1. **工作项本身就是后续指令入口**，用户不需要先找到另一个聊天空间；
2. **同一目标的再次执行建 Run，不必自动建子 Task**；只有可独立跟踪和验收的工作才拆子项；
3. **执行者可以变化，但所有评论、Run、状态和结果仍挂在同一个工作项下。**

但 Multica 的 Squad 默认只先唤醒 Leader，不等同于多人 Room；它也没有跨 Issue 模糊续接的 Manager。参考重点是“工作项作为协作主上下文”，不是复制角色名称。

---

## 6. 候选流程与 Multica 参考流程

### 6.1 CSGClaw 候选目标流程（原预期图保留）

下面保留原来的预期流程图，作为会议讨论的候选方案，不代表已经确定：

```text
┌────────────────────────────────────────────────────────────────────┐
│                            用户提出目标                             │
└───────────────────────────────┬────────────────────────────────────┘
                                │
                                ▼
                   ┌────────────────────────┐
                   │ 是否需要进入任务流程？   │
                   └────────────┬───────────┘
                                │
                 ┌──────────────┴──────────────┐
                 │                             │
                否                            是
                 │                             │
                 ▼                             ▼
       当前 Agent 直接完成            创建一个主 Task
       不建 Agent / Team / Task        目标 + 范围 + 验收标准
                 │                     creator + acceptor
                 ▼                             │
            直接回复用户                       ▼
                                    明确 assignee
                                             │
                         ┌───────────────────┴───────────────────┐
                         │                                       │
                         ▼                                       ▼
                  指派给单个 Agent                         指派给 Team
                         │                                       │
                         │                           校验是否有唯一 Leader
                         │                                       │
                         │                              ┌────────┴────────┐
                         │                              │                 │
                         │                             无                有
                         │                              │                 │
                         │                              ▼                 ▼
                         │                         阻止启动        Leader 生成计划
                         │                                         │
                         │                              子任务 + 依赖 + assignee
                         │                              Reviewer + 交付物
                         │                                         │
                         │                                  服务端策略校验
                         │                                         │
                         └────────────────────┬────────────────────┘
                                              ▼
                                      系统正式 dispatch
                                      房间只投影通知
                                              │
                                              ▼
                                      Agent claim → Run
                                              │
                              ┌───────────────┼───────────────┐
                              │               │               │
                              ▼               ▼               ▼
                           正常执行         blocked          failed
                              │               │               │
                              └───────────────┴──────┬────────┘
                                                     │
                                                     ▼
                                      Agent 提交交付和验证证据
                                      Task → in_review
                                                     │
                                                     ▼
                                           Reviewer 验收
                                                     │
                                        ┌────────────┴────────────┐
                                        │                         │
                                      驳回                       通过
                                        │                         │
                                        ▼                         ▼
                              返回 in_progress             子 Task → done
                                        │                         │
                                        └────────────┬────────────┘
                                                     │
                                      所有必需子 Task 都 done？
                                                     │
                                           ┌─────────┴─────────┐
                                           │                   │
                                          否                  是
                                           │                   │
                                           │                   ▼
                                           │          Leader 汇总主 Task
                                           │          主 Task → in_review
                                           │                   │
                                           │                   ▼
                                           │          Acceptor 最终验收
                                           │                   │
                                           │          ┌────────┴────────┐
                                           │          │                 │
                                           └──────────┤ 驳回            │ 通过
                                                      │                 │
                                                      ▼                 ▼
                                                重新规划或返工      主 Task → done
                                                                        │
                                                                        ▼
                                                                 通知用户并归档
```

### 6.2 Multica 当前参考流程

这张图描述的是当前 Multica 的关键产品路径，用来对照“为什么用户可以直接在任务看板 / Issue 上继续下达指令”：

```text
┌──────────────────────────────────────────────────────────────┐
│                用户创建或打开一个已有 Issue                  │
│        Issue = 状态 + assignee + 评论 + 时间线 + 执行记录     │
└─────────────────────────────┬────────────────────────────────┘
                              │
                              ▼
                       选择 assignee
                              │
                 ┌────────────┴────────────┐
                 │                         │
                 ▼                         ▼
            指派给 Agent              指派给 Squad
                 │                         │
                 │                  路由给 Squad Leader
                 │                         │
                 └────────────┬────────────┘
                              ▼
                   创建底层 Agent Task / Run
                              │
                              ▼
                    Agent / Leader 执行工作
                              │
                 ┌────────────┴────────────┐
                 │                         │
                 ▼                         ▼
          单 Agent 直接交付          Leader 拆分子 Issue
                                           │
                                 指派给不同 Worker Agent
                                           │
                                           ▼
                                    Worker 并行 / 分阶段执行
                 │                         │
                 └────────────┬────────────┘
                              ▼
                结果、评论和状态都回到 Issue
                              │
              ┌───────────────┴────────────────┐
              │                                │
              ▼                                ▼
       用户接受 → done              用户在同一 Issue 追加指令
                                               │
                                  ┌────────────┴────────────┐
                                  │                         │
                                  ▼                         ▼
                         唤醒当前 assignee          显式 @Agent / @Squad
                                  │                         │
                                  └────────────┬────────────┘
                                               ▼
                                      创建新的底层 Run
                                      不自动创建新 Issue
                                               │
                                               ▼
                                恢复同一 Agent + Issue 上下文
                                               │
                                               └────→ 回到交付 / Review
```

### 6.3 两条流程的关键差异

| 维度 | CSGClaw 当前 / 原候选流程 | Multica 参考流程 | 对 CSGClaw 的启发 |
| --- | --- | --- | --- |
| 后续指令入口 | Task 再跳到执行 Room | 直接在 Issue 追加评论 / 指令 | Task 详情应提供默认的“继续处理”入口 |
| 一次返工的对象 | 可能是 Room 消息、子 Task 或新 Task | 同一 Issue 下的新底层 Run | 明确 Task 与 Run，避免子 Task 滥用 |
| 协作路由 | Room 中再找 Leader / Worker | assignee、Squad Leader 或显式 `@Agent` | 路由可以变化，但必须携带当前 Task 上下文 |
| 历史上下文 | 分散在 Task、Room 和 Manager 私聊 | 聚合在 Issue 时间线和同一 Agent + Issue 会话 | Task 应成为可追溯的上下文锚点 |
| Manager 私聊 | 可能被期待为万能入口 | Chat 与 Issue 分离，没有模糊续接 | 若保留 Manager 入口，必须先解析并确认目标 Task |

---

## 7. 候选结论摘要

### 7.1 产品边界

| 对象 | 候选定位 |
| --- | --- |
| Task 详情 | 状态事实源，也是“继续处理”的默认入口 |
| Room | 多 Agent 讨论空间；执行指令必须绑定 Task |
| Manager 私聊 | 处理普通请求；继续已有任务前先确认目标 Task |

### 7.2 默认规则

- 同一交付物的修正 → 原 Task 新 Run；独立工作 → 子 Task；范围改变 → 新 Task；
- Task 与 Room 双向关联，Room 支持类型分组和归档；
- Manager 不能在多个相似 Task 之间静默猜测；
- 若保留 Team + 唯一 Leader，Team 不强制包含 Manager；
- 指派优先级：用户意图 > 权限和能力 > Team 规则 > Leader 偏好 > 负载；
- Worker 执行结束进入 `in_review`，不等于 Task 已完成；
- 权限由服务端校验，不由角色名称或 Prompt 暗示。

### 7.3 第一阶段

1. Task 详情增加“继续处理 / 追加要求”；
2. 明确展示本次会创建 Run、子 Task 还是新 Task；
3. 后续指令、接收者、Run 和结果进入原 Task 时间线；
4. Task 与 Room 双向跳转，Room 支持分组和归档；
5. Manager 无法唯一定位 Task 时要求用户确认；
6. 后续再补 Leader、Planner、指派和验收治理。
