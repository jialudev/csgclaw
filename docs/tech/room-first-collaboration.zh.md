# Room-first Manager 协作：首版实现与试用

本版支持内置 IM 的按需协作：先建房间并选择成员，再发送任务；Manager 统一收消息、规划具体子任务，Worker 串行执行，最后由 Manager 汇总。

## 用户入口

1. 在工作区新建房间，保留默认勾选的「按需协作」，选择需要的 Worker。Manager 自动加入；Manager 不可用时不能创建协作房间。
2. 在房间中发送明确的工作要求，例如「请 DEV 开发一个静态页面，再由 QA 测试并给出问题清单」。无需手动指派任务，也无需 @；用户 @ 某个 Worker 仍由 Manager 接收。
3. Manager 创建主任务并规划。聊天区域的「房间任务 · 拆解计划」面板显示真实主任务、子任务、负责人、依赖、执行状态和结果；可展开/收起，也可点击任务标题打开看板详情。页面每 3 秒同步一次，刷新页面后仍能查看。
4. Worker 完成后更新子任务，再派发下一项已满足依赖的工作。结束后查看 Manager 的交付总结、测试发现与未解决问题。

「新建任务」入口也先打开建房间流程。取消「按需协作」可创建普通房间；普通房间、私聊和已有 Team 流程保留原行为。

任务面板以服务端任务记录为准，不从自然语言或 @ 推断任务。没有记录时显示「尚未创建任务」；创建主任务但未拆解时显示等待规划；读取失败时提示同步异常，不伪装为空任务或已完成。执行结束后单独展示 Manager 的目标评估和总结投递状态。

## 关键约束

| 项目 | 本版行为 |
| --- | --- |
| 成员来源 | Room 是唯一成员来源，新流程不创建、不保存 Team 成员名单，也不创建第二个执行房间 |
| 运行时查询范围 | Manager 通过 room-task context 读取当前成员；Codex 运行时 CLI 调用按活跃执行租约限制范围，拒绝全局 Agent/Team 列表、旧私聊任务入口和不相关房间 |
| 任务归属 | 主任务和子任务以 room assignment 持久化；调度时临时读取当前房间成员 |
| 消息入口 | 用户消息、Worker 求助统一进入 Manager；Worker 只接受调度器已记录的正式派工 |
| 执行方式 | 串行，满足依赖后派发；没有依赖的子任务按稳定的计划顺序处理 |
| 重试 | 同一原始消息重试返回同一主任务，重启后仍可去重 |
| 主任务数量 | 一个协作房间仅一个主任务，结束后也不创建第二个；新任务需另建房间 |
| 变更需求 | 不支持执行中改计划、加任务、自动返工；Manager 明确说明限制，不声称已完成修改 |
| 成员变更 | 禁止移除 Manager；执行中的 Worker 需先停止。移除尚未执行的负责人会阻塞对应任务，不静默换人 |
| 失败与停止 | 失败/确认停止后收尾主任务和未开始的子任务，保留已完成结果，不把状态更新等同于运行时已停止 |
| 结果判断 | 执行 completed 不代表目标成功。Manager 分别记录 succeeded / issues / failed / stopped 与交付总结 |
| 投递 | 总结先持久化，再投递原房间；投递待重试不代表需要重做任务 |

闲聊、查询进度和解释结果不创建任务；意图识别与总结质量依赖 Manager 模型遵守运行指令。后端负责成员校验、单主任务约束、串行派工和事件去重。

## Worker 的任务 Session

主任务用于 Manager 统筹；具体执行始终落到子任务，即使只有一个 Worker 也要规划子任务，不降级为旧的直接 Agent 任务。

内置 IM 的 Codex 执行链路使用 Binding + Room + 子任务 ID 作为会话标识：

- 同一 Worker 的不同子任务不共享对话历史，房间聊天、私聊与任务执行分别隔离。
- 同一子任务的继续处理复用原 session；交互问题的回答也保留任务归属。
- 内置 IM 执行层允许独立任务 session 异步启动，新任务不会取消另一任务或重置其会话；同一 session 内仍按顺序处理。
- 派工事件的任务 ID 由服务端从已验证的调度记录取得，不从模型正文或用户伪造的元数据推断。
- 任务 session 是内部执行标识，不等于新 Room 或 UI thread；状态、产物和结果仍回原 Room。

调度策略与执行能力分开：首版房间调度器仍串行、一个主任务；独立 session 的能力不代表已开放追加主任务、执行中改需求或自动并行派工。Session 隔离也不等于工作目录/文件写入隔离。OpenClaw 沙箱 channel 的任务级 session 仍需单独适配，本次验证针对内置 IM/Codex 链路。

## 实现位置

