// @ts-nocheck
import { WORKSPACE_TAB_AGENTS, WORKSPACE_TAB_HUB, WORKSPACE_TAB_MESSAGES } from "@/bootstrap/constants";
import { Button } from "@/components/ui";
import { GlobeIcon, HubIcon, MoonIcon, RoomPlusIcon, RoomsIcon, SidebarToggleIcon, SunIcon, UsersIcon } from "@/components/ui/Icons";
import { formatSidebarVersionLabel } from "@/models/upgradeStatus";
import { localizeTemplateSourceTag } from "@/shared/i18n";
import { WorkspaceAgentRow, WorkspaceComputerRow, WorkspaceConversationRow, WorkspaceGroup } from "../WorkspaceRows";

export function WorkspaceSidebar({
  isSidebarCollapsed,
  onCollapseSidebar,
  onExpandSidebar,
  theme,
  onThemeChange,
  locale,
  onLocaleChange,
  t,
  currentWorkspaceLabel,
  runningAgentCount,
  agentItems,
  workspaceTab,
  onWorkspaceTabChange,
  roomCount,
  channels,
  directMessages,
  activePane,
  currentUserID,
  usersById,
  collapsedWorkspaceGroups,
  onToggleWorkspaceGroup,
  onCreateRoom,
  onCreateAgent,
  hubTemplates,
  hubError,
  hubLoaded,
  selectedHubTemplateId,
  onSelectHubTemplate,
  onSelectHub,
  agentsError,
  onSelectConversation,
  onPreviewUser,
  onSelectAgent,
  onPreviewAgent,
  onSelectComputer,
  appVersion,
  upgradeStatus,
  upgradeBusy,
  upgradePhase,
  upgradeError,
  onOpenUpgrade,
}) {
  return (
    <div className="sidebar-slot">
      <aside
        className={`sidebar ${isSidebarCollapsed ? "collapsed" : ""}`}
        aria-hidden={isSidebarCollapsed}
        inert={isSidebarCollapsed}
      >
        <div className="sidebar-header workspace-header">
          <div className="sidebar-brand-row">
            <div className="sidebar-brand-lockup" aria-label="CSGClaw">
              <div className="sidebar-brand-mark sidebar-brand-wordmark" aria-hidden="true">CSGClaw</div>
            </div>
            <div className="sidebar-controls">
              <div className="theme-switch" role="group" aria-label={t("themeSwitcher")}>
                <div className={`theme-switch-track ${theme === "dark" ? "is-dark" : "is-light"}`}>
                  <span className="theme-switch-thumb" aria-hidden="true"></span>
                  <Button
                    variant="ghost"
                    className="theme-toggle"
                    active={theme === "light"}
                    aria-label={t("themeLight")}
                    aria-pressed={theme === "light"}
                    title={t("themeLight")}
                    onClick={() => onThemeChange("light")}
                  >
                    <span aria-hidden="true"><SunIcon /></span>
                  </Button>
                  <Button
                    variant="ghost"
                    className="theme-toggle"
                    active={theme === "dark"}
                    aria-label={t("themeDark")}
                    aria-pressed={theme === "dark"}
                    title={t("themeDark")}
                    onClick={() => onThemeChange("dark")}
                  >
                    <span aria-hidden="true"><MoonIcon /></span>
                  </Button>
                </div>
              </div>
              <div className="language-switch sidebar-language-switch" role="group" aria-label={t("languageSwitcher")}>
                <span className="language-switch-icon" aria-hidden="true"><GlobeIcon /></span>
                <div className={`language-switch-track ${locale === "en" ? "is-en" : "is-zh"}`}>
                  <span className="language-switch-thumb" aria-hidden="true"></span>
                  <Button variant="ghost" className="language-toggle" active={locale === "zh"} aria-pressed={locale === "zh"} title={t("languageOptionZh")} onClick={() => onLocaleChange("zh")}>中</Button>
                  <Button variant="ghost" className="language-toggle" active={locale === "en"} aria-pressed={locale === "en"} title={t("languageOptionEn")} onClick={() => onLocaleChange("en")}>EN</Button>
                </div>
              </div>
              <Button
                variant="ghost"
                className="sidebar-toggle-button"
                aria-label={t("collapseSidebar")}
                title={t("collapseSidebar")}
                onClick={onCollapseSidebar}
              >
                <span className="sidebar-toggle-mark"><SidebarToggleIcon /></span>
              </Button>
            </div>
          </div>
          <div className="workspace-signal-panel" aria-label={currentWorkspaceLabel}>
            <div className="workspace-signal-copy">
              <span>{currentWorkspaceLabel}</span>
              <strong>{runningAgentCount}/{agentItems.length || 0} {t("activeNow")}</strong>
            </div>
            <div className="workspace-signal-meter" aria-hidden="true">
              <span></span>
              <span></span>
              <span></span>
            </div>
          </div>
        </div>
        <nav className="workspace-nav" aria-label="Workspace">
          <div className="workspace-tabbar" role="tablist" aria-label="Workspace sections">
            <Button
              className="workspace-tab"
              active={workspaceTab === WORKSPACE_TAB_MESSAGES}
              role="tab"
              aria-selected={workspaceTab === WORKSPACE_TAB_MESSAGES}
              aria-label={t("messagesTab")}
              title={t("messagesTab")}
              onClick={() => onWorkspaceTabChange(WORKSPACE_TAB_MESSAGES)}
            >
              <span className="workspace-tab-icon" aria-hidden="true"><RoomsIcon /></span>
              <span className="workspace-tab-copy">
                <strong>{t("messagesTab")}</strong>
                <small>{roomCount}</small>
              </span>
            </Button>
            <Button
              className="workspace-tab"
              active={workspaceTab === WORKSPACE_TAB_AGENTS}
              role="tab"
              aria-selected={workspaceTab === WORKSPACE_TAB_AGENTS}
              aria-label={t("agentsTab")}
              title={t("agentsTab")}
              onClick={() => onWorkspaceTabChange(WORKSPACE_TAB_AGENTS)}
            >
              <span className="workspace-tab-icon" aria-hidden="true"><UsersIcon /></span>
              <span className="workspace-tab-copy">
                <strong>{t("agentsTab")}</strong>
                <small>{agentItems.length}</small>
              </span>
            </Button>
            <Button
              className="workspace-tab"
              active={workspaceTab === WORKSPACE_TAB_HUB}
              role="tab"
              aria-selected={workspaceTab === WORKSPACE_TAB_HUB}
              aria-label={t("hubTab")}
              title={t("hubTab")}
              onClick={() => onSelectHub()}
            >
              <span className="workspace-tab-icon" aria-hidden="true"><HubIcon /></span>
              <span className="workspace-tab-copy">
                <strong>{t("hubTab")}</strong>
                <span className="workspace-tab-badge">{t("newBadge")}</span>
              </span>
            </Button>
          </div>
          {workspaceTab === WORKSPACE_TAB_MESSAGES
            ? (
                <div className="workspace-tab-panel" role="tabpanel" aria-label={t("messagesTab")}>
                  <WorkspaceGroup
                    id="rooms"
                    title={t("channelsSection")}
                    count={channels.length}
                    collapsed={Boolean(collapsedWorkspaceGroups.rooms)}
                    onToggle={() => onToggleWorkspaceGroup("rooms")}
                    onAdd={() => onCreateRoom()}
                    addLabel={t("createRoom")}
                  >
                    {channels.length
                      ? channels.map((conversation) => (
                          <WorkspaceConversationRow
                            key={conversation.id}
                            conversation={conversation}
                            active={activePane.type === "conversation" && activePane.id === conversation.id}
                            currentUserID={currentUserID}
                            usersById={usersById}
                            locale={locale}
                            t={t}
                            onSelect={onSelectConversation}
                            onPreviewUser={onPreviewUser}
                          />
                        ))
                      : (<div className="workspace-empty">{t("noChannels")}</div>)}
                  </WorkspaceGroup>
                  <WorkspaceGroup
                    id="direct-messages"
                    title={t("directMessagesSection")}
                    count={directMessages.length}
                    collapsed={Boolean(collapsedWorkspaceGroups["direct-messages"])}
                    onToggle={() => onToggleWorkspaceGroup("direct-messages")}
                  >
                    {directMessages.length
                      ? directMessages.map((conversation) => (
                          <WorkspaceConversationRow
                            key={conversation.id}
                            conversation={conversation}
                            active={activePane.type === "conversation" && activePane.id === conversation.id}
                            currentUserID={currentUserID}
                            usersById={usersById}
                            locale={locale}
                            t={t}
                            onSelect={onSelectConversation}
                            onPreviewUser={onPreviewUser}
                          />
                        ))
                      : (<div className="workspace-empty">{t("noDirectMessages")}</div>)}
                  </WorkspaceGroup>
                </div>
              )
            : workspaceTab === WORKSPACE_TAB_HUB
              ? (
                  <div className="workspace-tab-panel" role="tabpanel" aria-label={t("hubTab")}>
                    <WorkspaceGroup
                      id="hub"
                      title={t("hubTemplatesSection")}
                      count={hubTemplates.length}
                      collapsed={Boolean(collapsedWorkspaceGroups.hub)}
                      onToggle={() => onToggleWorkspaceGroup("hub")}
                    >
                      {hubError
                        ? (<div className="workspace-empty">{hubError}</div>)
                        : hubLoaded && hubTemplates.length === 0
                          ? (<div className="workspace-empty">{t("hubEmpty")}</div>)
                          : hubTemplates.slice(0, 6).map((item) => (
                              <button key={item.id} className={`workspace-row hub-template-row ${selectedHubTemplateId === item.id ? "active" : ""}`} onClick={() => onSelectHubTemplate(item)}>
                                <span className="workspace-row-icon"><HubIcon /></span>
                                <span className="workspace-row-main">
                                  <span className="workspace-row-title truncate">{item.name || item.id}</span>
                                  <span className="workspace-row-meta truncate">{item.description || item.source?.name || item.id}</span>
                                </span>
                                <span className="mini-badge template-source-badge"><span className="template-source-badge-dot" aria-hidden="true"></span>{localizeTemplateSourceTag(item.source?.name, locale)}</span>
                              </button>
                            ))}
                    </WorkspaceGroup>
                  </div>
                )
            : (
                <div className="workspace-tab-panel" role="tabpanel" aria-label={t("agentsTab")}>
                  <WorkspaceGroup
                    id="agents"
                    title={t("computerAgentsSection")}
                    count={agentItems.length}
                    collapsed={Boolean(collapsedWorkspaceGroups.agents)}
                    onToggle={() => onToggleWorkspaceGroup("agents")}
                    onAdd={onCreateAgent}
                    addLabel={t("createAgent")}
                  >
                    {agentItems.length
                      ? agentItems.map((item) => (
                          <WorkspaceAgentRow
                            key={item.id}
                            item={item}
                            active={activePane.type === "agent" && activePane.id === item.id}
                            t={t}
                            onSelect={onSelectAgent}
                            onPreview={onPreviewAgent}
                          />
                        ))
                      : (<div className="workspace-empty">{t("noAgents")}</div>)}
                  </WorkspaceGroup>
                  <WorkspaceGroup
                    id="computers"
                    title={t("computersSection")}
                    count={1}
                    collapsed={Boolean(collapsedWorkspaceGroups.computers)}
                    onToggle={() => onToggleWorkspaceGroup("computers")}
                  >
                    <WorkspaceComputerRow
                      title={t("localComputer")}
                      active={activePane.type === "computer"}
                      subtitle={`${agentItems.length} ${t("computerAgentsSection")}`}
                      onSelect={onSelectComputer}
                    />
                  </WorkspaceGroup>
                </div>
              )}
          {agentsError ? (<div className="form-error agent-error">{agentsError}</div>) : null}
        </nav>
        <div className="sidebar-footer">
          <div className="sidebar-footer-row">
            <span className="sidebar-version-label">{formatSidebarVersionLabel(appVersion)}</span>
            {upgradeStatus?.update_available || upgradeBusy || upgradeStatus?.upgrading || upgradePhase === "done" || upgradePhase === "error"
              ? (
                  <button
                    type="button"
                    className={`sidebar-upgrade-button ${upgradeBusy || upgradeStatus?.upgrading ? "is-running" : ""} ${upgradePhase === "done" ? "is-done" : ""}`}
                    onClick={onOpenUpgrade}
                  >
                    <span className="sidebar-upgrade-dot" aria-hidden="true"></span>
                    <span>{upgradePhase === "done" ? t("upgradeRefresh") : upgradeBusy || upgradeStatus?.upgrading ? t("upgradeBackground") : t("upgradeAction")}</span>
                  </button>
                )
              : null}
          </div>
          {upgradeError ? (<div className="sidebar-footer-error">{upgradeError}</div>) : null}
        </div>
      </aside>

      <div
        className={`sidebar-rail ${isSidebarCollapsed ? "visible" : ""}`}
        aria-hidden={!isSidebarCollapsed}
        inert={!isSidebarCollapsed}
      >
        <Button variant="ghost" className="sidebar-expand-button" aria-label={t("expandSidebar")} title={t("expandSidebar")} onClick={onExpandSidebar}>
          <span className="sidebar-toggle-mark"><SidebarToggleIcon /></span>
        </Button>
        <nav className="sidebar-rail-nav" aria-label="Workspace">
          <Button variant="ghost" className="sidebar-rail-button" active={workspaceTab === WORKSPACE_TAB_MESSAGES} aria-label={t("messagesTab")} title={t("messagesTab")} onClick={() => onWorkspaceTabChange(WORKSPACE_TAB_MESSAGES)}>
            <span className="sidebar-rail-icon" aria-hidden="true"><RoomsIcon /></span>
          </Button>
          <Button variant="ghost" className="sidebar-rail-button" active={workspaceTab === WORKSPACE_TAB_AGENTS} aria-label={t("agentsTab")} title={t("agentsTab")} onClick={() => onWorkspaceTabChange(WORKSPACE_TAB_AGENTS)}>
            <span className="sidebar-rail-icon" aria-hidden="true"><UsersIcon /></span>
          </Button>
          <Button variant="ghost" className="sidebar-rail-button" active={workspaceTab === WORKSPACE_TAB_HUB} aria-label={t("hubTab")} title={t("hubTab")} onClick={() => onSelectHub()}>
            <span className="sidebar-rail-icon" aria-hidden="true"><HubIcon /></span>
          </Button>
          <Button variant="ghost" className="sidebar-rail-button" aria-label={t("createRoom")} title={t("createRoom")} onClick={() => onCreateRoom()}>
            <span className="sidebar-rail-icon" aria-hidden="true"><RoomPlusIcon /></span>
          </Button>
        </nav>
      </div>
    </div>
  );
}
