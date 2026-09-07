import type { PropsWithChildren } from "react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { renderHook, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { fetchRoomTasks } from "@/api/tasks";
import { normalizeTaskList } from "@/models/tasks";
import type { IMConversation } from "@/models/conversations";
import { useRoomTasks } from "./useRoomTasks";

vi.mock("@/api/tasks", () => ({ fetchRoomTasks: vi.fn() }));

function wrapper() {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return function Provider({ children }: PropsWithChildren) {
    return <QueryClientProvider client={client}>{children}</QueryClientProvider>;
  };
}

const room: IMConversation = { id: "r1", members: [], messages: [], collaboration_mode: "on_demand" };

describe("useRoomTasks", () => {
  beforeEach(() => vi.resetAllMocks());

  it("loads persisted room tasks and never carries them into a different room", async () => {
    vi.mocked(fetchRoomTasks)
      .mockResolvedValueOnce(normalizeTaskList([{ id: "root", assignment_type: "room", assignment_id: "r1" }]))
      .mockResolvedValueOnce([]);
    const { result, rerender } = renderHook((conversation) => useRoomTasks(conversation), {
      initialProps: room,
      wrapper: wrapper(),
    });
    await waitFor(() => expect(result.current.data?.[0]?.id).toBe("root"));
    rerender({ ...room, id: "r2" });
    expect(result.current.data).toBeUndefined();
    await waitFor(() => expect(result.current.data).toEqual([]));
    expect(fetchRoomTasks).toHaveBeenLastCalledWith("r2", expect.any(AbortSignal));
  });

  it("does not fetch tasks for ordinary rooms or direct conversations", () => {
    const { rerender } = renderHook((conversation) => useRoomTasks(conversation), {
      initialProps: { ...room, collaboration_mode: "" },
      wrapper: wrapper(),
    });
    rerender({ ...room, collaboration_mode: "on_demand", is_direct: true });
    expect(fetchRoomTasks).not.toHaveBeenCalled();
  });
});
