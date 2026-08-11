import { useCallback, useEffect, useRef, useState } from "react";
import type { AgentLike } from "@/models/agents";
import {
  formatMessagePreviewText,
  isEventMessage,
  isToolCallMessage,
  localIdentitiesMatch,
  resolveAgentForUser,
  resolveUserByLocalIdentity,
} from "@/models/conversations";
import type { IMConversation, IMMessage, IMServerEvent, TranslateFn, UsersById } from "@/models/conversations";
import {
  isCompletedAgentTurnEvent,
  shouldShowTurnNotification,
  TurnNotificationModes,
} from "@/models/turnNotifications";
import type { TurnNotificationMode } from "@/models/turnNotifications";

export type SystemNotificationPermission = NotificationPermission | "unsupported";

type UseAgentTurnNotificationsArgs = {
  agents: readonly AgentLike[];
  currentUserID: string;
  mode: TurnNotificationMode;
  onSelectConversation: (roomID: string) => void;
  rooms: readonly IMConversation[];
  t: TranslateFn;
  usersById: UsersById;
};

type AgentTurnNotificationController = {
  handleRealtimeEvent: (event: IMServerEvent) => void;
  permission: SystemNotificationPermission;
  requestPermission: () => Promise<SystemNotificationPermission>;
};

const NOTIFICATION_PREVIEW_MAX_LENGTH = 160;

export function readSystemNotificationPermission(): SystemNotificationPermission {
  if (typeof window === "undefined" || typeof window.Notification !== "function") {
    return "unsupported";
  }
  return window.Notification.permission;
}

