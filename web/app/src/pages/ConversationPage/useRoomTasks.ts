import { useQuery } from "@tanstack/react-query";
import { fetchRoomTasks } from "@/api/tasks";
import type { IMConversation } from "@/models/conversations";

export function useRoomTasks(conversation: IMConversation) {
  return useQuery({
    queryKey: ["workspace", "room-tasks", conversation.id],
    queryFn: ({ signal }) => fetchRoomTasks(conversation.id, signal),
    enabled: conversation.collaboration_mode === "on_demand" && !conversation.is_direct,
    // Poll while the room is visible, even before a root exists. Task persistence
    // and IM projection can succeed independently; messages are not the task store.
    refetchInterval: 3000,
    refetchIntervalInBackground: false,
    refetchOnWindowFocus: true,
  });
}
