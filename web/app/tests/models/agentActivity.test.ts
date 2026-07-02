import { isToolActivityMessage, parseAgentActivity, parsePlainAgentCommand } from "@/models/agentActivity";
import { AgentActivityKinds, AgentActivityMsgTypes, CSGCLAW_AGENT_ACTIVITY_TYPE } from "@/shared/constants/messages";

describe("agent activity model", () => {
  it("parses tool activity payloads", () => {
    const activity = parseAgentActivity(
      JSON.stringify({
        type: CSGCLAW_AGENT_ACTIVITY_TYPE,
        version: 1,
        channel: "csgclaw",
        event_id: "act-1",
        room_id: "room-1",
        sender: "u-codex",
        origin_server_ts: 1779259200000,
        content: {
          msgtype: AgentActivityMsgTypes.tool,
          body: "Running tool",
          tool: {
            id: "tool-1",
            kind: "execute",
            status: "running",
            title: "Run shell command",
          },
        },
      }),
    );

    expect(activity?.content.tool).toMatchObject({
      id: "tool-1",
      kind: "execute",
      status: "running",
      title: "Run shell command",
    });
    expect(activity?.channel).toBe("csgclaw");
    expect(activity?.version).toBe(1);
  });

  it("defaults legacy activity payloads to version 1", () => {
    const activity = parseAgentActivity(
      JSON.stringify({
        type: CSGCLAW_AGENT_ACTIVITY_TYPE,
        content: {
          msgtype: AgentActivityMsgTypes.tool,
          body: "Running tool",
          tool: { id: "tool-1", status: "running", title: "Run shell command" },
        },
      }),
    );

    expect(activity?.version).toBe(1);
    expect(activity?.channel).toBe("");
  });

  it("classifies tool messages", () => {
    const tool = {
      content: JSON.stringify({
        type: CSGCLAW_AGENT_ACTIVITY_TYPE,
        content: {
          msgtype: AgentActivityMsgTypes.tool,
          body: "Running tool",
          tool: { id: "tool-1", status: "running", title: "Run shell command" },
        },
      }),
    };

    expect(isToolActivityMessage(tool)).toBe(true);
  });

  it("parses OpenClaw plain text command messages with structured output", () => {
    const command = parsePlainAgentCommand(
      '🔎 Web Search: for "上海天气" (top 5) {"query":"上海天气","provider":"duckduckgo","count":5}',
    );

    expect(command).toMatchObject({
      command: 'Web Search: for "上海天气" (top 5)',
      kind: AgentActivityKinds.execCommand,
      signature: 'web search: for "上海天气" (top 5)',
      title: "Web Search",
    });
    expect(command?.output).toContain('"provider": "duckduckgo"');
  });

  it("classifies csgclaw-cli command messages as exec command activity", () => {
    expect(parsePlainAgentCommand("🛠 csgclaw-cli task claim --task task-4 --participant-id pt-worker")).toMatchObject({
      command: "csgclaw-cli task claim --task task-4 --participant-id pt-worker",
      kind: AgentActivityKinds.execCommand,
      title: "csgclaw-cli task",
    });
  });

  it("classifies OpenClaw message receipts as other activity", () => {
    expect(
      parsePlainAgentCommand(`📩 Message
{"channel":"csgclaw","to":"csgclaw:room:room-1","result":{"ok":true}}`),
    ).toMatchObject({
      command: "Message",
      kind: AgentActivityKinds.other,
      title: "Message",
    });
  });

  it("parses legacy fenced command messages", () => {
    const command = parsePlainAgentCommand(`🔧 \`exec\`
\`\`\`json
{"command":"ls web/app","output":"src\\ntests"}
\`\`\``);

    expect(command).toMatchObject({
      command: "ls web/app",
      kind: AgentActivityKinds.execCommand,
      output: "src\ntests",
    });
  });

  it("rejects invalid activity shapes", () => {
    expect(parseAgentActivity("plain text")).toBeNull();
    expect(parseAgentActivity(JSON.stringify({ type: CSGCLAW_AGENT_ACTIVITY_TYPE, content: {} }))).toBeNull();
  });
});
