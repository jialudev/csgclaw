// The UI host supplies connection metadata before loading the app. Credentials
// remain in an HttpOnly pairing cookie or in a same-origin reverse proxy.
export type BackendConnection = {
  api_base: string;
  storage_scope?: string;
  event_base?: string;
};

declare global {
  var CSGCLAW_CONNECTION: BackendConnection | undefined;
}

function validatedBase(value: string): string {
  const base = value.trim();
  const url = new URL(base, document.baseURI);
  if (!["http:", "https:"].includes(url.protocol) || url.username || url.password || url.search || url.hash) {
    throw new Error("Backend connection must be an HTTP(S) URL without credentials, query, or fragment");
  }
  return url.href.replace(/\/?$/, "/");
}

export function resolveBackendPath(path: string, events = false): string | undefined {
  const connection = globalThis.CSGCLAW_CONNECTION;
  if (!connection || !/^\/?api\/v1\//.test(path)) return undefined;
  const base = validatedBase(events ? connection.event_base || connection.api_base : connection.api_base);
  const target = new URL(path.replace(/^\/?api\/v1\//, ""), base);
  const configured = new URL(base);
  if (target.origin !== configured.origin || !target.pathname.startsWith(configured.pathname)) {
    throw new Error("API path escapes the configured backend prefix");
  }
  return target.href;
}

export function backendCredentials(): RequestCredentials {
  return globalThis.CSGCLAW_CONNECTION ? "include" : "same-origin";
}

// Called by a trusted UI host after the user supplies a service credential.
// The token is sent once and is never written to storage or a URL.
export async function pairBackend(token: string): Promise<void> {
  const endpoint = resolveBackendPath("api/v1/control/pair");
  if (!endpoint) throw new Error("Configure a backend connection before pairing");
  const response = await fetch(endpoint, {
    method: "POST",
    credentials: "include",
    headers: { Authorization: `Bearer ${token}` },
  });
  if (!response.ok) throw new Error(`Backend pairing failed (${response.status})`);
}

export async function unpairBackend(): Promise<void> {
  const endpoint = resolveBackendPath("api/v1/control/logout");
  if (!endpoint) return;
  const response = await fetch(endpoint, { method: "POST", credentials: "include" });
  if (!response.ok) throw new Error(`Backend logout failed (${response.status})`);
}
