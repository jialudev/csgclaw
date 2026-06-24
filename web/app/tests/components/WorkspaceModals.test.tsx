import { render, screen } from "@testing-library/react";
import {
  ConfigSettingsModal,
  CreateRoomModal,
  InviteMembersModal,
} from "@/pages/WorkspacePage/components/WorkspaceModals";
import type { TranslateFn } from "@/models/conversations";
import type { ConfigSettingsDraft } from "@/models/configSettings";

const t: TranslateFn = (key) => key;

const avatarUser = {
  avatar: "avatar/3D-2.png",
  handle: "avatar-user",
  id: "u-avatar",
  name: "Avatar User",
};

const configDraft: ConfigSettingsDraft = {
  access_token: "",
  access_token_preview: "",
  access_token_set: false,
  advertise_base_url: "",
  advertise_base_url_effective: "http://127.0.0.1:18080",
  default_manager_template: "picoclaw-manager",
  default_worker_template: "picoclaw-worker",
  listen_host: "127.0.0.1",
  listen_port: "18080",
  sandbox_provider: "boxlite",
  show_upgrade: true,
};

describe("WorkspaceModals", () => {
  it("renders create-room member avatars from user avatar paths", () => {
    const { container } = render(
      <CreateRoomModal
        candidates={[avatarUser]}
        lockedRoomMemberIDs={[]}
        onClose={() => {}}
        onCreate={() => {}}
        onRoomDescriptionChange={() => {}}
        onRoomMemberIDsChange={() => {}}
        onRoomTitleChange={() => {}}
        roomDescription=""
        roomMemberIDs={[]}
        roomTitle=""
        submitError=""
        t={t}
      />,
    );

    expect(container.querySelector(".create-room-avatar .agent-avatar-image")).toHaveAttribute(
      "src",
      avatarUser.avatar,
    );
  });

  it("renders invite member avatars from user avatar paths", () => {
    const { container } = render(
      <InviteMembersModal
        candidates={[avatarUser]}
        currentUserID="u-test"
        members={[avatarUser]}
        allowMemberRemoval={false}
        inviteUserIDs={[]}
        onClose={() => {}}
        onInvite={() => {}}
        onInviteUserIDsChange={() => {}}
        submitError=""
        t={t}
      />,
    );

    expect(container.querySelector(".create-room-avatar .agent-avatar-image")).toHaveAttribute(
      "src",
      avatarUser.avatar,
    );
  });

  it("renders the GitHub issue feedback link with user feedback prefilled", () => {
    render(
      <ConfigSettingsModal
        appVersion="v0.3.0"
        configBusy={false}
        configDraft={configDraft}
        configError=""
        configPhase="idle"
        hubTemplates={[]}
        onClose={() => {}}
        onDraftChange={() => {}}
        onReload={() => {}}
        onSaveAndRestart={async () => {}}
        sandboxProviders={["boxlite"]}
        t={t}
        upgradeStatus={null}
      />,
    );

    const link = screen.getByRole("link", { name: /configSettingsGithubIssueAction/ });
    const href = link.getAttribute("href") || "";
    const url = new URL(href);
    expect(`${url.origin}${url.pathname}`).toBe("https://github.com/OpenCSGs/csgclaw/issues/new");
    expect(url.searchParams.has("title")).toBe(false);
    expect(url.searchParams.get("labels")).toBe("user-feedback");
    expect(url.searchParams.get("body")).toBe("## Version information\n- CSGClaw version: v0.3.0\n");
    expect(link).toHaveAttribute("target", "_blank");
  });
});