export function useAgentTurnNotifications(args: UseAgentTurnNotificationsArgs): AgentTurnNotificationController {
  const refs = useRef(args);
  const notifiedEventKeysRef = useRef(new Set<string>());
  const workAwareAgentKeysRef = useRef(new Set<string>());
  const [permission, setPermission] = useState<SystemNotificationPermission>(readSystemNotificationPermission);

  useEffect(() => {
    refs.current = args;
  }, [args]);

  useEffect(() => {
    const refreshPermission = () => setPermission(readSystemNotificationPermission());
    window.addEventListener("focus", refreshPermission);
    document.addEventListener("visibilitychange", refreshPermission);
    return () => {
      window.removeEventListener("focus", refreshPermission);
      document.removeEventListener("visibilitychange", refreshPermission);
    };
  }, []);

  const requestPermission = useCallback(async (): Promise<SystemNotificationPermission> => {
    if (typeof window.Notification !== "function") {
      setPermission("unsupported");
      return "unsupported";
    }
    try {
      const next = await window.Notification.requestPermission();
      setPermission(next);
      return next;
    } catch {
      const next = readSystemNotificationPermission();
      setPermission(next);
      return next;
    }
  }, []);

  const handleRealtimeEvent = useCallback((event: IMServerEvent) => {
    const current = refs.current;

    if (event.type === "participant.work.updated" && event.work) {
      const agent = resolveEventAgent(current.agents, current.usersById, [
        event.work.user_id,
        event.work.participant_id,
      ]);
      if (!agent) {
        return;
      }
      const key = agentNotificationKey(agent);
      workAwareAgentKeysRef.current.add(key);
      if (!isCompletedAgentTurnEvent(event)) {
        return;
      }
      showAgentTurnNotification(current, {
        agent,
        eventKey: `work:${event.work.registry_epoch}:${event.work.lease_id}`,
        participantIdentities: [event.work.user_id, event.work.participant_id],
        roomID: event.work.room_id || String(event.room_id || ""),
      });
      return;
    }

    if (event.type !== "message.created" || !event.message || isEventMessage(event.message)) {
      return;
    }
    if (isToolCallMessage(event.message) || !formatMessagePreviewText(event.message.content)) {
      return;
    }
    const senderID = String(event.message.sender_id || "").trim();
    const agent = resolveEventAgent(current.agents, current.usersById, [senderID]);
    if (!agent || workAwareAgentKeysRef.current.has(agentNotificationKey(agent))) {
      return;
    }
    showAgentTurnNotification(current, {
      agent,
      eventKey: `message:${String(event.message.id || event.message.created_at || "")}:${senderID}`,
      message: event.message,
      participantIdentities: [senderID],
      roomID: String(event.room_id || event.room?.id || ""),
    });
  }, []);

  function showAgentTurnNotification(
    current: UseAgentTurnNotificationsArgs,
    input: {
      agent: AgentLike;
      eventKey: string;
      message?: IMMessage;
      participantIdentities: string[];
      roomID: string;
    },
  ): void {
    if (
      current.mode === TurnNotificationModes.off ||
      readSystemNotificationPermission() !== "granted" ||
      input.participantIdentities.some((id) => localIdentitiesMatch(id, current.currentUserID)) ||
      notifiedEventKeysRef.current.has(input.eventKey)
    ) {
      return;
    }
    if (
      !shouldShowTurnNotification(current.mode, {
        documentVisible: document.visibilityState === "visible",
        windowFocused: typeof document.hasFocus !== "function" || document.hasFocus(),
      })
    ) {
      return;
    }

    const roomID = input.roomID.trim();
    const room = current.rooms.find((candidate) => candidate.id === roomID);
    const agentName = String(input.agent.name || input.agent.user_name || input.agent.id || "Agent").trim();
    const message = input.message ?? latestVisibleAgentMessage(room, input.participantIdentities);
    const preview = truncateNotificationPreview(formatMessagePreviewText(message?.content));
    const roomTitle = String(room?.title || "").trim();
    const body = preview
      ? roomTitle
        ? current.t("turnNotificationBody", { message: preview, room: roomTitle })
        : preview
      : roomTitle
        ? current.t("turnNotificationRoomBody", { room: roomTitle })
        : current.t("turnNotificationDefaultBody");

    try {
      const notification = new window.Notification(current.t("turnNotificationTitle", { agent: agentName }), {
        body,
        icon: "favicon.ico",
        tag: `csgclaw-${input.eventKey}`,
      });
      notifiedEventKeysRef.current.add(input.eventKey);
      notification.onclick = () => {
        notification.close();
        window.focus();
        if (roomID) {
          refs.current.onSelectConversation(roomID);
        }
      };
    } catch {
      setPermission(readSystemNotificationPermission());
    }
  }

  return { handleRealtimeEvent, permission, requestPermission };
}

function resolveEventAgent(
  agents: readonly AgentLike[],
  usersById: UsersById,
  identities: readonly (string | null | undefined)[],
): AgentLike | null {
  const ids = identities.map((value) => String(value || "").trim()).filter(Boolean);
  const users = ids.map((id) => resolveUserByLocalIdentity(id, usersById)).filter((user) => user !== undefined);
  return resolveAgentForUser(agents, users[0] ?? null, [...users.slice(1), ...ids.map((id) => ({ id }))]);
}

function agentNotificationKey(agent: AgentLike): string {
  return String(agent.id || agent.user_id || agent.name || "").trim();
}

function latestVisibleAgentMessage(
  room: IMConversation | null | undefined,
  participantIdentities: readonly string[],
): IMMessage | undefined {
  return [...(room?.messages || [])].reverse().find((message) => {
    if (
      !participantIdentities.some((identity) => localIdentitiesMatch(message.sender_id, identity)) ||
      isEventMessage(message) ||
      isToolCallMessage(message)
    ) {
      return false;
    }
    return Boolean(formatMessagePreviewText(message.content));
  });
}

function truncateNotificationPreview(preview: string): string {
  if (preview.length <= NOTIFICATION_PREVIEW_MAX_LENGTH) {
    return preview;
  }
  return `${preview.slice(0, NOTIFICATION_PREVIEW_MAX_LENGTH - 1).trimEnd()}…`;
}