| 层 | 内容 |
| --- | --- |
| Web | 协作房间入口、自动选择 Manager、隐藏广播开关、聊天内真实任务计划面板、详情跳转与总结状态 |
| IM / API | 房间配置、Manager 收口路由、正式派工校验、成员变更保护、Room task 接口 |
| taskcore / team 调度器 | Room 归属持久化、临时调度上下文、规划校验、串行执行、失败收尾、总结和投递重试 |
| CLI / Agent 模板 | room-task 命令与 Manager / Worker 操作约束 |
| Channel / Engine | 可信任务 ID 贯穿消息接入、会话键与交互续接，独立任务不会互相覆盖或取消 |

复用了现有调度器，因此内部仍有 TeamMeta / TeamTask 类型名和 room: 前缀的调度命名空间；这不是新增的 Team 实体或第二份成员存储。现有 Team 功能没有在本次改动中整体删除。

## CLI 调试

以下命令用于检查流程；日常用户只需在房间发送消息。room_id、message_id、requester_id 和 task_id 必须使用实际返回值。

```bash
csgclaw-cli room-task context --room <room_id>
csgclaw-cli room-task list --room <room_id>
csgclaw-cli room-task submit --room <room_id> --source-message <message_id> --actor-id <requester_id> --title "开发并测试页面" --body "交付页面和测试结果"
csgclaw-cli room-task plan --room <room_id> --task <root_task_id>
csgclaw-cli room-task claim --room <room_id> --task <child_task_id> --actor-id <worker_participant_id>
csgclaw-cli room-task update --room <room_id> --task <child_task_id> --actor-id <worker_participant_id> --status completed --result "产物路径与测试发现"
csgclaw-cli room-task report --room <room_id> --task <root_task_id> --outcome issues --result "已交付页面，仍有待处理的问题"
csgclaw-cli room-task retry-delivery --room <room_id>
```

plan 会调用 Manager 规划服务并启动派工；Worker 必须成功 claim 后执行。retry-delivery 重放已记录的消息，不重新运行任务。

### 配套 CLI 选择

宿主 Codex Manager / Worker 由服务注入 `CSGCLAW_CLI`，指向运行中 `csgclaw` 可执行文件同目录的 `csgclaw-cli`（Windows 为 `.exe`）。运行指令和派工消息要求通过该绝对路径调用；不再依赖登录 shell 的 PATH。服务通过符号链接启动时先解析真实路径，配套文件缺失时直接报错，不回退到系统旧版本。

排查 `unknown command "room-task"` 时，分别检查开发目录的 `bin/csgclaw-cli room-task -h` 与 `command -v csgclaw-cli` 指向的版本。源码/服务更新不等于用户安装目录中的 CLI 已更新。此机制不覆盖沙箱镜像 CLI 的升级，也不会修改用户系统 PATH 或覆盖全局安装文件。

## 验证与试用前提

- Go 定向测试覆盖：Room 任务持久化、并发提交去重、重启后去重、串行 DEV → QA、成员移除、非法计划、失败收尾、总结投递重试，以及正式派工才唤醒 Worker。
- 补充测试覆盖：阻止运行时查询全局成员/创建旧私聊任务/访问其他房间，当前成员上下文不包含外部 Worker，任务身份跨接入层传递，任务 A 未结束时 B 可异步启动且不取消 A。
- 前端检查覆盖 ESLint、格式、TypeScript、任务计划/负责人/依赖展示、空态/错误态、房间切换隔离及任务详情链接；使用临时目录验证 Vite 构建，不改动嵌入式静态产物。
- Rebase 到 `fcc25ecb` 的 Agent Engine 重构后，全量 `go test ./...` 通过。任务 session 隔离接入新的 Channel Event / Engine 交互续接，不恢复已删除的 legacy bridge。
- race 检查受本机 Go 工具链的 runtime/race 缺失错误阻断，未获得竞态检查结论。
- 尚未使用真实模型及运行中的 DEV / QA 沙箱完成端到端验收；自动恢复执行、停机中断后的现场恢复仍需专项验证。首版不承诺服务重启后自动续跑。

试用需按项目现有构建流程生成配套的服务、Web 与 csgclaw-cli，并使 Manager / Worker 使用本分支的运行指令和 CLI。只更新浏览器页面或保留旧 Worker CLI，不能验证完整闭环。本分支未发布安装包、镜像或升级现有运行环境。

本轮运行时范围检查需要重新构建服务和 csgclaw-cli，并在安全停止已有工作后重启相应 Codex 运行时，使其获得新的调用身份环境变量和托管指令。旧流程已经创建在私聊的任务不会自动迁移、删除或重跑。运行时 CLI 的调用身份用于功能范围约束，不是恶意代码隔离；持有服务全局凭据的宿主进程仍有管理员权限，不能据此宣称实现了强安全沙箱。

建议真实验收依次覆盖：无 @ 发任务、@ DEV 仍由 Manager 接收、DEV → QA 串行、QA 带缺陷完成、Worker 失败/停止、移除待执行成员、发送新需求被明确拒绝，以及总结投递重试不重复执行。

飞书接入、同房间多主任务、执行中需求调整、自动返工、自主协作与 thread 不在本版实现中。
