import { afterEach, describe, expect, it } from "vitest";
import { backendCredentials, resolveBackendPath } from "../../src/api/connection";

afterEach(() => {
  globalThis.CSGCLAW_CONNECTION = undefined;
});
describe("external backend connection", () => {
  it("keeps the existing deployment-relative default", () => {
    expect(resolveBackendPath("api/v1/agents")).toBeUndefined();
    expect(backendCredentials()).toBe("same-origin");
  });
  it("uses configured API and stream subpaths", () => {
    globalThis.CSGCLAW_CONNECTION = {
      api_base: "https://backend.example/tenant/api/v1/",
      event_base: "https://backend.example/tenant/events/",
    };
    expect(resolveBackendPath("/api/v1/agents/a/files?name=a.txt")).toBe(
      "https://backend.example/tenant/api/v1/agents/a/files?name=a.txt",
    );
    expect(resolveBackendPath("api/v1/events", true)).toBe("https://backend.example/tenant/events/events");
    expect(backendCredentials()).toBe("include");
    expect(resolveBackendPath("brand/logo.svg")).toBeUndefined();
  });
  it("rejects tokens and non-HTTP schemes in connection URLs", () => {
    for (const api_base of [
      "javascript:alert(1)",
      "https://user:secret@example.com",
      "https://backend.example/?token=secret",
    ]) {
      globalThis.CSGCLAW_CONNECTION = { api_base };
      expect(() => resolveBackendPath("api/v1/agents")).toThrow();
    }
  });
});
