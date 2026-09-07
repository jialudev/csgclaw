import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { MemoryRouter } from "react-router-dom";
import { describe, expect, it, vi } from "vitest";
import { normalizeTaskList } from "@/models/tasks";
import { RoomTaskPlan } from "./RoomTaskPlan";

const t = (key: string) => key;
const tasks = normalizeTaskList([
  {
    id: "root",
    title: "Travel website",
    assignment_type: "room",
    assignment_id: "r",
    status: "completed",
    goal_outcome: "issues",
    report: "Accessibility fixes needed",
    report_status: "pending",
  },
  {
    id: "dev",
    title: "Build pages",
    parent_id: "root",
    assignment_type: "room",
    assignment_id: "r",
    assigned_to_agent_name: "Developer",
    status: "completed",
  },
  {
    id: "qa",
    title: "Test pages",
    parent_id: "root",
    assignment_type: "room",
    assignment_id: "r",
    assigned_to_agent_name: "QA",
    depends_on: ["dev"],
    status: "completed",
  },
]);

describe("RoomTaskPlan", () => {
  it("shows the real breakdown, owners, dependencies, task links and assessment", async () => {
    const user = userEvent.setup();
    render(
      <MemoryRouter>
        <RoomTaskPlan roomID="r" tasks={tasks} loading={false} error="" refreshing={false} onRefresh={vi.fn()} t={t} />
      </MemoryRouter>,
    );
    expect(screen.getByRole("link", { name: "Travel website" })).toHaveAttribute("href", "/tasks/root");
    expect(screen.getByRole("link", { name: "Test pages" })).toHaveAttribute("href", "/tasks/qa");
    expect(screen.getByText("taskAssigneeLabel: Developer")).toBeInTheDocument();
    expect(screen.getByText("taskDependsOnLabel: Build pages")).toBeInTheDocument();
    expect(screen.getByText("roomTaskOutcome.issues")).toBeInTheDocument();
    expect(screen.getByText("roomTaskReportPending")).toBeInTheDocument();
    await user.click(screen.getByRole("button", { name: /roomPlanTitle/ }));
    expect(screen.queryByRole("link", { name: "Test pages" })).not.toBeInTheDocument();
  });

  it("distinguishes an empty task store from a loading or sync failure and supports retry", async () => {
    const retry = vi.fn();
    const props = { roomID: "r", tasks: [], refreshing: false, onRefresh: retry, t };
    const { rerender } = render(
      <MemoryRouter>
        <RoomTaskPlan {...props} loading error="" />
      </MemoryRouter>,
    );
    expect(screen.getByRole("status")).toHaveTextContent("roomPlanLoading");
    expect(screen.queryByText("roomPlanEmpty")).not.toBeInTheDocument();
    rerender(
      <MemoryRouter>
        <RoomTaskPlan {...props} loading={false} error="" />
      </MemoryRouter>,
    );
    expect(screen.getByText("roomPlanEmpty")).toBeInTheDocument();
    rerender(
      <MemoryRouter>
        <RoomTaskPlan {...props} loading={false} error="offline" />
      </MemoryRouter>,
    );
    expect(screen.getByRole("alert")).toHaveTextContent("offline");
    expect(screen.queryByText("roomPlanEmpty")).not.toBeInTheDocument();
    await userEvent.setup().click(screen.getByRole("button", { name: "tasksRefreshShort" }));
    expect(retry).toHaveBeenCalledOnce();
  });
});
