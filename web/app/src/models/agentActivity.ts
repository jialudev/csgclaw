import { AgentActivityKinds, AgentActivityMsgTypes, CSGCLAW_AGENT_ACTIVITY_TYPE } from "@/shared/constants/messages";
import type { IMMessage } from "@/models/conversations";

type UnknownRecord = Record<string, unknown>;

export type AgentActivityKind = (typeof AgentActivityKinds)[keyof typeof AgentActivityKinds];

export type AgentActivityCommand = {
  command: string;
  kind: typeof AgentActivityKinds.execCommand | typeof AgentActivityKinds.other;
  output?: string;
  signature: string;
  title: string;
};

export type AgentActivityTool = {
  command?: string;
  id: string;
  input?: unknown;
  input_summary?: string;
  kind?: string;
  output?: unknown;
  output_summary?: string;
  status: string;
  title: string;
};

export type AgentActivityActionOption = {
  id: string;
  kind: string;
  label: string;
  scope?: string;
};

export type AgentActivityActionDecision = {
  decided_at?: string;
  kind?: string;
  option_id?: string;
};

export type AgentActivityAction = {
  decision?: AgentActivityActionDecision | null;
  expires_at?: string;
  id: string;
  kind?: string;
  options?: AgentActivityActionOption[];
  requested_at?: string;
  status: string;
  title: string;
};

export type AgentActivityContent = {
  action?: AgentActivityAction;
  body: string;
  msgtype: string;
  tool?: AgentActivityTool;
};

export type AgentActivityPayload = {
  channel: string;
  content: AgentActivityContent;
  event_id: string;
  origin_server_ts: number;
  room_id: string;
  sender: string;
  type: typeof CSGCLAW_AGENT_ACTIVITY_TYPE;
  version: number;
};

export function parseAgentActivity(content: unknown): AgentActivityPayload | null {
  const parsed = typeof content === "string" ? parseJSON(content.trim()) : content;
  if (!isRecord(parsed) || parsed.type !== CSGCLAW_AGENT_ACTIVITY_TYPE || !isRecord(parsed.content)) {
    return null;
  }

  const activityContent = parsed.content;
  const msgtype = stringValue(activityContent.msgtype);
  if (
    !Object.values(AgentActivityMsgTypes).includes(
      msgtype as (typeof AgentActivityMsgTypes)[keyof typeof AgentActivityMsgTypes],
    )
  ) {
    return null;
  }

  return {
    content: {
      action: parseAction(activityContent.action),
      body: stringValue(activityContent.body, "Agent activity"),
      msgtype,
      tool: parseTool(activityContent.tool),
    },
    channel: stringValue(parsed.channel),
    event_id: stringValue(parsed.event_id),
    origin_server_ts: numberValue(parsed.origin_server_ts),
    room_id: stringValue(parsed.room_id),
    sender: stringValue(parsed.sender),
    type: CSGCLAW_AGENT_ACTIVITY_TYPE,
    version: numberValue(parsed.version, 1),
  };
}

export function isToolActivityMessage(message: IMMessage | null | undefined): boolean {
  const activity = parseAgentActivity(message?.content);
  return activity?.content.msgtype === AgentActivityMsgTypes.tool;
}

export function parsePlainAgentCommand(content: unknown): AgentActivityCommand | null {
  if (typeof content !== "string") {
    return null;
  }

  const text = content.replace(/\u200b/g, "").trim();
  if (!text) {
    return null;
  }

  const legacyTool = parseLegacyFencedCommand(text);
  if (legacyTool) {
    return legacyTool;
  }

  const stripped = stripToolMarker(text);
  if (!stripped) {
    return null;
  }

  const split = splitTrailingStructuredOutput(stripped);
  const command = split.command.trim();
  if (!command) {
    return null;
  }

  const title = plainCommandTitle(command);
  const looksLikeCommand = isKnownPlainCommand(command);
  if (!looksLikeCommand && stripped === text) {
    return null;
  }

  return {
    command,
    kind: looksLikeCommand ? AgentActivityKinds.execCommand : AgentActivityKinds.other,
    output: split.output,
    signature: commandSignature(command),
    title,
  };
}

