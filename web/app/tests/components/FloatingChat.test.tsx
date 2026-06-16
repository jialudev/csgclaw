import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { FloatingChat } from "@/pages/WorkspacePage/components/FloatingChat";
import type { TranslateFn } from "@/models/conversations";

const GUIDE_STORAGE_KEY = "csgclaw:floating-chat:manager-guide:v1";

const labels: Record<string, string> = {
  floatingChatGuideDismiss: "Do not show again",
  floatingChatGuideTitle: "Manager moved here",
  floatingChatOpen: "Open floating chat",
};

const t: TranslateFn = (key) => labels[key] ?? key;

function renderFloatingChat(props: { open?: boolean; onOpenChange?: (open: boolean) => void } = {}) {
  const onOpenChange = props.onOpenChange ?? vi.fn();
  render(
    <FloatingChat
      avatarFallback="M"
      chatProps={null}
      locale="en"
      open={props.open ?? false}
      t={t}
      title="Manager"
      onOpenChange={onOpenChange}
    />,
  );
  return { onOpenChange };
}

describe("FloatingChat manager guide", () => {
  beforeEach(() => {
    window.localStorage.clear();
  });

  it("shows a first-use manager guide and stores confirmation when opening it", async () => {
    const user = userEvent.setup();
    const { onOpenChange } = renderFloatingChat();

    expect(screen.getByText("Manager moved here")).toBeInTheDocument();

    await user.click(screen.getByRole("button", { name: "Manager moved here" }));

    expect(onOpenChange).toHaveBeenCalledWith(true);
    expect(window.localStorage.getItem(GUIDE_STORAGE_KEY)).toBe("seen");
  });

  it("does not show the guide after the user has acknowledged it", () => {
    window.localStorage.setItem(GUIDE_STORAGE_KEY, "seen");

    renderFloatingChat();

    expect(screen.queryByText("Manager moved here")).not.toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Open floating chat" })).toBeInTheDocument();
  });

  it("treats clicking the floating launcher as using the manager entry", async () => {
    const user = userEvent.setup();
    const { onOpenChange } = renderFloatingChat();

    await user.click(screen.getByRole("button", { name: "Open floating chat" }));

    expect(onOpenChange).toHaveBeenCalledWith(true);
    expect(window.localStorage.getItem(GUIDE_STORAGE_KEY)).toBe("seen");
  });
});
