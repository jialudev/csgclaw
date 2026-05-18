// @ts-nocheck
import { GATEWAY_RUNTIME_KIND_OPTIONS, PROVIDERS, RUNTIME_KIND_OPTIONS } from "@/bootstrap/constants";
import { AgentCreateProgress, APIKeyField, CLIProxyAuthControl, EnvKeyValueEditor, isBlank, NotifierControls, profileBaseURLMissing, requiredFieldLabel } from "@/components/business/ProfileControls";
import { Button } from "@/components/ui";
import { applyTemplateToDraft, ensureNotifierPullSubscriptionDraft, formatProviderLabel, formatRuntimeKindLabel, isNotifierRuntimeDraftOnAgentPage, normalizeAuthProviderName, normalizeRuntimeKind, normalizeTemplateSelection, notifierFormIsComplete, pickDefaultAgentTemplate, runtimeImageForKind, templateMatchesRuntime } from "@/models/agents";
import { upgradeStatusLabel } from "@/models/upgradeStatus";
import { toggleSelection } from "@/shared/lib/collections";

export function CreateRoomModal({
  t,
  roomTitle,
  onRoomTitleChange,
  roomDescription,
  onRoomDescriptionChange,
  candidates,
  roomMemberIDs,
  lockedRoomMemberIDs,
  onRoomMemberIDsChange,
  submitError,
  onClose,
  onCreate,
}) {
  const candidateIDs = candidates.map((user) => user.id).filter(Boolean);
  const selectableMemberIDs = candidateIDs.filter((id) => !lockedRoomMemberIDs.includes(id));
  const allMembersSelected = candidateIDs.length > 0 && candidateIDs.every((id) => roomMemberIDs.includes(id));
  const selectedMemberCount = candidateIDs.filter((id) => roomMemberIDs.includes(id)).length;

  return (
    <div className="modal-backdrop" onClick={onClose}>
      <div className="modal-card" onClick={(event) => event.stopPropagation()}>
        <div className="modal-header">
          <div>
            <div className="modal-title">{t("createRoomTitle")}</div>
            <div className="modal-subtitle">{t("createRoomSubtitle")}</div>
          </div>
          <Button className="modal-close" onClick={onClose}>{t("close")}</Button>
        </div>
        <label className="field">
          {requiredFieldLabel(t("roomName"))}
          <input
            value={roomTitle}
            required
            aria-required="true"
            onInput={(event) => onRoomTitleChange(event.target.value)}
            placeholder={t("roomNamePlaceholder")}
          />
        </label>
        <label className="field">
          <span>{t("roomDescription")}</span>
          <textarea value={roomDescription} onInput={(event) => onRoomDescriptionChange(event.target.value)} placeholder={t("roomDescriptionPlaceholder")} />
        </label>
        <div className="field">
          <span>{t("initialMembers")}</span>
          <div className="selection-list">
            <label className="selection-item selection-all-item">
              <input
                type="checkbox"
                checked={allMembersSelected}
                disabled={selectableMemberIDs.length === 0}
                onChange={() => {
                  onRoomMemberIDsChange((current) => {
                    const allSelected = candidateIDs.length > 0 && candidateIDs.every((id) => current.includes(id));
                    if (allSelected) {
                      return current.filter((id) => !selectableMemberIDs.includes(id));
                    }
                    return Array.from(new Set([...current, ...selectableMemberIDs]));
                  });
                }}
              />
              <span>{t("allMembers")}</span>
              <small>{selectedMemberCount}/{candidateIDs.length}</small>
            </label>
            {candidates.map((user) => (
              <label key={user.id} className="selection-item">
                <input
                  type="checkbox"
                  checked={roomMemberIDs.includes(user.id)}
                  disabled={lockedRoomMemberIDs.includes(user.id)}
                  onChange={() => onRoomMemberIDsChange((current) => toggleSelection(current, user.id))}
                />
                <span>{user.name}</span>
                <small>@{user.handle}</small>
              </label>
            ))}
          </div>
        </div>
        {submitError ? (<div className="form-error">{submitError}</div>) : null}
        <div className="modal-actions">
          <Button className="secondary-button" onClick={onClose}>{t("cancel")}</Button>
          <Button variant="primary" className="send-button" disabled={isBlank(roomTitle)} onClick={onCreate}>{t("create")}</Button>
        </div>
      </div>
    </div>
  );
}

