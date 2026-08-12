import type { DesktopUpdateChannel } from "../shared/desktopBridge.types";

export const DEFAULT_UPDATE_CHANNELS_BASE_URL =
  "https://opencsg-public-resource.oss-cn-beijing.aliyuncs.com/csgclaw-desktop/channels";

export type DesktopDownloadsManifest = {
  channel: DesktopUpdateChannel;
  latest: string;
};

export function desktopUpdateFeedOptions(
  url: string,
  platform: NodeJS.Platform,
): { url: string; serverType?: "json" } {
  return platform === "darwin" ? { url, serverType: "json" } : { url };
}

export function desktopUpdateFeedURL({
  channel,
  platform,
  arch,
  directURL = "",
  channelsBaseURL = DEFAULT_UPDATE_CHANNELS_BASE_URL,
}: {
  channel: DesktopUpdateChannel;
  platform: NodeJS.Platform;
  arch: string;
  directURL?: string;
  channelsBaseURL?: string;
}): string {
  const configured = normalizeHTTPSURL(directURL);
  if (configured) {
    return configured;
  }
  const baseURL = normalizeHTTPSURL(channelsBaseURL);
  if (!baseURL) {
    return "";
  }
  const feedURL = `${baseURL}/${channel}/updates/${platform}/${arch}`;
  return platform === "darwin" ? `${feedURL}/RELEASES.json` : feedURL;
}

export function desktopDownloadsManifestURL({
  channel,
  channelsBaseURL = DEFAULT_UPDATE_CHANNELS_BASE_URL,
}: {
  channel: DesktopUpdateChannel;
  channelsBaseURL?: string;
}): string {
  const baseURL = normalizeHTTPSURL(channelsBaseURL);
  return baseURL ? `${baseURL}/${channel}/downloads.json` : "";
}

export function parseDesktopDownloadsManifest(
  payload: unknown,
  expectedChannel: DesktopUpdateChannel,
): DesktopDownloadsManifest {
  if (!payload || typeof payload !== "object" || Array.isArray(payload)) {
    throw new Error("Desktop downloads manifest must be a JSON object.");
  }
  const source = payload as Record<string, unknown>;
  if (source.schema_version !== 1) {
    throw new Error(
      `Unsupported desktop downloads manifest schema: ${String(source.schema_version)}`,
    );
  }
  if (source.channel !== expectedChannel) {
    throw new Error(
      `Desktop downloads manifest channel is ${String(source.channel)}, expected ${expectedChannel}.`,
    );
  }
  const rawLatest =
    typeof source.latest === "string" ? source.latest.trim() : "";
  const latest = rawLatest.replace(/^v/, "");
  if (!latest) {
    throw new Error("Desktop downloads manifest latest version is missing.");
  }
  const versions = source.versions;
  if (
    !versions ||
    typeof versions !== "object" ||
    Array.isArray(versions) ||
    (!(rawLatest in versions) && !(latest in versions))
  ) {
    throw new Error(
      `Desktop downloads manifest latest version ${latest} is missing from versions.`,
    );
  }
  return { channel: expectedChannel, latest };
}

export function normalizeHTTPSURL(rawURL: string): string {
  const value = rawURL.trim();
  if (!value) {
    return "";
  }
  try {
    const parsed = new URL(value);
    if (
      parsed.protocol !== "https:" ||
      parsed.username ||
      parsed.password ||
      parsed.search ||
      parsed.hash
    ) {
      return "";
    }
    return parsed.toString().replace(/\/+$/, "");
  } catch {
    return "";
  }
}
