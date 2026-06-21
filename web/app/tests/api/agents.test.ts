import { afterEach, describe, expect, it, vi } from "vitest";
import type { Mock } from "vitest";
import { batchAddAgentSkillsRequest, deleteAgentSkillRequest } from "@/api/agents";

function mockFetch(): Mock<typeof fetch> {
  const fetchMock = vi.fn<typeof fetch>(async () => new Response("", { status: 200 }));
  vi.stubGlobal("fetch", fetchMock);
  return fetchMock;
}

describe("agents API", () => {
  afterEach(() => {
    vi.unstubAllGlobals();
  });

  it("posts batch add requests to the agent-scoped skills endpoint", async () => {
    const fetchMock = mockFetch();

    await batchAddAgentSkillsRequest("u-manager", ["alpha", "beta"]);

    expect(fetchMock).toHaveBeenCalledWith(
      "api/v1/agents/u-manager/skills:batchAdd",
      expect.objectContaining({
        body: JSON.stringify({ names: ["alpha", "beta"] }),
        method: "POST",
      }),
    );
  });

  it("deletes skills from the agent-scoped skills endpoint", async () => {
    const fetchMock = mockFetch();

    await deleteAgentSkillRequest("u-manager", "alpha");

    expect(fetchMock).toHaveBeenCalledWith(
      "api/v1/agents/u-manager/skills/alpha",
      expect.objectContaining({ method: "DELETE" }),
    );
  });
});