export function actionOptionLabel(option: AgentActivityActionOption): string {
  const label = stringValue(option.label, option.kind, option.id);
  if (optionScope(option) === "agent" && !/\bagent\b/i.test(label)) {
    return `${label} (this agent)`;
  }
  return label;
}

export function statusLabel(status: string): string {
  switch (status) {
    case "allowed":
      return "Allowed";
    case "rejected":
      return "Rejected";
    case "expired":
      return "Expired";
    case "canceled":
      return "Canceled";
    case "completed":
      return "Completed";
    case "failed":
      return "Failed";
    case "running":
      return "Running";
    case "pending":
      return "Pending";
    default:
      return stringValue(status, "Status");
  }
}

function parseTool(value: unknown): AgentActivityTool | undefined {
  if (!isRecord(value)) {
    return undefined;
  }
  return {
    command: stringValue(value.command, value.cmd),
    id: stringValue(value.id),
    input: firstDefined(value.input, value.arguments, value.args),
    input_summary: stringValue(value.input_summary),
    kind: stringValue(value.kind),
    output: firstDefined(value.output, value.result),
    output_summary: stringValue(value.output_summary),
    status: stringValue(value.status, "running"),
    title: stringValue(value.title, "Run tool"),
  };
}

function parseAction(value: unknown): AgentActivityAction | undefined {
  if (!isRecord(value)) {
    return undefined;
  }
  return {
    decision: parseDecision(value.decision),
    expires_at: stringValue(value.expires_at),
    id: stringValue(value.id),
    kind: stringValue(value.kind, "permission"),
    options: Array.isArray(value.options) ? value.options.map(parseOption).filter(isActionOption) : [],
    requested_at: stringValue(value.requested_at),
    status: stringValue(value.status, "pending"),
    title: stringValue(value.title, "Run tool"),
  };
}

function parseOption(value: unknown): AgentActivityActionOption | null {
  if (!isRecord(value)) {
    return null;
  }
  const id = stringValue(value.id);
  if (!id) {
    return null;
  }
  return {
    id,
    kind: stringValue(value.kind),
    label: stringValue(value.label, value.kind, id),
    scope: stringValue(value.scope, defaultOptionScope(stringValue(value.kind))) || undefined,
  };
}

function isActionOption(value: AgentActivityActionOption | null): value is AgentActivityActionOption {
  return value !== null;
}

function parseDecision(value: unknown): AgentActivityActionDecision | null {
  if (!isRecord(value)) {
    return null;
  }
  return {
    decided_at: stringValue(value.decided_at),
    kind: stringValue(value.kind),
    option_id: stringValue(value.option_id),
  };
}

function parseJSON(input: string): unknown {
  if (!input.startsWith("{")) {
    return null;
  }
  try {
    return JSON.parse(input);
  } catch {
    return null;
  }
}

function optionScope(option: AgentActivityActionOption): string {
  return stringValue(option.scope, defaultOptionScope(option.kind));
}

function defaultOptionScope(kind: string): string {
  return kind === "allow_always" ? "agent" : "";
}

function isRecord(value: unknown): value is UnknownRecord {
  return Boolean(value && typeof value === "object" && !Array.isArray(value));
}

function stringValue(...values: unknown[]): string {
  for (const value of values) {
    if (typeof value === "string" && value.trim()) {
      return value.trim();
    }
  }
  return "";
}

function firstDefined(...values: unknown[]): unknown {
  for (const value of values) {
    if (value !== undefined && value !== null) {
      return value;
    }
  }
  return undefined;
}

function numberValue(value: unknown, fallback = 0): number {
  return typeof value === "number" && Number.isFinite(value) ? value : fallback;
}

