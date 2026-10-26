import { afterEach, expect, it, vi } from "vitest";
import { updateAgentRequest } from "../../src/api/agents";
import { agentToDraft, agentDraftWithRuntimeFieldsFromAgent } from "../../src/models/agents";
afterEach(() => vi.unstubAllGlobals());
it("ties the precondition to the opened draft rather than a later roster refresh", () => {
  const draft = agentToDraft({ id: "a", name: "Assistant", configuration_revision: "opened-version" });
  const updated = agentDraftWithRuntimeFieldsFromAgent(draft, {
    id: "a",
    configuration_revision: "new-version",
    runtime_name: "codex",
  });
  expect(updated?.configuration_revision).toBe("opened-version");
});
it("sends a draft version as If-Match without adding it to the field mask", async () => {
  const fetcher = vi.fn().mockResolvedValue(new Response(JSON.stringify({ id: "a" }), { status: 200 }));
  vi.stubGlobal("fetch", fetcher);
  await updateAgentRequest("a", { instructions: "new instructions" }, "opened-version");
  const [, options] = fetcher.mock.calls[0] as [string, RequestInit];
  expect(new Headers(options.headers).get("If-Match")).toBe("opened-version");
  expect(JSON.parse(String(options.body)).field_mask).toEqual(["instructions"]);
});
