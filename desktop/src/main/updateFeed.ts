import type { DesktopUpdateChannel } from "../shared/desktopBridge.types";

export const DEFAULT_UPDATE_CHANNELS_BASE_URL =
  "https://opencsg-public-resource.oss-cn-beijing.aliyuncs.com/csgclaw-desktop/channels";

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
  return `${baseURL}/${channel}/updates/${platform}/${arch}`;
}

export function normalizeHTTPSURL(rawURL: string): string {
  const value = rawURL.trim();
  if (!value) {
    return "";
  }
  try {
    const parsed = new URL(value);
    if (parsed.protocol !== "https:" || parsed.username || parsed.password || parsed.search || parsed.hash) {
      return "";
    }
    return parsed.toString().replace(/\/+$/, "");
  } catch {
    return "";
  }
}
