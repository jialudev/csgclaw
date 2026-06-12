import { render, screen } from "@testing-library/react";
import { AgentProfileModal } from "@/pages/WorkspacePage/components";
import { agentToDraft } from "@/models/agents";

const labels: Record<string, string> = {
  agentDescription: "Description",
  agentInstructions: "Instructions",
  agentImage: "Image",
  agentName: "Name",
  agentNamePlaceholder: "For example: dev",
  close: "Close",
  editAgentSubtitle: "Change runtime settings.",
  editAgentTitle: "Edit Agent",
  profileBasics: "Basics",
  profileModelSection: "Model",
  profileProvider: "Provider",
  profileRuntimeKind: "Runtime",
};

function t(key: string): string {
  return labels[key] ?? key;
}

const worker = {
  id: "worker-1",
  name: "Worker",
  role: "worker",
  runtime_kind: "picoclaw_sandbox",
  status: "running",
  provider: "api",
  model_id: "gpt-test",
  image: "worker:latest",
};

describe("AgentProfileModal", () => {
  it("does not preselect an avatar in create mode", () => {
    const draft = { ...agentToDraft(worker), avatar: "" };

    const { container } = render(
      <AgentProfileModal
        t={t}
        agentModalMode="create"
        editingAgent={null}
        agentDraft={draft}
        onAgentDraftChange={vi.fn()}
        onAgentModelsReset={vi.fn()}
        hubTemplates={[]}
        bootstrapConfig={{}}
        managerAgent={null}
        agentModels={[]}
        agentModelBusy={false}
        authStatuses={{}}
        authBusyProvider=""
        agentCreateBotKind="worker"
        onAgentCreateBotKindChange={vi.fn()}
        notifierWebhookPublicOrigin="http://127.0.0.1:18080"
        onProviderLogin={vi.fn()}
        agentError=""
        agentProgress={null}
        agentBusy={false}
        onClose={vi.fn()}
        onSave={vi.fn()}
      />,
    );

    expect(container.querySelectorAll(".agent-avatar-option.selected")).toHaveLength(0);
  });

  it("keeps the agent name read-only in edit mode", () => {
    render(
      <AgentProfileModal
        t={t}
        agentModalMode="edit"
        editingAgent={worker}
        agentDraft={agentToDraft(worker)}
        onAgentDraftChange={vi.fn()}
        onAgentModelsReset={vi.fn()}
        hubTemplates={[]}
        bootstrapConfig={{}}
        managerAgent={null}
        agentModels={[]}
        agentModelBusy={false}
        authStatuses={{}}
        authBusyProvider=""
        agentCreateBotKind="worker"
        onAgentCreateBotKindChange={vi.fn()}
        notifierWebhookPublicOrigin="http://127.0.0.1:18080"
        onProviderLogin={vi.fn()}
        agentError=""
        agentProgress={null}
        agentBusy={false}
        onClose={vi.fn()}
        onSave={vi.fn()}
      />,
    );

    const nameInput = screen.getByDisplayValue("Worker");
    expect(nameInput).toBeDisabled();
    expect(nameInput).toHaveAttribute("readonly");
  });

  it("shows the instructions field for worker agents after the model section", () => {
    const { container } = render(
      <AgentProfileModal
        t={t}
        agentModalMode="edit"
        editingAgent={worker}
        agentDraft={{ ...agentToDraft(worker), instructions: "reply in Chinese" }}
        onAgentDraftChange={vi.fn()}
        onAgentModelsReset={vi.fn()}
        hubTemplates={[]}
        bootstrapConfig={{}}
        managerAgent={null}
        agentModels={[]}
        agentModelBusy={false}
        authStatuses={{}}
        authBusyProvider=""
        agentCreateBotKind="worker"
        onAgentCreateBotKindChange={vi.fn()}
        notifierWebhookPublicOrigin="http://127.0.0.1:18080"
        onProviderLogin={vi.fn()}
        agentError=""
        agentProgress={null}
        agentBusy={false}
        onClose={vi.fn()}
        onSave={vi.fn()}
      />,
    );

    const modelSectionTitle = screen.getByText("Model");
    const instructionSectionTitle = screen.getAllByText("Instructions")[0];
    expect(instructionSectionTitle).toBeInTheDocument();
    expect(screen.getByDisplayValue("reply in Chinese")).toBeInTheDocument();
    expect(modelSectionTitle.compareDocumentPosition(instructionSectionTitle)).toBe(Node.DOCUMENT_POSITION_FOLLOWING);
    expect(container.querySelector(".profile-section .profile-section-title")?.textContent).toBe("Basics");
  });

  it("places avatar and name above a full-width description before the create-kind tabs", () => {
    const { container } = render(
      <AgentProfileModal
        t={t}
        agentModalMode="create"
        editingAgent={null}
        agentDraft={{ ...agentToDraft(worker), avatar: "", description: "Research and report findings." }}
        onAgentDraftChange={vi.fn()}
        onAgentModelsReset={vi.fn()}
        hubTemplates={[]}
        bootstrapConfig={{}}
        managerAgent={null}
        agentModels={[]}
        agentModelBusy={false}
        authStatuses={{}}
        authBusyProvider=""
        agentCreateBotKind="worker"
        onAgentCreateBotKindChange={vi.fn()}
        notifierWebhookPublicOrigin="http://127.0.0.1:18080"
        onProviderLogin={vi.fn()}
        agentError=""
        agentProgress={null}
        agentBusy={false}
        onClose={vi.fn()}
        onSave={vi.fn()}
      />,
    );

    const identityLayout = container.querySelector(".agent-identity-layout");
    expect(identityLayout).toBeInTheDocument();
    expect(identityLayout?.querySelector(".agent-avatar-field .agent-avatar-picker")).toBeInTheDocument();
    expect(identityLayout?.querySelector(".agent-name-field")).toBeInTheDocument();
    expect(identityLayout?.querySelector(".agent-description-field")).toBeInTheDocument();
    expect(screen.getByDisplayValue("Worker")).toBeInTheDocument();
    expect(screen.getByDisplayValue("Research and report findings.")).toBeInTheDocument();
    const tabbar = container.querySelector(".agent-create-kind-tabbar");
    expect(identityLayout?.compareDocumentPosition(tabbar as Node)).toBe(Node.DOCUMENT_POSITION_FOLLOWING);
  });

  it("flattens notification bot fields without basics or notifications section titles", () => {
    render(
      <AgentProfileModal
        t={t}
        agentModalMode="create"
        editingAgent={null}
        agentDraft={{ ...agentToDraft(worker), bot_type: "notification", notifier_delivery_mode: "webhook" }}
        onAgentDraftChange={vi.fn()}
        onAgentModelsReset={vi.fn()}
        hubTemplates={[]}
        bootstrapConfig={{}}
        managerAgent={null}
        agentModels={[]}
        agentModelBusy={false}
        authStatuses={{}}
        authBusyProvider=""
        agentCreateBotKind="notification"
        onAgentCreateBotKindChange={vi.fn()}
        notifierWebhookPublicOrigin="http://127.0.0.1:18080"
        onProviderLogin={vi.fn()}
        agentError=""
        agentProgress={null}
        agentBusy={false}
        onClose={vi.fn()}
        onSave={vi.fn()}
      />,
    );

    expect(screen.queryByText("Basics")).not.toBeInTheDocument();
    expect(screen.queryByText("profileNotifierSection")).not.toBeInTheDocument();
    expect(screen.getByText("notifierDeliveryMode")).toBeInTheDocument();
  });
});
