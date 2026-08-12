import fs from "node:fs";
import path from "node:path";
import { app, autoUpdater } from "electron";
import type { DesktopUpdateChannel, DesktopUpdateStatus } from "../shared/desktopBridge.types";
import { DesktopPlatform } from "../shared/desktopEnvironment";
import { desktopUpdateFeedURL, DEFAULT_UPDATE_CHANNELS_BASE_URL } from "./updateFeed";
import { usesMicrosoftStoreUpdates } from "./updatePolicy";

const STARTUP_CHECK_DELAY_MS = 5_000;
const PERIODIC_CHECK_INTERVAL_MS = 60 * 60 * 1000;
const UPDATE_PREFERENCES_FILE = "desktop-update-preferences.json";

export class DesktopUpdater {
  private channel: DesktopUpdateChannel;
  private checkActive = false;
  private downloaded = false;
  private periodicTimer: ReturnType<typeof setInterval> | null = null;
  private startupTimer: ReturnType<typeof setTimeout> | null = null;
  private status: DesktopUpdateStatus;

  constructor(
    private readonly publishStatus: (status: DesktopUpdateStatus) => void,
    private readonly beforeInstall: () => Promise<void>,
  ) {
    this.channel = loadUpdateChannel();
    this.status = {
      state: "idle",
      channel: this.channel,
      currentVersion: app.getVersion(),
    };
    this.bindEvents();
  }

  currentStatus(): DesktopUpdateStatus {
    return { ...this.status };
  }

  startBackgroundChecks(): void {
    if (this.startupTimer === null) {
      this.startupTimer = setTimeout(() => {
        this.startupTimer = null;
        void this.checkForUpdates().catch(() => undefined);
      }, STARTUP_CHECK_DELAY_MS);
    }
    if (this.periodicTimer === null) {
      this.periodicTimer = setInterval(() => {
        void this.checkForUpdates().catch(() => undefined);
      }, PERIODIC_CHECK_INTERVAL_MS);
    }
  }

  stopBackgroundChecks(): void {
    if (this.startupTimer !== null) {
      clearTimeout(this.startupTimer);
      this.startupTimer = null;
    }
    if (this.periodicTimer !== null) {
      clearInterval(this.periodicTimer);
      this.periodicTimer = null;
    }
  }

  async setChannel(channel: DesktopUpdateChannel): Promise<void> {
    if (channel === this.channel) {
      this.publishStatus({ ...this.status });
      return;
    }
    if (this.checkActive || this.downloaded) {
      throw new Error("Wait for the current desktop update before changing channels.");
    }
    this.channel = channel;
    saveUpdateChannel(channel);
    this.updateStatus({
      state: "idle",
      channel,
      currentVersion: app.getVersion(),
    });
    await this.checkForUpdates();
  }

  async checkForUpdates(): Promise<void> {
    if (this.downloaded) {
      this.publishStatus({ ...this.status });
      return;
    }
    if (this.checkActive) {
      return;
    }
    if (usesMicrosoftStoreUpdates(process.platform, process.windowsStore)) {
      this.updateStatus({
        state: "unsupported",
        channel: this.channel,
        currentVersion: app.getVersion(),
        message: "Updates are managed automatically by Microsoft Store.",
      });
      return;
    }
    if (process.platform === DesktopPlatform.Linux) {
      this.updateStatus({
        state: "unsupported",
        channel: this.channel,
        currentVersion: app.getVersion(),
        message: "Linux desktop updates are provided by the system package manager.",
      });
      return;
    }
    const updateURL = resolveUpdateURL(this.channel);
    if (!app.isPackaged || !updateURL) {
      this.updateStatus({
        state: "unsupported",
        channel: this.channel,
        currentVersion: app.getVersion(),
        message: app.isPackaged ? "Desktop update feed is not configured." : "Updates are disabled in development.",
      });
      return;
    }
    this.checkActive = true;
    try {
      autoUpdater.setFeedURL({ url: updateURL });
      autoUpdater.checkForUpdates();
    } catch (error) {
      this.checkActive = false;
      this.updateStatus({
        state: "error",
        channel: this.channel,
        currentVersion: app.getVersion(),
        message: error instanceof Error ? error.message : String(error),
      });
      throw error;
    }
  }

