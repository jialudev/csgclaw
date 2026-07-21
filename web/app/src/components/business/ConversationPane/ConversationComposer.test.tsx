import { createRef } from "react";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";
import { ConversationComposer } from "./ConversationComposer";
import { ConversationWorkingActions, type ConversationWorkingParticipant } from "./types";

describe("ConversationComposer working activity", () => {
  it("shows the latest activity instead of a generic working label and opens that activity", async () => {
    const user = userEvent.setup();
    const participant: ConversationWorkingParticipant = {
      activity: {
        action: ConversationWorkingActions.replying,
        entryID: "manager:message-7",
        summary: "正在检查可用的 agent",
      },
      id: "u-manager",
      name: "manager",
    };
    const onWorkingAction = vi.fn();

    const { container } = render(
      <ConversationComposer
        authBusyProvider=""
        authStatuses={{}}
        composerDisabled={false}
        composerError=""
        draftSegments={[]}
        draftText=""
        editorRef={createRef<HTMLDivElement>()}
        managerProvider=""
        mentionCandidates={[]}
        mentionIndex={0}
        mentionableUsersByName={new Map()}
        slashCandidates={[]}
        slashIndex={0}
        slashPickerLoading={false}
        slashPickerOpen={false}
        t={(key, params) => {
          if (key === "conversationWorkingReplying") return "正在回复";
          if (key === "conversationWorkingOpenActivity") return `查看 ${params?.name} 的活动记录`;
          return key;
        }}
        workingParticipants={[participant]}
        onApplyMention={vi.fn()}
        onApplySlashCandidate={vi.fn()}
        onComposerCompositionEnd={vi.fn()}
        onComposerCompositionStart={vi.fn()}
        onComposerKeyDown={vi.fn()}
        onProviderLogin={vi.fn()}
        onSendMessage={vi.fn()}
        onSyncComposer={vi.fn()}
        onWorkingAction={onWorkingAction}
      />,
    );

    const activity = screen.getByRole("button", { name: "查看 manager 的活动记录" });
    expect(activity).toHaveTextContent("manager");
    expect(activity).toHaveTextContent("正在回复");
    expect(activity).toHaveTextContent("正在检查可用的 agent");
    expect(screen.queryByText("manager 正在工作")).not.toBeInTheDocument();
    expect(container.querySelector(".composer > .composer-working")).toBeInTheDocument();
    expect(container.querySelector(".composer-box .composer-working")).not.toBeInTheDocument();

    await user.click(activity);
    expect(onWorkingAction).toHaveBeenCalledWith(participant);
  });
});