export function InviteMembersModal({
  t,
  candidates,
  inviteUserIDs,
  onInviteUserIDsChange,
  submitError,
  onClose,
  onInvite,
}) {
  const candidateIDs = candidates.map((user) => user.id).filter(Boolean);
  const allCandidatesSelected = candidateIDs.length > 0 && candidateIDs.every((id) => inviteUserIDs.includes(id));
  const selectedMemberCount = candidateIDs.filter((id) => inviteUserIDs.includes(id)).length;

  return (
    <div className="modal-backdrop" onClick={onClose}>
      <div className="modal-card" onClick={(event) => event.stopPropagation()}>
        <div className="modal-header">
          <div>
            <div className="modal-title">{t("inviteTitle")}</div>
            <div className="modal-subtitle">{t("inviteSubtitle")}</div>
          </div>
          <Button className="modal-close" onClick={onClose}>{t("close")}</Button>
        </div>
        <div className="field">
          <span>{t("inviteCandidates")}</span>
          <div className="selection-list">
            {candidates.length > 0
              ? (
                  <>
                  <label className="selection-item selection-all-item">
                    <input
                      type="checkbox"
                      checked={allCandidatesSelected}
                      onChange={() => {
                        onInviteUserIDsChange((current) => {
                          const allSelected = candidateIDs.length > 0 && candidateIDs.every((id) => current.includes(id));
                          if (allSelected) {
                            return current.filter((id) => !candidateIDs.includes(id));
                          }
                          return Array.from(new Set([...current, ...candidateIDs]));
                        });
                      }}
                    />
                    <span>{t("allMembers")}</span>
                    <small>{selectedMemberCount}/{candidateIDs.length}</small>
                  </label>
                  {candidates.map((user) => (
                    <label key={user.id} className="selection-item">
                      <input
                        type="checkbox"
                        checked={inviteUserIDs.includes(user.id)}
                        onChange={() => onInviteUserIDsChange((current) => toggleSelection(current, user.id))}
                      />
                      <span>{user.name}</span>
                      <small>@{user.handle}</small>
                    </label>
                  ))}
                  </>
                )
              : (<div className="selection-empty">{t("noInviteCandidates")}</div>)}
          </div>
        </div>
        {submitError ? (<div className="form-error">{submitError}</div>) : null}
        <div className="modal-actions">
          <Button className="secondary-button" onClick={onClose}>{t("cancel")}</Button>
          <Button variant="primary" className="send-button" disabled={inviteUserIDs.length === 0} onClick={onInvite}>{t("sendInvite")}</Button>
        </div>
      </div>
    </div>
  );
}

export function UpgradeModal({
  t,
  upgradeStatus,
  appVersion,
  upgradePhase,
  upgradeBusy,
  upgradeError,
  onClose,
  onApply,
}) {
  return (
    <div className="modal-backdrop" onClick={onClose}>
      <div className="modal-card upgrade-modal" onClick={(event) => event.stopPropagation()}>
        <div className="modal-header">
          <div>
            <div className="modal-title">{t("upgradeTitle")}</div>
            <div className="modal-subtitle">{t("upgradeSubtitle")}</div>
          </div>
          <Button
            className="modal-close"
            onClick={onClose}
          >
            {t("close")}
          </Button>
        </div>
        <div className="upgrade-summary">
          <div className="upgrade-summary-row">
            <span>{t("upgradeCurrentVersion")}</span>
            <strong>{upgradeStatus?.current_version || appVersion || "dev"}</strong>
          </div>
          <div className="upgrade-summary-row">
            <span>{t("upgradeLatestVersion")}</span>
            <strong>{upgradeStatus?.latest_version || t("upgradeNoLatest")}</strong>
          </div>
          <div className="upgrade-summary-row">
            <span>{t("upgradeStatus")}</span>
            <strong>{upgradeStatusLabel(upgradePhase, t)}</strong>
          </div>
        </div>
        <div className={`upgrade-status-card ${upgradePhase}`}>
          <span className="upgrade-status-dot" aria-hidden="true"></span>
          <p>
            {upgradePhase === "done"
              ? t("upgradeDoneBody")
              : upgradePhase === "restarting" || upgradePhase === "starting" || upgradeBusy || upgradeStatus?.upgrading
                ? t("upgradeContinueUsing")
                : t("upgradeConfirmBody")}
          </p>
        </div>
        {upgradeError || upgradeStatus?.last_error
          ? (<div className="form-error">{upgradeError || upgradeStatus.last_error}</div>)
          : null}
        <div className="modal-actions">
          {upgradePhase === "done"
            ? (
                <Button variant="primary" className="send-button" onClick={() => window.location.reload()}>
                  {t("upgradeRefresh")}
                </Button>
              )
            : (
                <>
                <Button
                  className="secondary-button"
                  onClick={onClose}
                >
                  {upgradeBusy || upgradeStatus?.upgrading ? t("close") : t("upgradeLater")}
                </Button>
                <Button
                  variant="primary"
                  className="send-button"
                  disabled={upgradeBusy || upgradeStatus?.upgrading || !upgradeStatus?.update_available}
                  onClick={onApply}
                >
                  {upgradeBusy || upgradeStatus?.upgrading ? t("upgradeActionBusy") : t("upgradeConfirm")}
                </Button>
                </>
              )}
        </div>
      </div>
    </div>
  );
}

