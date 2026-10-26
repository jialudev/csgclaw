import { afterEach, expect, it } from "vitest";
import { backendStorageKey } from "../../src/shared/storage/backendScope";
afterEach(() => {
  globalThis.CSGCLAW_CONNECTION = undefined;
});
it("preserves local keys and isolates hosted instances and users", () => {
  expect(backendStorageKey("draft")).toBe("draft");
  globalThis.CSGCLAW_CONNECTION = { api_base: "https://example.com/a/api/v1/", storage_scope: "backend-a:user-1" };
  const first = backendStorageKey("draft");
  globalThis.CSGCLAW_CONNECTION = { api_base: "https://example.com/b/api/v1/", storage_scope: "backend-b:user-1" };
  expect(backendStorageKey("draft")).not.toBe(first);
});
