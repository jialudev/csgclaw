import { resolveBackendPath, backendCredentials } from "@/api/connection";
import { ApiEndpoints, IM_EVENTS_SHARED_WORKER_PATH } from "@/shared/constants/api";
import type { IMServerEvent } from "@/models/conversations";

let connected = true;
const connectionListeners = new Set<() => void>();
export const imEventsConnected = () => connected;
export function subscribeIMConnection(listener: () => void) {
  connectionListeners.add(listener);
  return () => {
    connectionListeners.delete(listener);
  };
}
function updateConnection(value: boolean) {
  if (value !== connected) {
    connected = value;
    connectionListeners.forEach((listener) => listener());
  }
}

const sharedWorkerURL = import.meta.env.DEV ? "/src/shared/realtime/sseSharedWorker.ts" : IM_EVENTS_SHARED_WORKER_PATH;

function createSharedWorker(endpoint: string) {
  if (import.meta.env.DEV) {
    return new SharedWorker(sharedWorkerURL, { type: "module", name: endpoint });
  }
  return new SharedWorker(sharedWorkerURL, { name: endpoint });
}

type SharedWorkerEnvelope = {
  data?: string;
  type?: string;
};

export function safeParseEventData(raw: string): IMServerEvent | null {
  try {
    return JSON.parse(raw) as IMServerEvent;
  } catch (error) {
    console.warn("Failed to parse IM event payload", error);
    return null;
  }
}

export function resolveIMEventsEndpoint(baseURI: string): string {
  return resolveBackendPath(ApiEndpoints.imEvents, true) ?? new URL(ApiEndpoints.imEvents, baseURI).toString();
}

export function subscribeIMEvents(onEvent: (payload: IMServerEvent) => void): () => void {
  const endpoint = resolveIMEventsEndpoint(document.baseURI);

  if (typeof window.SharedWorker === "function") {
    try {
      const worker = createSharedWorker(endpoint);
      const port = worker.port;
      const handleMessage = ({ data }: MessageEvent<SharedWorkerEnvelope>) => {
        if (data?.type === "open") {
          updateConnection(true);
          onEvent({ type: "connection.open" });
          return;
        }
        if (data?.type === "error") {
          updateConnection(false);
          return;
        }
        if (!data || data.type !== "message") {
          return;
        }
        const payload = safeParseEventData(String(data.data ?? ""));
        if (payload) {
          onEvent(payload);
        }
      };

      port.addEventListener("message", handleMessage);
      port.start();
      port.postMessage({ type: "subscribe", endpoint });

      return () => {
        port.postMessage({ type: "close" });
        port.removeEventListener("message", handleMessage);
        port.close();
      };
    } catch (error) {
      console.warn("SharedWorker SSE unavailable, falling back to EventSource", error);
    }
  }

  const source = new EventSource(endpoint, { withCredentials: backendCredentials() === "include" });
  source.onopen = () => {
    updateConnection(true);
    onEvent({ type: "connection.open" });
  };
  source.onerror = () => updateConnection(false);
  source.onmessage = (event) => {
    const payload = safeParseEventData(event.data);
    if (payload) {
      onEvent(payload);
    }
  };

  return () => source.close();
}
