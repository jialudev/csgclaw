import { act, renderHook } from "@testing-library/react";
import { vi } from "vitest";
import { useWorkspaceRealtime } from "@/hooks/workspace/useWorkspaceRealtime";
import { buildUsersById, type IMData, type IMServerEvent } from "@/models/conversations";
import type { AgentLike } from "@/models/agents";

const subscribeIMEventsMock = vi.fn();

vi.mock("@/shared/realtime/imEvents", () => ({
  subscribeIMEvents: (handler: (payload: IMServerEvent) => void) => subscribeIMEventsMock(handler),
}));

const bootstrapData: IMData = {
  current_user_id: "u-admin",
  rooms: [],
  users: [
    { id: "u-admin", name: "admin" },
    { id: "u-worker", name: "worker" },
  ],
};

function renderWorkspaceRealtime(
  options: {
    agents?: AgentLike[];
    onRefreshAgentState?: (agentID: string) => Promise<AgentLike | null>;
    refreshWorkspaceAgents?: (options?: { silent?: boolean }) => Promise<AgentLike[]>;
    refreshWorkspaceBootstrap?: () => Promise<IMData | null>;
    setBootstrapData?: (value: IMData | null | ((current: IMData | null) => IMData | null)) => void;
  } = {},
) {
  const agents: AgentLike[] = options.agents ?? [];
  const setBootstrapData =
    options.setBootstrapData ??
    vi.fn((value: IMData | null | ((current: IMData | null) => IMData | null)) => {
      if (typeof value === "function") {
        value(bootstrapData);
      }
    });

  return renderHook(() =>
    useWorkspaceRealtime({
      agents,
      onConversationEvent: vi.fn(),
      onFloatingConversationEvent: vi.fn(),
      onRefreshAgentState: options.onRefreshAgentState ?? vi.fn(async () => null),
      onUpgradeStatusChange: vi.fn(),
      refreshWorkspaceAgents: options.refreshWorkspaceAgents ?? vi.fn(async () => []),
      refreshWorkspaceBootstrap: options.refreshWorkspaceBootstrap ?? vi.fn(async () => null),
      setBootstrapData,
      usersById: buildUsersById(bootstrapData.users),
    }),
  );
}

describe("useWorkspaceRealtime", () => {
  beforeEach(() => {
    vi.useFakeTimers();
    subscribeIMEventsMock.mockReset();
    subscribeIMEventsMock.mockReturnValue(() => {});
  });

  afterEach(() => {
    vi.useRealTimers();
  });

  it("subscribes once and debounces participant roster refreshes", async () => {
    let eventHandler: ((payload: IMServerEvent) => void) | null = null;
    subscribeIMEventsMock.mockImplementation((handler: (payload: IMServerEvent) => void) => {
      eventHandler = handler;
      return () => {};
    });
    const refreshWorkspaceAgents = vi.fn(async () => []);
    const refreshWorkspaceBootstrap = vi.fn(async () => null);
    const setBootstrapData = vi.fn((value: IMData | null | ((current: IMData | null) => IMData | null)) => {
      if (typeof value === "function") {
        value(bootstrapData);
      }
    });

    renderWorkspaceRealtime({ refreshWorkspaceAgents, refreshWorkspaceBootstrap, setBootstrapData });

    expect(subscribeIMEventsMock).toHaveBeenCalledTimes(1);

    act(() => {
      eventHandler?.({
        participant: { id: "pt-worker", type: "agent" },
        type: "participant.created",
      });
    });

    expect(setBootstrapData).toHaveBeenCalledTimes(1);
    expect(refreshWorkspaceAgents).not.toHaveBeenCalled();

    act(() => {
      vi.advanceTimersByTime(100);
    });

    expect(refreshWorkspaceBootstrap).toHaveBeenCalledTimes(1);
    expect(refreshWorkspaceAgents).toHaveBeenCalledWith({ silent: true });
  });

  it("refreshes non-running agent state when that agent sends a message", () => {
    let eventHandler: ((payload: IMServerEvent) => void) | null = null;
    subscribeIMEventsMock.mockImplementation((handler: (payload: IMServerEvent) => void) => {
      eventHandler = handler;
      return () => {};
    });
    const onRefreshAgentState = vi.fn(async () => null);

    renderWorkspaceRealtime({
      agents: [
        {
          id: "u-worker",
          name: "worker",
          role: "worker",
          runtime_kind: "picoclaw_sandbox",
          status: "unknown",
        },
      ],
      onRefreshAgentState,
    });

    act(() => {
      eventHandler?.({
        message: {
          content: "reply",
          sender_id: "u-worker",
        },
        room_id: "room-1",
        type: "message.created",
      });
    });

    expect(onRefreshAgentState).toHaveBeenCalledWith("u-worker");
  });
});