function parseLegacyFencedCommand(text: string): AgentActivityCommand | null {
  if (!text.startsWith("🔧 ")) {
    return null;
  }

  const firstLineBreak = text.indexOf("\n");
  const firstLine = (firstLineBreak >= 0 ? text.slice(0, firstLineBreak) : text).replace(/^🔧\s*/, "").trim();
  const body = legacyFencedPayload(text.slice(firstLineBreak >= 0 ? firstLineBreak + 1 : text.length));
  const decoded = decodeSummary(body);
  const command = firstNonEmpty(
    summaryValue(decoded, "command", "cmd"),
    summaryValue(decoded, "input", "query", "path", "file", "filename", "pattern", "description", "prompt"),
    summaryText(decoded),
  );
  if (!command) {
    return null;
  }

  const output = firstNonEmpty(
    summaryValue(decoded, "output", "result", "stdout", "stderr", "error"),
    typeof decoded === "string" && decoded !== command ? decoded : "",
  );

  return {
    command,
    kind: AgentActivityKinds.execCommand,
    output,
    signature: commandSignature(command || firstLine),
    title: firstLine.replace(/`/g, "").trim() || "exec_command",
  };
}

function stripToolMarker(text: string): string {
  return text.replace(/^(?:🔧|🛠️?|🔎|📄|📩)\s*/u, "").trim();
}

function splitTrailingStructuredOutput(text: string): { command: string; output?: string } {
  for (let index = 1; index < text.length; index += 1) {
    const char = text[index];
    if ((char !== "{" && char !== "[") || !/\s/.test(text[index - 1] || "")) {
      continue;
    }
    const before = text.slice(0, index).trim();
    const after = text.slice(index).trim();
    if (!before || !after) {
      continue;
    }
    try {
      const parsed = JSON.parse(after);
      return {
        command: before,
        output: JSON.stringify(parsed, null, 2),
      };
    } catch {
      // Keep scanning; a command argument can contain a non-result brace.
    }
  }
  return { command: text.trim() };
}

function isKnownPlainCommand(command: string): boolean {
  const normalized = command.toLowerCase();
  return (
    normalized.startsWith("csgclaw cli ") ||
    normalized.startsWith("csgclaw-cli ") ||
    normalized.startsWith("web search:") ||
    normalized.startsWith("web fetch:")
  );
}

function plainCommandTitle(command: string): string {
  const colon = command.indexOf(":");
  if (colon > 0 && colon <= 32) {
    return command.slice(0, colon).trim();
  }
  const words = command.split(/\s+/).slice(0, 2).join(" ").trim();
  return words || "exec_command";
}

function commandSignature(command: string): string {
  return command.replace(/\s+/g, " ").trim().toLowerCase();
}

function legacyFencedPayload(body: string): string {
  const sections: string[] = [];
  const fencePattern = /```[a-zA-Z0-9_-]*\s*\n?([\s\S]*?)```/g;
  for (const match of body.matchAll(fencePattern)) {
    const value = String(match[1] || "").trim();
    if (value) {
      sections.push(value);
    }
  }
  if (sections.length) {
    return sections.join("\n\n");
  }
  return body.trim();
}

function summaryValue(value: unknown, ...keys: string[]): string {
  const decoded = decodeSummary(value);
  if (!isRecord(decoded)) {
    return "";
  }
  for (const key of keys) {
    const text = summaryText(decoded[key]);
    if (text) {
      return text;
    }
  }
  return "";
}

function summaryText(value: unknown): string {
  const decoded = decodeSummary(value);
  if (typeof decoded === "string") {
    return decoded.trim();
  }
  if (typeof decoded === "number" || typeof decoded === "boolean") {
    return String(decoded);
  }
  if (decoded === null || decoded === undefined) {
    return "";
  }
  try {
    return JSON.stringify(decoded, null, 2);
  } catch {
    return "";
  }
}

function decodeSummary(value: unknown): unknown {
  if (typeof value !== "string") {
    return value;
  }
  const text = value.trim();
  if (!text) {
    return "";
  }
  if (!text.startsWith("{") && !text.startsWith("[")) {
    return text;
  }
  try {
    return JSON.parse(text);
  } catch {
    return text;
  }
}

function firstNonEmpty(...values: unknown[]): string {
  for (const value of values) {
    const text = typeof value === "string" ? value.trim() : summaryText(value);
    if (text) {
      return text;
    }
  }
  return "";
}
