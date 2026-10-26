import type { DesktopBootstrapMessage } from "./contract";

// Secrets travel only through the private bootstrap pipe, never process args,
// renderer globals, ready messages, or backend logs.
export function externalBackendFromEnvironment(env: NodeJS.ProcessEnv): Pick<DesktopBootstrapMessage, "backend_api_base" | "backend_token"> {
  const base = env.CSGCLAW_DESKTOP_BACKEND_API_BASE?.trim();
  const token = env.CSGCLAW_DESKTOP_BACKEND_TOKEN?.trim();
  if (!base && !token) return {};
  if (!base || !token || /[\r\n]/.test(token)) throw new Error("External backend requires an API base and a service credential");
  let url: URL;
  try { url = new URL(base); } catch { throw new Error("External backend API base is invalid"); }
  if (!["http:", "https:"].includes(url.protocol) || url.username || url.password || url.search || url.hash || !url.pathname.endsWith("/")) {
    throw new Error("External backend API base must be an HTTP(S) API prefix ending with / without credentials or query");
  }
  return { backend_api_base: url.href, backend_token: token };
}