  async installDownloadedUpdate(): Promise<void> {
    if (!this.downloaded) {
      throw new Error("No desktop update has been downloaded.");
    }
    await this.beforeInstall();
    autoUpdater.quitAndInstall();
  }

  private bindEvents(): void {
    autoUpdater.on("checking-for-update", () => {
      this.updateStatus({
        state: "checking",
        channel: this.channel,
        currentVersion: app.getVersion(),
      });
    });
    autoUpdater.on("update-available", () => {
      // Squirrel downloads automatically. Keep the UI quiet until the package
      // is complete, matching the Multica background-update interaction.
      this.updateStatus({
        state: "available",
        channel: this.channel,
        currentVersion: app.getVersion(),
      });
    });
    autoUpdater.on("update-not-available", () => {
      this.checkActive = false;
      this.updateStatus({
        state: "not-available",
        channel: this.channel,
        currentVersion: app.getVersion(),
      });
    });
    autoUpdater.on("update-downloaded", (_event, _releaseNotes, releaseName) => {
      this.checkActive = false;
      this.downloaded = true;
      this.updateStatus({
        state: "downloaded",
        channel: this.channel,
        currentVersion: app.getVersion(),
        availableVersion: typeof releaseName === "string" ? releaseName : undefined,
      });
    });
    autoUpdater.on("error", (error) => {
      this.checkActive = false;
      this.updateStatus({
        state: "error",
        channel: this.channel,
        currentVersion: app.getVersion(),
        message: error.message,
      });
    });
  }

  private updateStatus(status: DesktopUpdateStatus): void {
    this.status = status;
    this.publishStatus({ ...status });
  }
}

function resolveUpdateURL(channel: DesktopUpdateChannel): string {
  return desktopUpdateFeedURL({
    channel,
    platform: process.platform,
    arch: process.arch,
    directURL: process.env.CSGCLAW_DESKTOP_UPDATE_URL || "",
    channelsBaseURL: readChannelsBaseURL(),
  });
}

function readChannelsBaseURL(): string {
  const configured = process.env.CSGCLAW_DESKTOP_UPDATE_CHANNELS_URL?.trim();
  if (configured) {
    return configured;
  }
  try {
    const source = JSON.parse(
      fs.readFileSync(path.join(process.resourcesPath, "desktop-update.json"), "utf8"),
    ) as unknown;
    if (source && typeof source === "object" && !Array.isArray(source)) {
      const value = String((source as Record<string, unknown>).channels_base_url || "").trim();
      if (value) {
        return value;
      }
    }
  } catch {
    // Fall back to the official public channel root.
  }
  return DEFAULT_UPDATE_CHANNELS_BASE_URL;
}

function updatePreferencesPath(): string {
  return path.join(app.getPath("userData"), UPDATE_PREFERENCES_FILE);
}

function loadUpdateChannel(): DesktopUpdateChannel {
  try {
    const source = JSON.parse(fs.readFileSync(updatePreferencesPath(), "utf8")) as unknown;
    if (source && typeof source === "object" && !Array.isArray(source)) {
      const channel = (source as Record<string, unknown>).channel;
      if (channel === "release" || channel === "beta") {
        return channel;
      }
    }
  } catch {
    // Missing or invalid preferences use the stable channel.
  }
  return "release";
}

function saveUpdateChannel(channel: DesktopUpdateChannel): void {
  const filePath = updatePreferencesPath();
  fs.mkdirSync(path.dirname(filePath), { recursive: true });
  fs.writeFileSync(filePath, `${JSON.stringify({ channel }, null, 2)}\n`, {
    mode: 0o600,
  });
}