export function ManagerRebuildModal({
  t,
  runtimeOptions,
  runtimeKind,
  image,
  bootstrapConfig,
  managerAgent,
  busy,
  error,
  onRuntimeKindChange,
  onImageChange,
  onClose,
  onConfirm,
}) {
  const selectedRuntimeKind = normalizeRuntimeKind(runtimeKind) || runtimeOptions[0]?.value || "picoclaw_sandbox";
  return (
    <div className="modal-backdrop" onClick={onClose}>
      <div className="modal-card profile-modal" onClick={(event) => event.stopPropagation()}>
        <div className="modal-header">
          <div>
            <div className="modal-title">{t("managerRebuildTitle")}</div>
            <div className="modal-subtitle">{t("managerRebuildSubtitle")}</div>
          </div>
          <Button className="modal-close" onClick={onClose}>{t("close")}</Button>
        </div>
        <div className="profile-editor-shell">
          <section className="profile-section">
            <div className="profile-grid profile-grid-compact manager-rebuild-grid">
              <label className="field manager-rebuild-runtime-field">
                <span>{t("profileRuntimeKind")}</span>
                <select
                  value={selectedRuntimeKind}
                  onChange={(event) => {
                    const nextRuntimeKind = normalizeRuntimeKind(event.target.value);
                    onRuntimeKindChange(nextRuntimeKind);
                    onImageChange(runtimeImageForKind(nextRuntimeKind, bootstrapConfig, managerAgent?.image || ""));
                  }}
                >
                  {runtimeOptions.map((option) => (
                    <option key={option.value} value={option.value}>
                      {formatRuntimeKindLabel(option.value, t)}
                    </option>
                  ))}
                </select>
              </label>
              <label className="field manager-rebuild-image-field">
                <span>{t("agentImage")}</span>
                <input
                  value={image}
                  onInput={(event) => onImageChange(event.target.value)}
                  placeholder={t("agentImagePlaceholder")}
                />
              </label>
            </div>
          </section>
          {error ? (<div className="form-error">{error}</div>) : null}
          <div className="modal-actions">
            <Button className="secondary-button" disabled={busy} onClick={onClose}>
              {t("close")}
            </Button>
            <Button variant="primary" className="send-button" disabled={busy} onClick={onConfirm}>
              {busy ? t("profileLoadingModels") : t("managerRebuildAction")}
            </Button>
          </div>
        </div>
      </div>
    </div>
  );
}

