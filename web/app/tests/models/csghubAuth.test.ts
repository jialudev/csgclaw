import { normalizeCSGHubAuthStatus, normalizeCSGHubLoginResponse } from "@/models/csghubAuth";

describe("csghubAuth model", () => {
  it("normalizes authenticated status without preserving secrets", () => {
    const got = normalizeCSGHubAuthStatus({
      authenticated: true,
      user_id: " alice ",
      user_uuid: " user-1 ",
      avatar: " https://example.test/avatar.png ",
      csghub_base_url: " https://hub.example.test/ ",
      portal_url: " https://hub.example.test/portal ",
      logged_in_at: "2026-06-22T09:00:00Z",
      access_token: "secret-token",
      ai_gateway_builtin_api_key: "secret-gateway-key",
    });

    expect(got).toEqual({
      authenticated: true,
      user_id: "alice",
      user_uuid: "user-1",
      avatar: "https://example.test/avatar.png",
      csghub_base_url: "https://hub.example.test",
      portal_url: "https://hub.example.test/portal",
      logged_in_at: "2026-06-22T09:00:00Z",
    });
    expect(Object.prototype.hasOwnProperty.call(got, "access_token")).toBe(false);
    expect(Object.prototype.hasOwnProperty.call(got, "ai_gateway_builtin_api_key")).toBe(false);
  });

  it("clears user fields when unauthenticated", () => {
    const got = normalizeCSGHubAuthStatus({
      authenticated: false,
      user_id: "alice",
      csghub_base_url: "https://hub.example.test",
    });

    expect(got.authenticated).toBe(false);
    expect(got.user_id).toBe("");
    expect(got.csghub_base_url).toBe("");
  });

  it("normalizes login response", () => {
    expect(normalizeCSGHubLoginResponse({ login_url: " https://iam.example.test/login " })).toEqual({
      login_url: "https://iam.example.test/login",
    });
    expect(normalizeCSGHubLoginResponse(null)).toEqual({ login_url: "" });
  });
});
