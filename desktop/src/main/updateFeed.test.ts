import assert from "node:assert/strict";
import test from "node:test";
import { desktopUpdateFeedURL, normalizeHTTPSURL } from "./updateFeed";

test("builds isolated release and beta OSS update feeds", () => {
  const base = "https://downloads.example/csgclaw/channels";
  assert.equal(
    desktopUpdateFeedURL({
      channel: "release",
      platform: "darwin",
      arch: "arm64",
      channelsBaseURL: base,
    }),
    "https://downloads.example/csgclaw/channels/release/updates/darwin/arm64",
  );
  assert.equal(
    desktopUpdateFeedURL({
      channel: "beta",
      platform: "win32",
      arch: "x64",
      channelsBaseURL: base,
    }),
    "https://downloads.example/csgclaw/channels/beta/updates/win32/x64",
  );
});

test("allows a direct feed override and rejects unsafe URLs", () => {
  assert.equal(
    desktopUpdateFeedURL({
      channel: "beta",
      platform: "darwin",
      arch: "arm64",
      directURL: "https://updates.example/custom/",
    }),
    "https://updates.example/custom",
  );
  assert.equal(normalizeHTTPSURL("http://updates.example/feed"), "");
  assert.equal(normalizeHTTPSURL("https://user:secret@updates.example/feed"), "");
});
