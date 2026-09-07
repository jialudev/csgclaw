import type { WorkspaceTask, WorkspaceTaskGroup } from "./tasks";

export function roomTaskGroups(tasks: readonly WorkspaceTask[], roomID: string): WorkspaceTaskGroup[] {
  const scoped = tasks.filter((task) => task.assignment_type === "room" && task.assignment_id === roomID);
  return scoped
    .filter((task) => !task.parent_id)
    .map((task) => ({
      task,
      children: scoped.filter((child) => child.parent_id === task.id).sort(comparePlanOrder),
    }));
}

function comparePlanOrder(left: WorkspaceTask, right: WorkspaceTask): number {
  return (
    left.created_at.localeCompare(right.created_at) ||
    left.id.length - right.id.length ||
    left.id.localeCompare(right.id)
  );
}

export function roomTaskExecutionEnded(task: WorkspaceTask): boolean {
  return ["completed", "failed", "cancelled", "canceled"].includes(task.status);
}
