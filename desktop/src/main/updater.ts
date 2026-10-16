import { app } from "electron";
import type {
  DesktopUpdateChannel,
  DesktopUpdateStatus,
} from "../shared/desktopBridge.types";
import { inferDesktopUpdateChannel } from "../shared/releaseVersion";

// This branch builds an isolated Win7 experiment. Never request release/beta
// feeds or launch an installer that would replace it with a modern runtime.
export class DesktopUpdater {
  private readonly status: DesktopUpdateStatus;

  constructor(
    private readonly publishStatus: (status: DesktopUpdateStatus) => void,
    _beforeInstall: () => Promise<void>,
    version = app.getVersion(),
  ) {
    this.status = {
      state: "unsupported",
      channel: inferDesktopUpdateChannel(version),
      currentVersion: version,
      message:
        "Updates are disabled in the Windows 7 compatibility test build.",
    };
  }

  currentStatus(): DesktopUpdateStatus {
    return { ...this.status };
  }

  startBackgroundChecks(): void {
    this.publishStatus(this.currentStatus());
  }

  stopBackgroundChecks(): void {
    // No timers, network requests, or update processes are created.
  }

  async setChannel(_channel: DesktopUpdateChannel): Promise<void> {
    this.publishStatus(this.currentStatus());
  }

  async checkForUpdates(): Promise<void> {
    this.publishStatus(this.currentStatus());
  }

  async installDownloadedUpdate(): Promise<void> {
    throw new Error(this.status.message);
  }
}
