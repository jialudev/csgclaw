---
name: agent-teams
description: Default manager skill for multi-worker CSGClaw orchestration. Create teams and tasks, plan and start work through `csgclaw-cli team`, and prefer this over manager-worker-dispatch so dispatch, claim, and status stay on server task state.
---

# Agent Teams Manager

Use this skill when you are the manager and need to coordinate one or more workers through CSGClaw team tasks.

Use `manager-worker-dispatch` only when the user explicitly needs tracker-driven sequential handoff outside team tasks.

## Default orchestration path

For executable multi-worker work:

1. Ensure a team exists (`team create` or reuse an existing `team_id`).
2. Create one main task (`team task create-batch`) for the user-visible goal.
3. Plan and start it with `team task plan --start`; this uses the server planner, creates child tasks, creates dependencies, and dispatches ready child tasks.
4. Use `team task start` only when the parent already has subtasks but is not started yet. When `create-batch` creates a parent and runnable subtasks in the same batch, the server auto-starts the parent and projects dispatches.
5. Do **not** also run `manager-worker-dispatch` / `start-tracking` for the same work.

Workers must claim and update tasks via `agent-teams` (specified `team task claim --task`, not ad-hoc room handoff).

## Manager actions

- create a team and tasks with `team create` / `team task create-batch`
- plan and start parent tasks through the shipped CLI
- inspect progress with `team task list`
- inspect pending approvals with `team approval list`
- resolve approvals with `team approval resolve`
- summarize status back into the execution room in plain language

Do not invent extra commands. Stay within the shipped CLI surface and Web Tasks controls.

## Commands

Create or enable a team room:

```bash
csgclaw-cli team create --channel csgclaw --room-id <room_id> --lead-participant-id <manager_participant_id> --member-participant-ids <worker_participant_ids>
```

Create one or more tasks:

```bash
csgclaw-cli team task create-batch --team <team_id> --created-by <manager_participant_id> --file <tasks.json>
```

Start an existing parent task after subtasks are attached:

```bash
csgclaw-cli team task start --team <team_id> --task <parent_task_id>
```

Plan and start a parent task through the server planner:

```bash
csgclaw-cli team task plan --team <team_id> --task <parent_task_id> --start
```

Recommended batch shape for the default path:

```json
{
  "tasks": [
    {
      "id_ref": "story",
      "title": "Release v1",
      "body": "Goal, context, scope, and acceptance criteria for the server planner."
    }
  ]
}
```

Attach a new subtask to an existing main task with `parent_id`:

```json
{
  "tasks": [
    {
      "title": "Prepare rollback note",
      "parent_id": "task-12",
      "assign_to": "writer"
    }
  ]
}
```

List tasks:

```bash
csgclaw-cli team task list --team <team_id>
```

List approvals:

```bash
csgclaw-cli team approval list --team <team_id>
```

Resolve an approval:

```bash
csgclaw-cli team approval resolve --team <team_id> --approval <approval_id> --status approved --reason "<note>"
csgclaw-cli team approval resolve --team <team_id> --approval <approval_id> --status rejected --reason "<reason>"
```

## Working Rules

- For any team task that creates, inspects, or updates a project, use `~/.openclaw/workspace/projects/{name}` as the shared project directory. Check there first before creating a new project path.
- Keep project artifacts, notes, and generated files under the same `~/.openclaw/workspace/projects/{name}` tree so managers and workers can inspect the same content.
- Default to one top-level main task for each user-visible goal, then add execution items as child tasks.
- In the same batch, prefer `id_ref` + `parent_ref` to build the hierarchy clearly.
- Add `depends_on_refs` when testing, QA, review, validation, or release checks must wait for implementation work.
- Use `parent_id` only when attaching a child task to an already-existing main task.
- Do not flatten every execution step into a top-level task; the global Tasks page groups by main task.
- Prefer a single-item batch when you only need one task; do not wait for a separate single-task command.
- After auto-start or `team task start`, collaboration for that main task happens in its execution room (not the team home room).
- Keep plans small and explicit when possible; prefer clear parent/child tasks over flat room chatter.
- After listing tasks and approvals, summarize only the current state, blockers, and next action.
- If a human already handled an approval or cancellation in the room with `approve`, `reject`, or `cancel`, refresh with `team task list` or `team approval list` before acting again.
