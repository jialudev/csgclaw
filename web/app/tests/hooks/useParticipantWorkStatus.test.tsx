import { act, renderHook } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import type { AgentLike } from "@/models/agents";
import type { IMServerEvent } from "@/models/conversations";
import { useParticipantWorkStatus } from "@/hooks/workspace/useParticipantWorkStatus";

const startedAt = Date.parse("2026-07-14T12:00:00Z");

function workEvent(overrides: Partial<NonNullable<IMServerEvent["work"]>> = {}): IMServerEvent {
  return {
    type: "participant.work.updated",
    work: {
      expires_at: "2026-07-14T12:00:15Z",
      kind: "agent_turn",
      lease_id: "lease-1",
      participant_id: "pt-worker",
      reason: "started",
      registry_epoch: "epoch-1",
      request_id: "message-1",
      revision: 1,
      room_id: "room-1",
      state: "working",
      user_id: "user-worker",
      ...overrides,
    },
  };
}

afterEach(() => {
  vi.useRealTimers();
});

describe("useParticipantWorkStatus", () => {
  it("projects active lease state and expires it through one deadline timer", () => {
    vi.useFakeTimers();
    vi.setSystemTime(startedAt);
    const { result } = renderHook(() =>
      useParticipantWorkStatus({
        agents: [{ id: "agent-worker", name: "Agent Worker", user_id: "user-worker" } as AgentLike],
        users: [{ id: "user-worker", name: "Roster Worker" }],
      }),
    );

    expect(result.current.workingParticipantsForRoom("room-1")).toEqual([]);
    expect(result.current.hasObservedWorkLease("user-worker")).toBe(false);

    act(() => result.current.handleRealtimeEvent(workEvent()));
    expect(result.current.workingParticipantsForRoom("room-1")).toEqual([
      { id: "user-worker", name: "Agent Worker", requestID: "message-1" },
    ]);
    expect(result.current.hasObservedWorkLease("pt-worker")).toBe(true);
    expect(result.current.hasObservedWorkLease("user-worker")).toBe(true);
    expect(vi.getTimerCount()).toBe(1);

    act(() => vi.advanceTimersByTime(15_000));
    expect(result.current.workingParticipantsForRoom("room-1")).toEqual([]);
    expect(vi.getTimerCount()).toBe(1);

    act(() =>
      result.current.handleRealtimeEvent({
        participant: { id: "pt-worker" },
        type: "participant.updated",
      }),
    );
    expect(result.current.workingParticipantsForRoom("room-1")).toEqual([]);
    expect(result.current.hasObservedWorkLease("pt-worker")).toBe(false);
  });

  it("does not display idle work updates", () => {
    vi.useFakeTimers();
    vi.setSystemTime(startedAt);
    const { result } = renderHook(() =>
      useParticipantWorkStatus({
        agents: [],
        users: [{ id: "user-worker", name: "Worker" }],
      }),
    );

    act(() => result.current.handleRealtimeEvent(workEvent({ reason: "released", revision: 2, state: "idle" })));
    expect(result.current.workingParticipantsForRoom("room-1")).toEqual([]);
  });
});
