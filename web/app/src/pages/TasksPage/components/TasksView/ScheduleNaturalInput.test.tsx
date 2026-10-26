import { useState } from "react";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { ScheduleNaturalInput } from "./ScheduleNaturalInput";
import type { ScheduledTaskFormDraft } from "./taskDialogTypes";
import { parseScheduledTask } from "@/api/scheduledTasks";
vi.mock("@/api/scheduledTasks", () => ({ parseScheduledTask: vi.fn() }));
const draft: ScheduledTaskFormDraft = {
  title: "",
  prompt: "",
  agentID: "",
  date: "",
  time: "",
  expiresDate: "",
  recurrence: "once",
  timeZone: "Asia/Shanghai",
};
function Fixture() {
  const [value, setValue] = useState(draft);
  return (
    <>
      <ScheduleNaturalInput
        draft={value}
        onChange={setValue}
        onBusyChange={() => {}}
        disabled={false}
        t={(key) => key}
      />
      <output>
        {value.title}|{value.date}|{value.time}|{value.agentID}
      </output>
    </>
  );
}
test("natural-language input populates an editable preview without saving", async () => {
  vi.mocked(parseScheduledTask).mockResolvedValue({
    title: "Morning tasks",
    prompt: "Summarize tasks",
    agent_id: "agent-manager",
    recurrence: "daily",
    first_run_at: "2026-10-27T01:00:00Z",
    time_zone: "Asia/Shanghai",
    client_request_id: "preview-1",
  });
  render(<Fixture />);
  const user = userEvent.setup();
  await user.type(screen.getByRole("textbox", { name: "scheduleNaturalLabel" }), "每天9点整理待办");
  await user.click(screen.getByRole("button", { name: "scheduleParse" }));
  expect(await screen.findByText("Morning tasks|2026-10-27|09:00|agent-manager")).toBeVisible();
  expect(parseScheduledTask).toHaveBeenCalledWith("每天9点整理待办", "Asia/Shanghai", undefined);
  expect(screen.getByText("schedulePreviewReady")).toBeVisible();
});
test("clarification leaves the user's existing form alone", async () => {
  vi.mocked(parseScheduledTask).mockResolvedValue({ clarification: "几点执行？" } as Awaited<
    ReturnType<typeof parseScheduledTask>
  >);
  render(<Fixture />);
  const user = userEvent.setup();
  await user.type(screen.getByRole("textbox", { name: "scheduleNaturalLabel" }), "明天提醒我");
  await user.click(screen.getByRole("button", { name: "scheduleParse" }));
  expect(await screen.findByText("几点执行？")).toBeVisible();
  expect(screen.getByText("|||")).toBeVisible();
});
