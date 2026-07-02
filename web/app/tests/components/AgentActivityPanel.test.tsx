import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import type { ReactElement } from "react";
import { AgentActivityPanel } from "@/pages/AgentPage/components/AgentDetailPane/AgentActivityPanel";
import { AgentActivityMsgTypes, CSGCLAW_AGENT_ACTIVITY_TYPE } from "@/shared/constants/messages";

const labels: Record<string, string> = {
  agentActivityAction: "Action",
  agentActivityDescription: "Activity description",
  agentActivityEmpty: "No activity",
  agentActivityLoadFailed: "Activity failed",
  agentActivityLoading: "Loading activity",
  agentActivityNoRooms: "No rooms",
  agentActivityNotice: "Notice",
  agentActivityRefresh: "Refresh",
  agentActivityReply: "Reply",
  agentActivityTitle: "Activity",
  agentActivityTool: "Tool",
  agentActivityCommand: "Command",
  agentActivityChronological: "Chronological",
  agentActivityClearFilters: "Clear filters",
  agentActivityResult: "Result",
  agentActivityEventsCount: "{count} events",
  agentActivityFilter: "Filter",
  agentActivityFilterActiveCount: "{count} filters",
  agentActivityFilteredEventsCount: "{shown}/{total} events",
  agentActivityInput: "Input",
  agentActivityMessageFilter: "Messages",
  agentActivityNewestFirst: "Newest first",
  agentActivityNoFilteredResults: "No matching activity",
  agentActivityOtherFilter: "Other",
  agentActivityRemoveFilter: "Remove filter {label}",
  agentActivitySelectedFiltersLabel: "Selected filters",
  agentActivitySortLabel: "Activity sort",
  agentActivitySummaryLabel: "Activity summary",
  agentActivityTimelineLabel: "Activity timeline",
  agentActivityTimelineSegment: "{label} #{index} {time}",
  agentActivityToolCallsCount: "{count} tool calls",
};

function t(key: string, params: Record<string, string | number> = {}): string {
  return (labels[key] ?? key).replace(/\{(\w+)}/g, (_, name) => String(params[name] ?? ""));
}

function renderWithClient(ui: ReactElement) {
  const queryClient = new QueryClient({
    defaultOptions: {
      queries: { retry: false },
    },
  });
  return render(<QueryClientProvider client={queryClient}>{ui}</QueryClientProvider>);
}

