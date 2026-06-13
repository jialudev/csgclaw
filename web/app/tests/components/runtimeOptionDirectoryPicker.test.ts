import { afterEach, describe, expect, it, vi } from "vitest";
import { pickHostDirectoryRequest } from "@/api/agents";
import { pickLocalDirectoryPath } from "@/components/business/ProfileControls/runtimeOptionDirectoryPicker";

vi.mock("@/api/agents", () => ({
  pickHostDirectoryRequest: vi.fn(),
}));

describe("runtimeOptionDirectoryPicker", () => {
  const pickHostDirectoryRequestMock = vi.mocked(pickHostDirectoryRequest);

  afterEach(() => {
    pickHostDirectoryRequestMock.mockReset();
    delete (window as Window & { showDirectoryPicker?: unknown }).showDirectoryPicker;
  });

  it("returns the path selected by the host picker", async () => {
    pickHostDirectoryRequestMock.mockResolvedValue({ status: "selected", path: "/tmp/host-project" });

    await expect(pickLocalDirectoryPath()).resolves.toBe("/tmp/host-project");
  });

  it("returns null when the host picker is canceled", async () => {
    const showDirectoryPicker = vi.fn();
    (window as Window & { showDirectoryPicker?: unknown }).showDirectoryPicker = showDirectoryPicker;
    pickHostDirectoryRequestMock.mockResolvedValue({ status: "canceled" });

    await expect(pickLocalDirectoryPath()).resolves.toBeNull();
    expect(showDirectoryPicker).not.toHaveBeenCalled();
  });

  it("falls back to the browser directory picker when the host picker is unavailable", async () => {
    pickHostDirectoryRequestMock.mockResolvedValue({ status: "unavailable" });
    const showDirectoryPicker = vi.fn().mockResolvedValue({ path: "/tmp/browser-project" });
    (window as Window & { showDirectoryPicker?: unknown }).showDirectoryPicker = showDirectoryPicker;

    await expect(pickLocalDirectoryPath()).resolves.toBe("/tmp/browser-project");
    expect(showDirectoryPicker).toHaveBeenCalledTimes(1);
  });

  it("does not fall back to the browser picker when the host picker rejects", async () => {
    const showDirectoryPicker = vi.fn().mockResolvedValue({ path: "/tmp/browser-project" });
    (window as Window & { showDirectoryPicker?: unknown }).showDirectoryPicker = showDirectoryPicker;
    pickHostDirectoryRequestMock.mockRejectedValue(new Error("host picker failed"));

    await expect(pickLocalDirectoryPath()).rejects.toThrow("host picker failed");
    expect(showDirectoryPicker).not.toHaveBeenCalled();
  });
});
