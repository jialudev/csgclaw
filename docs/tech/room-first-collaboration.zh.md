# Room-first Manager 协作：首版实现与试用

本版支持内置 IM 的按需协作：先建房间并选择成员，再发送任务；Manager 统一收消息、规划具体子任务，Worker 串行执行，最后由 Manager 汇总。

## 用户入口

1. 在工作区新建房间，保留默认勾选的「按需协作」，选择需要的 Worker。Manager 自动加入；Manager 不可用时不能创建协作房间。
2. 在房间中发送明确的工作要求，例如「请 DEV 开发一个静态页面，再由 QA 测试并给出问题清单」。无需手动指派任务，也无需 @；用户 @ 某个 Worker 仍由 Manager 接收。
3. Manager 创建主任务并规划。房间显示计划和派工记录，任务看板显示主任务、子任务、负责人、状态和结果。
4. Worker 完成后更新子任务，再派发下一项已满足依赖的工作。结束后查看 Manager 的交付总结、测试发现与未解决问题。

「新建任务」入口也先打开建房间流程。取消「按需协作」可创建普通房间；普通房间、私聊和已有 Team 流程保留原行为。

## 关键约束

| 项目 | 本版行为 |
| --- | --- |
| 成员来源 | Room 是唯一成员来源，新流程不创建、不保存 Team 成员名单，也不创建第二个执行房间 |
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

## 实现位置

| 层 | 内容 |
| --- | --- |
| Web | 协作房间入口、自动选择 Manager、隐藏广播开关、任务归属与总结状态 |
| IM / API | 房间配置、Manager 收口路由、正式派工校验、成员变更保护、Room task 接口 |
| taskcore / team 调度器 | Room 归属持久化、临时调度上下文、规划校验、串行执行、失败收尾、总结和投递重试 |
| CLI / Agent 模板 | room-task 命令与 Manager / Worker 操作约束 |

复用了现有调度器，因此内部仍有 TeamMeta / TeamTask 类型名和 room: 前缀的调度命名空间；这不是新增的 Team 实体或第二份成员存储。现有 Team 功能没有在本次改动中整体删除。

## CLI 调试

以下命令用于检查流程；日常用户只需在房间发送消息。room_id、message_id、requester_id 和 task_id 必须使用实际返回值。

```bash
csgclaw-cli room-task list --room <room_id>
csgclaw-cli room-task submit --room <room_id> --source-message <message_id> --actor-id <requester_id> --title "开发并测试页面" --body "交付页面和测试结果"
csgclaw-cli room-task plan --room <room_id> --task <root_task_id>
csgclaw-cli room-task claim --room <room_id> --task <child_task_id> --actor-id <worker_participant_id>
csgclaw-cli room-task update --room <room_id> --task <child_task_id> --actor-id <worker_participant_id> --status completed --result "产物路径与测试发现"
csgclaw-cli room-task report --room <room_id> --task <root_task_id> --outcome issues --result "已交付页面，仍有待处理的问题"
csgclaw-cli room-task retry-delivery --room <room_id>
```

plan 会调用 Manager 规划服务并启动派工；Worker 必须成功 claim 后执行。retry-delivery 重放已记录的消息，不重新运行任务。

## 验证与试用前提

- Go 定向测试覆盖：Room 任务持久化、并发提交去重、重启后去重、串行 DEV → QA、成员移除、非法计划、失败收尾、总结投递重试，以及正式派工才唤醒 Worker。
- 前端完整检查通过：ESLint、格式、TypeScript、1,051 个测试；临时目录 Vite 构建通过，未改动嵌入式静态产物。
- 全量 Go 测试未全绿：存在缺少打包后的 Codex CLI、端口占用及远端服务/飞书相关失败，已在干净上游基线复核同类失败。
- race 检查受本机 Go 工具链的 runtime/race 缺失错误阻断，未获得竞态检查结论。
- 尚未使用真实模型及运行中的 DEV / QA 沙箱完成端到端验收；自动恢复执行、停机中断后的现场恢复仍需专项验证。首版不承诺服务重启后自动续跑。

试用需按项目现有构建流程生成配套的服务、Web 与 csgclaw-cli，并使 Manager / Worker 使用本分支的运行指令和 CLI。只更新浏览器页面或保留旧 Worker CLI，不能验证完整闭环。本分支未发布安装包、镜像或升级现有运行环境。

建议真实验收依次覆盖：无 @ 发任务、@ DEV 仍由 Manager 接收、DEV → QA 串行、QA 带缺陷完成、Worker 失败/停止、移除待执行成员、发送新需求被明确拒绝，以及总结投递重试不重复执行。

飞书接入、同房间多主任务、执行中需求调整、自动返工、自主协作与 thread 不在本版实现中。
