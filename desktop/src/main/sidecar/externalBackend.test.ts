import assert from "node:assert/strict";
import test from "node:test";
import { externalBackendFromEnvironment } from "./externalBackend";

test("external backend is opt-in and sends credentials in the bootstrap fields", () => {
  assert.deepEqual(externalBackendFromEnvironment({}), {});
  assert.deepEqual(externalBackendFromEnvironment({ CSGCLAW_DESKTOP_BACKEND_API_BASE: "https://service.example/tenant/api/v1/", CSGCLAW_DESKTOP_BACKEND_TOKEN: "test-service-token" }), {
    backend_api_base: "https://service.example/tenant/api/v1/", backend_token: "test-service-token",
  });
});
test("external backend errors do not echo tokens or invalid URLs", () => {
  for (const base of ["file:///etc/", "https://user:password@example.com/api/v1/", "https://example.com/api/v1/?token=secret", "https://example.com/api/v1"]) {
    assert.throws(() => externalBackendFromEnvironment({ CSGCLAW_DESKTOP_BACKEND_API_BASE: base, CSGCLAW_DESKTOP_BACKEND_TOKEN: "secret" }), (error: unknown) => error instanceof Error && !error.message.includes("secret") && !error.message.includes("password"));
  }
});
