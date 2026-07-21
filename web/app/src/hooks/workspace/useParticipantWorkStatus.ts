import { useCallback, useEffect, useMemo, useReducer, useRef } from "react";
import type { ConversationWorkingParticipant } from "@/components/business/ConversationPane";
import type { AgentLike } from "@/models/agents";
import {
  agentMatchesUser,
  buildUsersById,
  localIdentitiesMatch,
  resolveUserByLocalIdentity,
} from "@/models/conversations";
import type { IMServerEvent, IMUser } from "@/models/conversations";
import {
  activeParticipantWorkForRoom,
  createParticipantWorkState,
  nextParticipantWorkDeadline,
  participantWorkReducer,
} from "./participantWorkState";

type UseParticipantWorkStatusArgs = {
  agents: readonly AgentLike[];
  users: readonly IMUser[];
};

export type ParticipantWorkStatus = {
  handleRealtimeEvent: (event: IMServerEvent) => void;
  hasObservedWorkLease: (participantID: string | null | undefined) => boolean;
  workingParticipantsForRoom: (roomID: string | null | undefined) => ConversationWorkingParticipant[];
};

export function useParticipantWorkStatus({ agents, users }: UseParticipantWorkStatusArgs): ParticipantWorkStatus {
  const [state, dispatch] = useReducer(participantWorkReducer, undefined, createParticipantWorkState);
  const observedLeaseIdentitiesRef = useRef(new Set<string>());
  const usersById = useMemo(() => buildUsersById(users), [users]);

  useEffect(() => {
    const deadline = nextParticipantWorkDeadline(state);
    if (deadline === null) {
      return;
    }
    const timer = window.setTimeout(
      () => {
        dispatch({ now: Date.now(), type: "clock" });
      },
      Math.max(0, deadline - Date.now()),
    );
    return () => window.clearTimeout(timer);
  }, [state]);

  const handleRealtimeEvent = useCallback((event: IMServerEvent) => {
    if (event?.type === "participant.work.updated" && event.work) {
      [event.work.participant_id, event.work.user_id].forEach((value) => {
        const id = String(value || "").trim();
        if (id) {
          observedLeaseIdentitiesRef.current.add(id);
        }
      });
      dispatch({ now: Date.now(), type: "workEvent", work: event.work });
      return;
    }
    if ((event?.type === "participant.updated" || event?.type === "participant.deleted") && event.participant) {
      const participantIDs = [
        event.participant.id,
        event.participant.channel_user_ref,
        event.participant.user_id,
      ].filter((value): value is string => Boolean(value));
      participantIDs.forEach((id) => observedLeaseIdentitiesRef.current.delete(id));
      dispatch({ participantIDs, type: "participantLifecycle" });
      return;
    }
    if (event?.type === "room.deleted") {
      dispatch({ roomID: String(event.room_id || event.room?.id || ""), type: "roomLifecycle" });
      return;
    }
    if (event?.type === "room.members_removed" && event.room?.id && Array.isArray(event.room.members)) {
      dispatch({
        memberParticipantIDs: event.room.members,
        roomID: event.room.id,
        type: "roomLifecycle",
      });
    }
  }, []);

  const hasObservedWorkLease = useCallback((participantID: string | null | undefined): boolean => {
    const id = String(participantID || "").trim();
    return Boolean(id && observedLeaseIdentitiesRef.current.has(id));
  }, []);

  const workingParticipantsForRoom = useCallback(
    (roomIDValue: string | null | undefined): ConversationWorkingParticipant[] => {
      const roomID = String(roomIDValue || "").trim();
      if (!roomID) {
        return [];
      }
      const byID = new Map<string, ConversationWorkingParticipant>();
      Object.entries(activeParticipantWorkForRoom(state, roomID)).forEach(([participantID, byLease]) => {
        const lease = Object.values(byLease)[0];
        if (!lease) {
          return;
        }
        const user =
          resolveUserByLocalIdentity(lease.user_id, usersById) ?? resolveUserByLocalIdentity(participantID, usersById);
        const agent = agents.find(
          (candidate) =>
            localIdentitiesMatch(candidate.id, participantID) ||
            localIdentitiesMatch(candidate.user_id, lease.user_id) ||
            (user ? agentMatchesUser(candidate, user) : false),
        );
        const id = String(user?.id || lease.user_id).trim();
        const name = String(agent?.name || user?.name || id).trim();
        if (id && name) {
          byID.set(id, { id, name, requestID: lease.request_id });
        }
      });
      return Array.from(byID.values()).sort((left, right) => left.name.localeCompare(right.name));
    },
    [agents, state, usersById],
  );

  return { handleRealtimeEvent, hasObservedWorkLease, workingParticipantsForRoom };
}