describe("AgentActivityPanel", () => {
  afterEach(() => {
    vi.unstubAllGlobals();
  });

  it("loads thread-inclusive room messages and shows tool activity plus text replies", async () => {
    const user = userEvent.setup();
    const fetchMock = vi.fn().mockResolvedValue(
      new Response(
        JSON.stringify([
          {
            id: "tool-1",
            sender_id: "manager",
            created_at: "2026-07-01T10:00:00Z",
            content: JSON.stringify({
              type: CSGCLAW_AGENT_ACTIVITY_TYPE,
              channel: "csgclaw",
              sender: "manager",
              content: {
                msgtype: AgentActivityMsgTypes.tool,
                body: "Tool running",
                tool: {
                  id: "exec-1",
                  input_summary: '{"cmd":"pwd"}',
                  output_summary: '{"output":"/workspace"}',
                  kind: "execute",
                  status: "completed",
                  title: "Run shell command",
                },
              },
            }),
          },
          {
            id: "reply-1",
            sender_id: "manager",
            created_at: "2026-07-01T10:00:02Z",
            content: "Done with the workspace check.",
          },
          {
            id: "legacy-tool-1",
            sender_id: "manager",
            created_at: "2026-07-01T10:00:03Z",
            content: `🔧 \`exec\`
\`\`\`json
{"command":"ls web/app","output":"src\\ntests"}
\`\`\``,
          },
        ]),
        { headers: { "Content-Type": "application/json" }, status: 200 },
      ),
    );
    vi.stubGlobal("fetch", fetchMock);

    renderWithClient(
      <AgentActivityPanel
        item={{
          id: "agent-manager",
          name: "manager",
          role: "manager",
          participants: [{ channel: "csgclaw", channel_user_ref: "manager", id: "manager" }],
        }}
        locale="en"
        rooms={[
          {
            id: "room-1",
            title: "Dev group",
            members: ["manager", "u-admin"],
            messages: [{ id: "root", sender_id: "u-admin", content: "please inspect" }],
            threads: [{ root_message_id: "root" }],
          },
        ]}
        t={t}
      />,
    );

    await waitFor(() =>
      expect(fetchMock).toHaveBeenCalledWith(
        "api/v1/messages?room_id=room-1&include_thread_replies=true",
        expect.anything(),
      ),
    );

    expect(await screen.findAllByText("exec_command")).toHaveLength(2);
    expect(screen.getByText(/pwd/)).toBeInTheDocument();
    expect(screen.queryByText("/workspace")).not.toBeInTheDocument();
    expect(screen.getByText("Done with the workspace check.")).toBeInTheDocument();
    expect(screen.getAllByText("Dev group")).toHaveLength(3);
    expect(screen.getByText("3s")).toBeInTheDocument();
    expect(screen.getByText("2 tool calls")).toBeInTheDocument();
    expect(screen.getByText("3 events")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: /Newest first/ })).toHaveAttribute("aria-pressed", "true");
    let rows = await screen.findAllByRole("listitem");
    expect(rows).toHaveLength(3);
    expect(within(rows[0]).getByText(/ls web\/app/)).toBeInTheDocument();
    expect(within(rows[0]).getByText("#3")).toBeInTheDocument();
    expect(within(rows[1]).getByText("Done with the workspace check.")).toBeInTheDocument();
    expect(within(rows[1]).getByText("#2")).toBeInTheDocument();
    expect(within(rows[2]).getByText(/pwd/)).toBeInTheDocument();
    expect(within(rows[2]).getByText("#1")).toBeInTheDocument();

    await user.click(screen.getByRole("button", { name: /Chronological/ }));
    expect(screen.getByRole("button", { name: /Chronological/ })).toHaveAttribute("aria-pressed", "true");
    rows = await screen.findAllByRole("listitem");
    expect(within(rows[0]).getByText(/pwd/)).toBeInTheDocument();
    expect(within(rows[0]).getByText("#1")).toBeInTheDocument();
    expect(within(rows[2]).getByText(/ls web\/app/)).toBeInTheDocument();
    expect(within(rows[2]).getByText("#3")).toBeInTheDocument();

    const timeline = screen.getByRole("navigation", { name: "Activity timeline" });
    const segments = within(timeline).getAllByRole("button");
    await user.click(segments[0]);
    expect(rows[0]).toHaveClass("selected");

    await user.click(screen.getByRole("button", { name: /pwd/ }));
    expect(screen.getByText("Command")).toBeInTheDocument();
    expect(screen.getByText("Result")).toBeInTheDocument();
    expect(screen.getByText("/workspace")).toBeInTheDocument();

    await user.click(screen.getByRole("button", { name: /Filter/ }));
    await user.click(screen.getByRole("menuitemcheckbox", { name: "Tool:exec (1)" }));
    await waitFor(() => expect(screen.getAllByRole("listitem")).toHaveLength(1));
    expect(screen.getByText("1/3 events")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: /Remove filter Tool:exec/ })).toBeInTheDocument();
    expect(screen.queryByText("Done with the workspace check.")).not.toBeInTheDocument();

    expect(screen.queryByText(/src\\ntests/)).not.toBeInTheDocument();
    await user.click(screen.getByRole("button", { name: /ls web\/app/ }));
    expect(screen.getByText(/src/)).toBeInTheDocument();
    expect(screen.getByText(/tests/)).toBeInTheDocument();
  });

  it("folds OpenClaw plain command output under the command row", async () => {
    const user = userEvent.setup();
    const fetchMock = vi.fn().mockResolvedValue(
      new Response(
        JSON.stringify([
          {
            id: "search-start",
            sender_id: "openclaw-weather",
            created_at: "2026-07-01T10:00:00Z",
            content: '🔎 Web Search: for "上海天气" (top 5)',
          },
          {
            id: "search-result",
            sender_id: "openclaw-weather",
            created_at: "2026-07-01T10:00:01Z",
            content: '🔎 Web Search: for "上海天气" (top 5) {"query":"上海天气","count":5,"status":"ok"}',
          },
          {
            id: "reply-1",
            sender_id: "openclaw-weather",
            created_at: "2026-07-01T10:00:02Z",
            content: "上海这周多云。",
          },
        ]),
        { headers: { "Content-Type": "application/json" }, status: 200 },
      ),
    );
    vi.stubGlobal("fetch", fetchMock);

    renderWithClient(
      <AgentActivityPanel
        item={{
          id: "agent-openclaw-weather",
          name: "openclaw-weather",
          role: "worker",
          participants: [
            {
              channel: "csgclaw",
              channel_user_ref: "openclaw-weather",
              id: "openclaw-weather",
            },
          ],
        }}
        locale="zh-CN"
        rooms={[
          {
            id: "room-weather",
            title: "天气查询小队",
            members: ["openclaw-weather", "admin"],
            messages: [{ id: "root", sender_id: "admin", content: "查天气" }],
          },
        ]}
        t={t}
      />,
    );

    expect(await screen.findAllByText("exec_command")).toHaveLength(1);
    expect(screen.getByText("message")).toBeInTheDocument();
    expect(screen.getByText('Web Search: for "上海天气" (top 5)')).toBeInTheDocument();
    expect(screen.queryByText(/"status": "ok"/)).not.toBeInTheDocument();

    const weatherCommandRow = screen
      .getAllByRole("listitem")
      .find((row) => within(row).queryByText('Web Search: for "上海天气" (top 5)'));
    expect(weatherCommandRow).toBeTruthy();
    await user.click(within(weatherCommandRow as HTMLElement).getByRole("button", { name: /Web Search/ }));

    expect(screen.getByText("Result")).toBeInTheDocument();
    expect(screen.getByText(/"status": "ok"/)).toBeInTheDocument();
    expect(screen.getByText("上海这周多云。")).toBeInTheDocument();
  });
});
