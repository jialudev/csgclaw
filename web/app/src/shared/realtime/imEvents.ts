// @ts-nocheck
import { IM_EVENTS_ENDPOINT } from "@/bootstrap/constants";

const sharedWorkerURL = new URL("./sseSharedWorker.ts", import.meta.url);

export function safeParseEventData(raw) {
  try {
    return JSON.parse(raw);
  } catch (error) {
    console.warn("Failed to parse IM event payload", error);
    return null;
  }
}

export function subscribeIMEvents(onEvent) {
  if (typeof window.SharedWorker === "function") {
    try {
      const worker = new SharedWorker(sharedWorkerURL, { type: "module" });
      const port = worker.port;
      const handleMessage = ({ data }) => {
        if (!data || data.type !== "message") {
          return;
        }
        const payload = safeParseEventData(data.data);
        if (payload) {
          onEvent(payload);
        }
      };

      port.addEventListener("message", handleMessage);
      port.start();
      port.postMessage({ type: "subscribe", endpoint: IM_EVENTS_ENDPOINT });

      return () => {
        port.postMessage({ type: "close" });
        port.removeEventListener("message", handleMessage);
        port.close();
      };
    } catch (error) {
      console.warn("SharedWorker SSE unavailable, falling back to EventSource", error);
    }
  }

  const source = new EventSource(IM_EVENTS_ENDPOINT);
  source.onmessage = (event) => {
    const payload = safeParseEventData(event.data);
    if (payload) {
      onEvent(payload);
    }
  };

  return () => source.close();
}
