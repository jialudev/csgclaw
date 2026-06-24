import { ExternalLink } from "lucide-react";
import type { TranslateFn } from "@/models/conversations";
import type { UpgradeStatus } from "@/models/upgradeStatus";
import { formatSidebarVersionLabel } from "@/models/upgradeStatus";
import styles from "./ConfigSettingsFeedback.module.css";

const GITHUB_ISSUE_CREATE_URL = "https://github.com/OpenCSGs/csgclaw/issues/new";
const GITHUB_FEEDBACK_LABEL = "user-feedback";

type ConfigSettingsFeedbackProps = {
  appVersion: string;
  t: TranslateFn;
  upgradeStatus: UpgradeStatus | null;
};

export function ConfigSettingsFeedback({ appVersion, t, upgradeStatus }: ConfigSettingsFeedbackProps) {
  const issueURL = githubFeedbackIssueURL(appVersion, upgradeStatus);

  return (
    <section className={`config-settings-section ${styles.section}`}>
      <h3 className="config-settings-section-title">{t("configSettingsFeedbackSection")}</h3>
      <div className={styles.panel}>
        <p>{t("configSettingsFeedbackSubtitle")}</p>
        <a
          className={styles.iconLink}
          href={issueURL}
          target="_blank"
          rel="noreferrer"
          aria-label={t("configSettingsGithubIssueAction")}
          title={t("configSettingsGithubIssueAction")}
        >
          <ExternalLink size={18} strokeWidth={2.1} aria-hidden="true" />
        </a>
      </div>
    </section>
  );
}

function githubFeedbackIssueURL(appVersion: string, upgradeStatus: UpgradeStatus | null): string {
  const currentVersion = formatSidebarVersionLabel(upgradeStatus?.current_version || appVersion || "dev");
  const body = ["## Version information", `- CSGClaw version: ${currentVersion}`, ""].join("\n");
  const params = new URLSearchParams({
    body,
    labels: GITHUB_FEEDBACK_LABEL,
  });
  return `${GITHUB_ISSUE_CREATE_URL}?${params.toString()}`;
}