export function AgentProfileModal({
  t,
  agentModalMode,
  editingAgent,
  agentDraft,
  onAgentDraftChange,
  onAgentModelsReset,
  hubTemplates,
  bootstrapConfig,
  managerAgent,
  agentModels,
  agentModelBusy,
  authStatuses,
  authBusyProvider,
  notifierWebhookOrigin,
  setNotifierWebhookOrigin,
  onProviderLogin,
  agentError,
  agentProgress,
  agentBusy,
  onClose,
  onSave,
}) {
  return (
    <div className="modal-backdrop" onClick={onClose}>
      <div className="modal-card profile-modal agent-modal" onClick={(event) => event.stopPropagation()}>
        <div className="modal-header">
          <div>
            <div className="modal-title">{agentModalMode === "create" ? t("createAgentTitle") : t("editAgentTitle")}</div>
            <div className="modal-subtitle">
              {agentModalMode === "create"
                ? isNotifierRuntimeDraftOnAgentPage(agentDraft, editingAgent)
                  ? t("createAgentSubtitleNotifier")
                  : t("createAgentSubtitle")
                : t("editAgentSubtitle")}
            </div>
          </div>
          <Button className="modal-close" onClick={onClose}>{t("close")}</Button>
        </div>
        <div className="profile-editor-shell">
          <section className="profile-section">
            <div className="profile-section-title">{t("profileBasics")}</div>
            <div className="profile-grid profile-grid-compact">
              {agentModalMode === "create"
                ? (
                    <label className="field span-2">
                      <span>{t("templateLabel")}</span>
                      <select
                        value={agentDraft.from_template || ""}
                        onChange={(event) => {
                          const nextTemplate = normalizeTemplateSelection(hubTemplates.find((item) => item.id === event.target.value) || null);
                          onAgentDraftChange((current) => applyTemplateToDraft(current, nextTemplate, bootstrapConfig, managerAgent?.image || ""));
                        }}
                      >
                        <option value="">{t("templateNone")}</option>
                        {hubTemplates.map((item) => (
                          <option key={item.id} value={item.id}>{item.name || item.id}</option>
                        ))}
                      </select>
                    </label>
                  )
                : null}
              <label className="field">
                {requiredFieldLabel(t("agentName"))}
                <input
                  value={agentDraft.name}
                  disabled={agentModalMode === "edit" && editingAgent?.id === "u-manager"}
                  required
                  aria-required="true"
                  onInput={(event) => onAgentDraftChange({ ...agentDraft, name: event.target.value })}
                  placeholder={t("agentNamePlaceholder")}
                />
              </label>
              {agentModalMode === "create"
                ? (
                    <label className="field">
                      <span>{t("roleLabel")}</span>
                      <input value={agentDraft.role || "worker"} readOnly disabled />
                    </label>
                  )
                : null}
              <label className="field">
                <span>{t("profileRuntimeKind")}</span>
                {agentModalMode === "create"
                  ? (
                      <select
                        value={normalizeRuntimeKind(agentDraft.runtime_kind) || "picoclaw_sandbox"}
                        onChange={(event) => {
                          const runtimeKind = normalizeRuntimeKind(event.target.value);
                          const currentTemplate = normalizeTemplateSelection(hubTemplates.find((item) => item.id === agentDraft.from_template) || null);
                          const nextTemplate = templateMatchesRuntime(currentTemplate, runtimeKind)
                            ? currentTemplate
                            : pickDefaultAgentTemplate(hubTemplates, runtimeKind, bootstrapConfig);
                          let nextDraft = {
                            ...agentDraft,
                            role: "worker",
                            runtime_kind: runtimeKind,
                            image: runtimeImageForKind(runtimeKind, bootstrapConfig, agentDraft.default_image || managerAgent?.image || ""),
                          };
                          nextDraft = applyTemplateToDraft(nextDraft, nextTemplate, bootstrapConfig, managerAgent?.image || "");
                          if (runtimeKind === "notifier") {
                            nextDraft = ensureNotifierPullSubscriptionDraft({
                              ...nextDraft,
                              from_template: "",
                              image: "",
                              notifier_delivery_mode: nextDraft.notifier_delivery_mode || "webhook",
                              template_name: "",
                            });
                          }
                          onAgentDraftChange(nextDraft);
                        }}
                      >
                        {RUNTIME_KIND_OPTIONS.map((option) => (
                          <option key={option.value} value={option.value}>{formatRuntimeKindLabel(option.value, t)}</option>
                        ))}
                      </select>
                    )
                  : (<input value={agentDraft.runtime_kind || editingAgent?.runtime_kind || ""} readOnly disabled />)}
              </label>
              {!isNotifierRuntimeDraftOnAgentPage(agentDraft, editingAgent)
                ? (
                    <label className="field">
                      <span>{t("agentImage")}</span>
                      <input
                        value={agentDraft.image}
                        readOnly={agentModalMode === "edit"}
                        disabled={agentModalMode === "edit"}
                        onInput={(event) => onAgentDraftChange({ ...agentDraft, image: event.target.value })}
                        placeholder={t("agentImagePlaceholder")}
                      />
                    </label>
                  )
                : null}
              <label className="field span-2">
                <span>{t("agentDescription")}</span>
                <textarea className="compact-textarea" value={agentDraft.description} onInput={(event) => onAgentDraftChange({ ...agentDraft, description: event.target.value })} />
              </label>
            </div>
          </section>
          {!isNotifierRuntimeDraftOnAgentPage(agentDraft, editingAgent)
            ? (
                <section className="profile-section">
                  <div className="profile-section-title">{t("profileModelSection")}</div>
                  <div className="profile-runtime-grid">
                    <label className="field">
                      <span>{t("profileProvider")}</span>
                      <select
                        value={agentDraft.provider}
                        onChange={(event) => {
                          onAgentDraftChange({ ...agentDraft, provider: event.target.value, model_id: "" });
                          onAgentModelsReset();
                        }}
                      >
                        {PROVIDERS.map((provider) => (
                          <option key={provider} value={provider}>{formatProviderLabel(provider)}</option>
                        ))}
                      </select>
                    </label>
                    <label className="field">
                      {requiredFieldLabel(t("profileModel"))}
                      <select
                        value={agentDraft.model_id}
                        required
                        aria-required="true"
                        onChange={(event) => onAgentDraftChange({ ...agentDraft, model_id: event.target.value })}
                      >
                        <option value="">{agentModelBusy ? t("profileLoadingModels") : t("profileSelectModel")}</option>
                        {agentModels.map((model) => (<option key={model} value={model}>{model}</option>))}
                        {agentDraft.model_id && !agentModels.includes(agentDraft.model_id)
                          ? (<option value={agentDraft.model_id}>{agentDraft.model_id}</option>)
                          : null}
                      </select>
                    </label>
                    <label className="field">
                      <span>{t("profileReasoning")}</span>
                      <select
                        value={agentDraft.reasoning_effort}
                        onChange={(event) => onAgentDraftChange({ ...agentDraft, reasoning_effort: event.target.value })}
                      >
                        {["low", "medium", "high", "xhigh"].map((effort) => (<option key={effort} value={effort}>{effort}</option>))}
                      </select>
                    </label>
                    <label className="selection-item compact-toggle-row">
                      <input type="checkbox" checked={agentDraft.enable_fast_mode} onChange={() => onAgentDraftChange({ ...agentDraft, enable_fast_mode: !agentDraft.enable_fast_mode })} />
                      <span>{t("profileFastMode")}</span>
                    </label>
                  </div>
                  <CLIProxyAuthControl
                    provider={agentDraft.provider}
                    t={t}
                    status={authStatuses[normalizeAuthProviderName(agentDraft.provider)]}
                    busy={authBusyProvider === normalizeAuthProviderName(agentDraft.provider)}
                    onLogin={onProviderLogin}
                  />
                </section>
              )
            : (
                <NotifierControls
                  agentID={agentModalMode === "edit" ? editingAgent?.id : ""}
                  draft={agentDraft}
                  t={t}
                  webhookOrigin={notifierWebhookOrigin}
                  setWebhookOrigin={setNotifierWebhookOrigin}
                  onPatch={(patch) => onAgentDraftChange({ ...agentDraft, ...patch })}
                />
              )}
          {!isNotifierRuntimeDraftOnAgentPage(agentDraft, editingAgent) && agentDraft.provider === "api"
            ? (
                <section className="profile-section">
                  <div className="profile-section-title">{t("profileAPIProvider")}</div>
                  <div className="profile-api-grid">
                    <label className="field">
                      {requiredFieldLabel(t("profileBaseURL"))}
                      <input
                        value={agentDraft.base_url}
                        required
                        aria-required="true"
                        onInput={(event) => onAgentDraftChange({ ...agentDraft, base_url: event.target.value })}
                        placeholder="https://api.openai.com/v1"
                      />
                    </label>
                    <APIKeyField
                      value={agentDraft.api_key}
                      onInput={(event) => onAgentDraftChange({ ...agentDraft, api_key: event.target.value })}
                      profile={agentDraft}
                      t={t}
                    />
                    <label className="field span-2">
                      <span>{t("profileHeaders")}</span>
                      <textarea className="compact-textarea" value={agentDraft.headersText} onInput={(event) => onAgentDraftChange({ ...agentDraft, headersText: event.target.value })} />
                    </label>
                  </div>
                </section>
              )
            : null}
          <section className="profile-section">
            <div className="profile-section-title">{t("profileAdvanced")}</div>
            <div className="profile-advanced-grid">
              {!isNotifierRuntimeDraftOnAgentPage(agentDraft, editingAgent)
                ? (
                    <label className="field">
                      <span>{t("profileRequestOptions")}</span>
                      <textarea className="compact-json" value={agentDraft.requestOptionsText} onInput={(event) => onAgentDraftChange({ ...agentDraft, requestOptionsText: event.target.value })} />
                    </label>
                  )
                : null}
              <div className="field">
                <span>{t("profileEnv")}</span>
                <EnvKeyValueEditor
                  rows={agentDraft.envRows}
                  t={t}
                  onChange={(rows) => onAgentDraftChange({ ...agentDraft, envRows: rows })}
                />
              </div>
            </div>
          </section>
        </div>
        {agentError ? (<div className="form-error">{agentError}</div>) : null}
        <AgentCreateProgress progress={agentProgress} t={t} />
        <div className="modal-actions">
          <Button className="secondary-button" onClick={onClose}>{t("cancel")}</Button>
          <Button
            variant="primary"
            className="send-button"
            disabled={agentBusy || isBlank(agentDraft.name) || (
              isNotifierRuntimeDraftOnAgentPage(agentDraft, editingAgent)
                ? !notifierFormIsComplete(agentDraft, editingAgent)
                : !agentDraft.model_id || profileBaseURLMissing(agentDraft)
            )}
            onClick={onSave}
          >
            {agentBusy ? "..." : agentModalMode === "create" ? t("agentCreateSave") : t("agentUpdateSave")}
          </Button>
        </div>
      </div>
    </div>
  );
}

