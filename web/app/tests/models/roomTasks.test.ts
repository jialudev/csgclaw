import { describe, expect, it } from "vitest";
import { normalizeTaskList } from "@/models/tasks";
import { roomTaskExecutionEnded, roomTaskGroups } from "@/models/roomTasks";

describe("room tasks", () => {
  it("only groups persisted tasks owned by this room, in stable plan order", () => {
    const tasks = normalizeTaskList([
      { id: "root", assignment_type: "room", assignment_id: "r" },
      { id: "task-10", parent_id: "root", assignment_type: "room", assignment_id: "r" },
      { id: "task-2", parent_id: "root", assignment_type: "room", assignment_id: "r" },
      { id: "outside", parent_id: "root", assignment_type: "room", assignment_id: "other" },
      { id: "direct", assignment_type: "agent", assignment_id: "r", room_id: "r" },
    ]);
    const groups = roomTaskGroups(tasks, "r");
    expect(groups).toHaveLength(1);
    expect(groups[0].children.map((child) => child.id)).toEqual(["task-2", "task-10"]);
    expect(roomTaskGroups(tasks, "empty")).toEqual([]);
  });

  it("does not confuse execution ending with a successful goal", () => {
    const [task] = normalizeTaskList([
      { id: "root", assignment_type: "room", assignment_id: "r", status: "completed", goal_outcome: "issues" },
    ]);
    expect(roomTaskExecutionEnded(task)).toBe(true);
    expect(task.goal_outcome).toBe("issues");
  });
});
