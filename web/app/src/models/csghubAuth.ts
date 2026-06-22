export type CSGHubAuthStatus = {
  authenticated: boolean;
  user_id: string;
  user_uuid: string;
  avatar: string;
  csghub_base_url: string;
  portal_url: string;
  logged_in_at: string;
};

export type CSGHubLoginResponse = {
  login_url: string;
};

export function emptyCSGHubAuthStatus(): CSGHubAuthStatus {
  return {
    authenticated: false,
    user_id: "",
    user_uuid: "",
    avatar: "",
    csghub_base_url: "",
    portal_url: "",
    logged_in_at: "",
  };
}

export function normalizeCSGHubAuthStatus(source: unknown): CSGHubAuthStatus {
  if (!source || typeof source !== "object") {
    return emptyCSGHubAuthStatus();
  }
  const value = source as Record<string, unknown>;
  const authenticated = value.authenticated === true;
  return {
    authenticated,
    user_id: authenticated ? stringFromUnknown(value.user_id) : "",
    user_uuid: authenticated ? stringFromUnknown(value.user_uuid) : "",
    avatar: authenticated ? stringFromUnknown(value.avatar) : "",
    csghub_base_url: authenticated ? normalizeBaseURL(value.csghub_base_url) : "",
    portal_url: authenticated ? stringFromUnknown(value.portal_url) : "",
    logged_in_at: authenticated ? stringFromUnknown(value.logged_in_at) : "",
  };
}

export function normalizeCSGHubLoginResponse(source: unknown): CSGHubLoginResponse {
  if (!source || typeof source !== "object") {
    return { login_url: "" };
  }
  return { login_url: stringFromUnknown((source as Record<string, unknown>).login_url) };
}

export function isCSGHubAuthenticated(status: CSGHubAuthStatus | null | undefined): boolean {
  return Boolean(status?.authenticated);
}

function normalizeBaseURL(source: unknown): string {
  return stringFromUnknown(source).replace(/\/+$/, "");
}

function stringFromUnknown(source: unknown): string {
  return typeof source === "string" ? source.trim() : "";
}