export function ManagerProfileSetupModal({
  t,
  managerProfile,
  profileDraft,
  onProfileDraftChange,
  onProfileModelsReset,
  bootstrapConfig,
  profileModels,
  profileModelBusy,
  authStatuses,
  authBusyProvider,
  onProviderLogin,
  profileError,
  profileBusy,
  onSave,
}) {
  return (
    <div className="modal-backdrop profile-backdrop nonblocking">
      <div className="modal-card profile-modal" onClick={(event) => event.stopPropagation()}>
        <div className="modal-header">
          <div>
            <div className="modal-title">{t("profileSetupTitle")}</div>
            <div className="modal-subtitle">{t("profileSetupSubtitle")}</div>
          </div>
        </div>
        {managerProfile?.detection_results?.length
          ? (
              <div className="detection-list">
                <div className="section-label">{t("detectionResults")}</div>
                {managerProfile.detection_results.map((item) => (
                  <div key={item.provider} className={`detection-row ${item.status === "ok" ? "ok" : "failed"}`}>
                    <span>{formatProviderLabel(item.provider)}</span>
                    <small>{item.status === "ok" ? item.model_id : item.error}</small>
                  </div>
                ))}
              </div>
            )
          : null}
        <div className="profile-editor-shell">
          <section className="profile-section">
            <div className="profile-section-title">{t("profileModelSection")}</div>
            <div className="profile-runtime-grid">
              <label className="field">
                <span>{t("profileRuntimeKind")}</span>
                <select
                  value={normalizeRuntimeKind(profileDraft.runtime_kind || bootstrapConfig?.runtime_kind)}
                  onChange={(event) => onProfileDraftChange({ ...profileDraft, runtime_kind: event.target.value })}
                >
                  {GATEWAY_RUNTIME_KIND_OPTIONS.map((option) => (<option key={option.value} value={option.value}>{formatRuntimeKindLabel(option.value, t)}</option>))}
                </select>
              </label>
              <label className="field">
                <span>{t("profileProvider")}</span>
                <select
                  value={profileDraft.provider}
                  onChange={(event) => {
                    onProfileDraftChange({ ...profileDraft, provider: event.target.value, model_id: "" });
                    onProfileModelsReset();
                  }}
                >
                  {["csghub_lite", "codex", "claude_code", "api"].map((provider) => (
                    <option key={provider} value={provider}>{formatProviderLabel(provider)}</option>
                  ))}
                </select>
              </label>
              <label className="field">
                {requiredFieldLabel(t("profileModel"))}
                <select
                  value={profileDraft.model_id}
                  required
                  aria-required="true"
                  onChange={(event) => onProfileDraftChange({ ...profileDraft, model_id: event.target.value })}
                >
                  <option value="">{profileModelBusy ? t("profileLoadingModels") : t("profileSelectModel")}</option>
                  {profileModels.map((model) => (<option key={model} value={model}>{model}</option>))}
                  {profileDraft.model_id && !profileModels.includes(profileDraft.model_id)
                    ? (<option value={profileDraft.model_id}>{profileDraft.model_id}</option>)
                    : null}
                </select>
              </label>
              <label className="field">
                <span>{t("profileReasoning")}</span>
                <select
                  value={profileDraft.reasoning_effort}
                  onChange={(event) => onProfileDraftChange({ ...profileDraft, reasoning_effort: event.target.value })}
                >
                  {["low", "medium", "high", "xhigh"].map((effort) => (<option key={effort} value={effort}>{effort}</option>))}
                </select>
              </label>
              <label className="selection-item compact-toggle-row">
                <input
                  type="checkbox"
                  checked={profileDraft.enable_fast_mode}
                  onChange={() => onProfileDraftChange({ ...profileDraft, enable_fast_mode: !profileDraft.enable_fast_mode })}
                />
                <span>{t("profileFastMode")}</span>
              </label>
            </div>
            <CLIProxyAuthControl
              provider={profileDraft.provider}
              t={t}
              status={authStatuses[normalizeAuthProviderName(profileDraft.provider)]}
              busy={authBusyProvider === normalizeAuthProviderName(profileDraft.provider)}
              onLogin={onProviderLogin}
            />
          </section>
          {profileDraft.provider === "api"
            ? (
                <section className="profile-section">
                  <div className="profile-section-title">{t("profileAPIProvider")}</div>
                  <div className="profile-api-grid">
                    <label className="field">
                      {requiredFieldLabel(t("profileBaseURL"))}
                      <input
                        value={profileDraft.base_url}
                        required
                        aria-required="true"
                        onInput={(event) => onProfileDraftChange({ ...profileDraft, base_url: event.target.value })}
                        placeholder="https://api.openai.com/v1"
                      />
                    </label>
                    <APIKeyField
                      value={profileDraft.api_key}
                      onInput={(event) => onProfileDraftChange({ ...profileDraft, api_key: event.target.value })}
                      profile={profileDraft}
                      t={t}
                    />
                    <label className="field span-2">
                      <span>{t("profileHeaders")}</span>
                      <textarea className="compact-textarea" value={profileDraft.headersText} onInput={(event) => onProfileDraftChange({ ...profileDraft, headersText: event.target.value })} />
                    </label>
                  </div>
                </section>
              )
            : null}
          <section className="profile-section">
            <div className="profile-section-title">{t("profileAdvanced")}</div>
            <div className="profile-advanced-grid">
              <label className="field">
                <span>{t("profileRequestOptions")}</span>
                <textarea className="compact-json" value={profileDraft.requestOptionsText} onInput={(event) => onProfileDraftChange({ ...profileDraft, requestOptionsText: event.target.value })} />
              </label>
              <div className="field">
                <span>{t("profileEnv")}</span>
                <EnvKeyValueEditor
                  rows={profileDraft.envRows}
                  t={t}
                  onChange={(rows) => onProfileDraftChange({ ...profileDraft, envRows: rows })}
                />
              </div>
            </div>
          </section>
        </div>
        {profileError ? (<div className="form-error">{profileError}</div>) : null}
        <div className="modal-actions">
          <Button variant="primary" className="send-button" disabled={profileBusy || !profileDraft.model_id || profileBaseURLMissing(profileDraft)} onClick={onSave}>
            {profileBusy ? "..." : t("profileSave")}
          </Button>
        </div>
      </div>
    </div>
  );
}
