// @ts-nocheck
import React, { useCallback, useEffect, useLayoutEffect, useMemo, useRef, useState } from "react";
import { useQueryClient } from "@tanstack/react-query";
import { applyUpgradeRequest } from "@/api/upgrade";
import { createBotRequest, createManagerAgentRequest, deleteBotRequest, fetchAgentProfile, fetchAgentProfileDefaults, fetchAgentProfileModels, fetchAgents, fetchManagerProfile, runAgentActionRequest, saveManagerProfileRequest, updateAgentRequest } from "@/api/agents";
import { fetchVersion } from "@/api/app";
import { fetchCLIProxyAuthStatus as fetchCLIProxyAuthStatusRequest, loginCLIProxyProviderRequest } from "@/api/cliproxy";
import { fetchHubTemplate, fetchHubTemplates as fetchHubTemplatesRequest, fetchHubWorkspaceFile, publishAgentTemplateRequest } from "@/api/hub";
import { createRoomRequest, createUserRequest, deleteRoomRequest, inviteRoomUsersRequest, joinAgentToRoomRequest, sendMessageRequest } from "@/api/im";
import { ACTION_REBUILD_MANAGER, MESSAGE_LIST_BOTTOM_THRESHOLD, WORKSPACE_TAB_AGENTS, WORKSPACE_TAB_HUB, WORKSPACE_TAB_MESSAGES } from "@/bootstrap/constants";
import { applyTemplateToDraft, advanceAgentProgress, agentToDraft, draftNotifierRuntimeOptionsForSave, draftToProfile, ensureNotifierPullSubscriptionDraft, isAgentRunning, isManagerAgent, isNotifierRuntimeDraft, isNotifierRuntimeDraftOnAgentPage, modelRequestKey, normalizeAuthProviderName, normalizeRuntimeKind, normalizeTemplateSelection, parseJSONMap, pickDefaultAgentTemplate, profileToDraft, providerNeedsAuth, runtimeImageForKind, startAgentCreateProgress } from "@/models/agents";
import { agentMatchesUser, appendMessageToData, applyIMEvent, isDirectConversation, isToolCallMessage, removeConversationFromData, upsertConversationInData } from "@/models/conversations";
import { areComposerSegmentsEqual, getComposerMentionState, insertComposerLineBreak, parseComposerSegments, placeCaretAtEnd, removeAdjacentMentionToken, renderComposerSegments, replaceMentionQueryWithToken, segmentsToPlainText, serializeComposerSegments, updateDrafts } from "@/models/composer";
import { paneFromLocation, syncBrowserPath, workspaceTabForPane } from "@/models/routing";
import { AgentDetailPane, AgentProfileModal, ComputerDetailPane, ConversationPane, CreateRoomModal, HubDetailPane, InviteMembersModal, ManagerProfileSetupModal, ProfilePreviewPopover, UpgradeModal, WorkspaceSidebar } from "./components";
import { normalizeUpgradeStatus } from "@/models/upgradeStatus";
import { createTranslator, localizeError } from "@/shared/i18n";
import { messages } from "@/shared/i18n/messages";
import { initializeMermaidTheme } from "@/components/business/MessageContent";
import { subscribeIMEvents } from "@/shared/realtime/imEvents";
import { LOCALE_STORAGE_KEY, SIDEBAR_COLLAPSED_STORAGE_KEY, THEME_STORAGE_KEY, WORKSPACE_GROUPS_COLLAPSED_STORAGE_KEY } from "@/shared/storage/keys";
import { errorMessage } from "@/api/client";
import { useWorkspaceUiStore } from "./workspaceUiStore";
import {
  fetchWorkspaceBootstrapData,
  fetchWorkspaceUpgradeStatus,
  useWorkspaceAgentsQuery,
  useWorkspaceAppVersionQuery,
  useWorkspaceBootstrapConfigQuery,
  useWorkspaceBootstrapQuery,
  useWorkspaceHubTemplatesQuery,
  useWorkspaceManagerProfileQuery,
  useWorkspaceUpgradeStatusQuery,
  workspaceQueryKeys,
} from "./workspaceQueries";


function WorkspacePage() {
  const queryClient = useQueryClient();
  const locale = useWorkspaceUiStore((state) => state.locale);
  const setLocale = useWorkspaceUiStore((state) => state.setLocale);
  const theme = useWorkspaceUiStore((state) => state.theme);
  const setTheme = useWorkspaceUiStore((state) => state.setTheme);
  const showToolCalls = useWorkspaceUiStore((state) => state.showToolCalls);
  const setShowToolCalls = useWorkspaceUiStore((state) => state.setShowToolCalls);
  const isSidebarCollapsed = useWorkspaceUiStore((state) => state.isSidebarCollapsed);
  const setIsSidebarCollapsed = useWorkspaceUiStore((state) => state.setIsSidebarCollapsed);
  const workspaceTab = useWorkspaceUiStore((state) => state.workspaceTab);
  const setWorkspaceTab = useWorkspaceUiStore((state) => state.setWorkspaceTab);
  const collapsedWorkspaceGroups = useWorkspaceUiStore((state) => state.collapsedWorkspaceGroups);
  const setCollapsedWorkspaceGroups = useWorkspaceUiStore((state) => state.setCollapsedWorkspaceGroups);
  const activeConversationId = useWorkspaceUiStore((state) => state.activeConversationId);
  const setActiveConversationId = useWorkspaceUiStore((state) => state.setActiveConversationId);
  const activePane = useWorkspaceUiStore((state) => state.activePane);
  const setActivePane = useWorkspaceUiStore((state) => state.setActivePane);
  const bootstrapQuery = useWorkspaceBootstrapQuery();
  const bootstrapConfigQuery = useWorkspaceBootstrapConfigQuery();
  const managerProfileQuery = useWorkspaceManagerProfileQuery();
  const agentsQuery = useWorkspaceAgentsQuery();
  const hubTemplatesQuery = useWorkspaceHubTemplatesQuery();
  const appVersionQuery = useWorkspaceAppVersionQuery();
  const upgradeStatusQuery = useWorkspaceUpgradeStatusQuery();
  const data = bootstrapQuery.data ?? null;
  const bootstrapConfig = bootstrapConfigQuery.data ?? null;
  const managerProfile = managerProfileQuery.data ?? null;
  const agents = agentsQuery.data ?? [];
  const agentsLoaded = agentsQuery.isFetched;
  const hubTemplates = hubTemplatesQuery.data ?? [];
  const hubLoaded = hubTemplatesQuery.isFetched;
  const appVersion = appVersionQuery.data ?? "dev";
  const upgradeStatus = upgradeStatusQuery.data ?? null;
  const [draftsByConversationId, setDraftsByConversationId] = useState({});
  const [composerMentionState, setComposerMentionState] = useState(null);
  const [mentionIndex, setMentionIndex] = useState(0);
  const [showCreateRoom, setShowCreateRoom] = useState(false);
  const [showInvite, setShowInvite] = useState(false);
  const [showMemberList, setShowMemberList] = useState(false);
  const [showChannelTools, setShowChannelTools] = useState(false);
  const [roomTitle, setRoomTitle] = useState("");
  const [roomDescription, setRoomDescription] = useState("");
  const [roomMemberIDs, setRoomMemberIDs] = useState([]);
  const [lockedRoomMemberIDs, setLockedRoomMemberIDs] = useState([]);
  const [inviteUserIDs, setInviteUserIDs] = useState([]);
  const [submitError, setSubmitError] = useState("");
  const [composerError, setComposerError] = useState("");
  const [profileDraft, setProfileDraft] = useState(null);
  const [profileModels, setProfileModels] = useState([]);
  const [profileError, setProfileError] = useState("");
  const [profileBusy, setProfileBusy] = useState(false);
  const [profileModelBusy, setProfileModelBusy] = useState(false);
  const [cliproxyAuthStatuses, setCLIProxyAuthStatuses] = useState({});
  const [cliproxyAuthBusy, setCLIProxyAuthBusy] = useState("");
  const [agentsError, setAgentsError] = useState("");
  const [hubManualError, setHubManualError] = useState("");
  const [selectedHubTemplateId, setSelectedHubTemplateId] = useState("");
  const [hubTemplateDetail, setHubTemplateDetail] = useState(null);
  const [hubTemplateDetailLoading, setHubTemplateDetailLoading] = useState(false);
  const [hubTemplateDetailError, setHubTemplateDetailError] = useState("");
  const [hubWorkspaceFile, setHubWorkspaceFile] = useState(null);
  const [hubWorkspaceFileLoading, setHubWorkspaceFileLoading] = useState(false);
  const [hubWorkspaceFileError, setHubWorkspaceFileError] = useState("");
  const [selectedHubWorkspacePath, setSelectedHubWorkspacePath] = useState("");
  const [showAgentModal, setShowAgentModal] = useState(false);
  const [agentModalMode, setAgentModalMode] = useState("create");
  const [editingAgent, setEditingAgent] = useState(null);
  const [agentDraft, setAgentDraft] = useState(null);
  const [agentModels, setAgentModels] = useState([]);
  const [agentBusy, setAgentBusy] = useState(false);
  const [agentModelBusy, setAgentModelBusy] = useState(false);
  const [agentError, setAgentError] = useState("");
  const [agentProgress, setAgentProgress] = useState(null);
  const [agentActionBusy, setAgentActionBusy] = useState("");
  const [messageActionBusy, setMessageActionBusy] = useState("");
  const [messageActionError, setMessageActionError] = useState({ key: "", message: "" });
  const [agentPageDraft, setAgentPageDraft] = useState(null);
  const [agentPageModels, setAgentPageModels] = useState([]);
  const [agentPageBusy, setAgentPageBusy] = useState(false);
  const [agentPagePublishBusy, setAgentPagePublishBusy] = useState(false);
  const [agentPageModelBusy, setAgentPageModelBusy] = useState(false);
  const [agentPageError, setAgentPageError] = useState("");
  const [notifierModalWebhookOrigin, setNotifierModalWebhookOrigin] = useState("");
  const [notifierPageWebhookOrigin, setNotifierPageWebhookOrigin] = useState("");
  const [profilePreview, setProfilePreview] = useState(null);
  const [upgradeBusy, setUpgradeBusy] = useState(false);
  const [upgradeError, setUpgradeError] = useState("");
  const [showUpgradeModal, setShowUpgradeModal] = useState(false);
  const [upgradePhase, setUpgradePhase] = useState("idle");
  const editorRef = useRef(null);
  const messageListRef = useRef(null);
  const memberMenuRef = useRef(null);
  const channelToolsRef = useRef(null);
  const profilePreviewRef = useRef(null);
  const agentRefreshTimerRef = useRef(null);
  const profileDraftRef = useRef(null);
  const agentDraftRef = useRef(null);
  const agentPageDraftRef = useRef(null);
  const upgradePollTimerRef = useRef(null);
  const shouldAutoScrollRef = useRef(true);
  const autoScrollConversationRef = useRef(activeConversationId);

  const setBootstrapData = useCallback((value) => {
    queryClient.setQueryData(workspaceQueryKeys.bootstrap(), (current) => (
      typeof value === "function" ? value(current ?? null) : value
    ));
  }, [queryClient]);

  const setManagerProfileData = useCallback((value) => {
    queryClient.setQueryData(workspaceQueryKeys.managerProfile(), (current) => (
      typeof value === "function" ? value(current ?? null) : value
    ));
  }, [queryClient]);

  const setAgentsData = useCallback((value) => {
    queryClient.setQueryData(workspaceQueryKeys.agents(), (current) => (
      typeof value === "function" ? value(current ?? []) : value
    ));
  }, [queryClient]);

  const setHubTemplatesData = useCallback((value) => {
    queryClient.setQueryData(workspaceQueryKeys.hubTemplates(), (current) => (
      typeof value === "function" ? value(current ?? []) : value
    ));
  }, [queryClient]);

  const setAppVersionData = useCallback((value) => {
    queryClient.setQueryData(workspaceQueryKeys.appVersion(), (current) => (
      typeof value === "function" ? value(current ?? "dev") : value
    ));
  }, [queryClient]);

  const setUpgradeStatusData = useCallback((value) => {
    queryClient.setQueryData(workspaceQueryKeys.upgradeStatus(), (current) => (
      typeof value === "function" ? value(current ?? null) : value
    ));
  }, [queryClient]);

  useEffect(() => {
    return () => {
      if (upgradePollTimerRef.current) {
        window.clearInterval(upgradePollTimerRef.current);
        upgradePollTimerRef.current = null;
      }
    };
  }, []);

  useEffect(() => {
    if (!bootstrapConfig?.runtime_kind) {
      return;
    }
    setProfileDraft((current) => current && !current.runtime_kind
      ? { ...current, runtime_kind: normalizeRuntimeKind(bootstrapConfig.runtime_kind) }
      : current);
  }, [bootstrapConfig?.runtime_kind]);

  useEffect(() => {
    if (!agentBusy || !agentProgress?.steps?.length) {
      return undefined;
    }
    const timer = window.setInterval(() => {
      setAgentProgress((current) => advanceAgentProgress(current));
    }, 1200);
    return () => window.clearInterval(timer);
  }, [agentBusy, agentProgress?.startedAt]);

  useEffect(() => {
    // Temporarily disable background agent polling for debugging multi-tab pending requests.
    // function refreshVisibleAgents() {
    //   if (document.visibilityState === "visible") {
    //     refreshAgents({ silent: true });
    //   }
    // }
    //
    // const intervalID = window.setInterval(refreshVisibleAgents, AGENT_STATUS_REFRESH_INTERVAL_MS);
    // document.addEventListener("visibilitychange", refreshVisibleAgents);
    return () => {
      // window.clearInterval(intervalID);
      // document.removeEventListener("visibilitychange", refreshVisibleAgents);
    };
  }, []);

  useEffect(() => {
    const unsubscribe = subscribeIMEvents((payload) => {
      setBootstrapData((current) => applyIMEvent(current, payload));
      if (payload?.type === "upgrade.status_changed" && payload.upgrade) {
        const next = normalizeUpgradeStatus(payload.upgrade);
        setUpgradeStatusData(next);
        if (next?.upgrading) {
          setUpgradeBusy(true);
          setUpgradePhase((phase) => phase === "done" ? phase : "restarting");
        } else if (!next?.update_available) {
          setUpgradeBusy(false);
        }
      }
      // Temporarily disable event-driven agent polling for debugging multi-tab pending requests.
      // if (isAgentRosterEvent(payload)) {
      //   scheduleAgentsRefresh();
      // }
    });

    return () => {
      unsubscribe();
      if (agentRefreshTimerRef.current) {
        window.clearTimeout(agentRefreshTimerRef.current);
        agentRefreshTimerRef.current = null;
      }
    };
  }, []);

  useEffect(() => {
    document.documentElement.lang = locale === "zh" ? "zh-CN" : "en";
    document.title = messages[locale].pageTitle;
    window.localStorage.setItem(LOCALE_STORAGE_KEY, locale);
  }, [locale]);

  useEffect(() => {
    document.documentElement.dataset.theme = theme;
    document.documentElement.style.colorScheme = theme;
    window.localStorage.setItem(THEME_STORAGE_KEY, theme);
    initializeMermaidTheme(theme);
  }, [theme]);

  useEffect(() => {
    window.localStorage.setItem(SIDEBAR_COLLAPSED_STORAGE_KEY, String(isSidebarCollapsed));
  }, [isSidebarCollapsed]);

  useEffect(() => {
    window.localStorage.setItem(WORKSPACE_GROUPS_COLLAPSED_STORAGE_KEY, JSON.stringify(collapsedWorkspaceGroups));
  }, [collapsedWorkspaceGroups]);

  useEffect(() => {
    profileDraftRef.current = profileDraft;
  }, [profileDraft]);

  useEffect(() => {
    agentDraftRef.current = agentDraft;
  }, [agentDraft]);

  useEffect(() => {
    agentPageDraftRef.current = agentPageDraft;
  }, [agentPageDraft]);

  useEffect(() => {
    if (!showAgentModal || !agentDraft || !isNotifierRuntimeDraftOnAgentPage(agentDraft, editingAgent)) {
      return;
    }
    setNotifierModalWebhookOrigin(typeof window !== "undefined" ? window.location.origin : "");
  }, [showAgentModal, agentDraft?.runtime_kind, editingAgent?.id]);

  useEffect(() => {
    setWorkspaceTab(workspaceTabForPane(activePane));
  }, [activePane?.type]);

  useEffect(() => {
    if (hubTemplatesQuery.isSuccess) {
      setHubManualError("");
    }
  }, [hubTemplatesQuery.isSuccess, hubTemplatesQuery.dataUpdatedAt]);

  useEffect(() => {
    function handlePopState() {
      const next = paneFromLocation();
      setActivePane(next);
      if (next.type === "conversation") {
        setActiveConversationId(next.id);
      }
      setShowMemberList(false);
    }

    window.history.replaceState({ pane: activePane }, "", window.location.pathname);
    window.addEventListener("popstate", handlePopState);
    return () => window.removeEventListener("popstate", handlePopState);
  }, []);

  const t = useMemo(() => createTranslator(locale), [locale]);
  const loadingError = bootstrapQuery.isError ? t("loadingFailed") : "";
  const hubError = hubManualError || (hubTemplatesQuery.isError ? t("hubLoadFailed") : "");
  const agentsDisplayError = agentsError || (agentsQuery.isError ? errorMessage(agentsQuery.error, t("agentActionFailed")) : "");
  const managerProfileIncomplete = managerProfile && managerProfile.profile_complete === false;

  useEffect(() => {
    if (!managerProfile) {
      return;
    }
    setProfileDraft({
      ...profileToDraft(managerProfile),
      runtime_kind: normalizeRuntimeKind(bootstrapConfig?.runtime_kind || managerProfile.runtime_kind),
    });
  }, [managerProfile, bootstrapConfig?.runtime_kind]);

  const usersById = useMemo(() => {
    const result = new Map();
    data?.users.forEach((user) => result.set(user.id, user));
    return result;
  }, [data]);

  const activeConversation = useMemo(
    () => data?.rooms.find((item) => item.id === activeConversationId) ?? null,
    [data, activeConversationId],
  );

  const visibleMessages = useMemo(() => {
    if (!activeConversation) {
      return [];
    }
    if (showToolCalls) {
      return activeConversation.messages;
    }
    return activeConversation.messages.filter((message) => !isToolCallMessage(message.content));
  }, [activeConversation, showToolCalls]);

  const rooms = useMemo(
    () => data?.rooms ?? [],
    [data],
  );
  const roomCount = rooms.length;
  const channels = useMemo(
    () => rooms.filter((room) => !isDirectConversation(room)),
    [rooms],
  );
  const directMessages = useMemo(
    () => rooms.filter((room) => isDirectConversation(room)),
    [rooms],
  );
  const selectedAgentForPage = useMemo(() => {
    if (activePane.type !== "agent") {
      return null;
    }
    const managerAgent = agents.find((item) => item.role === "manager" || item.id === "u-manager");
    const workerAgents = agents.filter((item) => item.id !== managerAgent?.id);
    return [managerAgent, ...workerAgents].filter(Boolean).find((item) => item.id === activePane.id) ?? null;
  }, [agents, activePane]);

  useEffect(() => {
    if (!agentPageDraft || !isNotifierRuntimeDraftOnAgentPage(agentPageDraft, selectedAgentForPage)) {
      return;
    }
    setNotifierPageWebhookOrigin(typeof window !== "undefined" ? window.location.origin : "");
  }, [agentPageDraft?.runtime_kind, selectedAgentForPage?.id]);

  const mentionCandidates = useMemo(() => {
    if (!data || !composerMentionState) {
      return [];
    }
    const allowed = new Set(activeConversation?.members ?? []);
    return data.users
      .filter((user) => allowed.has(user.id))
      .filter((user) => user.handle.toLowerCase().includes(composerMentionState.query.toLowerCase()) || user.name.toLowerCase().includes(composerMentionState.query.toLowerCase()))
      .slice(0, 5);
  }, [data, activeConversation, composerMentionState]);
  const mentionableUsersByHandle = useMemo(() => {
    const result = new Map();
    if (!data) {
      return result;
    }
    const allowed = new Set(activeConversation?.members ?? []);
    data.users
      .filter((user) => allowed.has(user.id))
      .forEach((user) => {
        const handle = String(user.handle ?? "").trim().toLowerCase();
        if (handle && !result.has(handle)) {
          result.set(handle, user);
        }
      });
    return result;
  }, [data, activeConversation]);

  const draftSegments = useMemo(
    () => draftsByConversationId[activeConversationId] ?? [],
    [draftsByConversationId, activeConversationId],
  );
  const draftText = useMemo(() => segmentsToPlainText(draftSegments), [draftSegments]);

  useEffect(() => {
    setMentionIndex(0);
  }, [activeConversationId, composerMentionState?.query, draftText]);

  useEffect(() => {
    if (!showCreateRoom) {
      setRoomTitle("");
      setRoomDescription("");
      setRoomMemberIDs([]);
      setLockedRoomMemberIDs([]);
      setSubmitError("");
    }
  }, [showCreateRoom]);

  useEffect(() => {
    if (!showInvite) {
      setInviteUserIDs([]);
      setSubmitError("");
    }
  }, [showInvite]);

  useEffect(() => {
    setShowMemberList(false);
    setShowChannelTools(false);
  }, [activeConversationId]);

  useEffect(() => {
    if (!showMemberList) {
      return undefined;
    }

    function handlePointerDown(event) {
      const menu = memberMenuRef.current;
      if (!menu || menu.contains(event.target)) {
        return;
      }
      setShowMemberList(false);
    }

    document.addEventListener("mousedown", handlePointerDown);
    return () => document.removeEventListener("mousedown", handlePointerDown);
  }, [showMemberList]);

  useEffect(() => {
    if (!showChannelTools) {
      return undefined;
    }

    function handlePointerDown(event) {
      const menu = channelToolsRef.current;
      if (!menu || menu.contains(event.target)) {
        return;
      }
      setShowChannelTools(false);
    }

    document.addEventListener("mousedown", handlePointerDown);
    return () => document.removeEventListener("mousedown", handlePointerDown);
  }, [showChannelTools]);

  useEffect(() => {
    if (!profilePreview) {
      return undefined;
    }

    function handlePointerDown(event) {
      const preview = profilePreviewRef.current;
      const anchor = profilePreview?.anchorEl;
      if (!preview || preview.contains(event.target) || anchor?.contains?.(event.target)) {
        return;
      }
      closeProfilePreview();
    }

    function handleViewportChange() {
      closeProfilePreview();
    }

    document.addEventListener("mousedown", handlePointerDown);
    window.addEventListener("resize", handleViewportChange);
    window.addEventListener("scroll", handleViewportChange, true);
    return () => {
      document.removeEventListener("mousedown", handlePointerDown);
      window.removeEventListener("resize", handleViewportChange);
      window.removeEventListener("scroll", handleViewportChange, true);
    };
  }, [profilePreview]);

  useEffect(() => {
    if (!data) {
      return;
    }
    if (!activeConversationId) {
      if (data.rooms.length > 0) {
        setActiveConversationId(data.rooms[0].id);
        if (!activePane.id) {
          const next = { type: "conversation", id: data.rooms[0].id };
          setActivePane(next);
          syncBrowserPath(next, data.rooms, "replace");
        }
      } else {
        if (!activePane.id) {
          const next = { type: "computer", id: "local" };
          setActivePane(next);
          syncBrowserPath(next, data.rooms, "replace");
        }
      }
      return;
    }
    if (!data.rooms.some((room) => room.id === activeConversationId)) {
      const nextID = data.rooms[0]?.id ?? "";
      if (nextID) {
        selectConversation(nextID, { replace: true });
      } else {
        setActiveConversationId("");
        selectComputer({ replace: true });
      }
    }
  }, [data, activeConversationId, activePane.id]);

  useEffect(() => {
    if (!activePane || activePane.type !== "agent") {
      return;
    }
    if (!agentsLoaded) {
      return;
    }
    if (!agents.some((item) => item.id === activePane.id)) {
      if (activeConversationId) {
        selectConversation(activeConversationId, { replace: true });
      } else {
        selectComputer({ replace: true });
      }
    }
  }, [agents, agentsLoaded, activePane, activeConversationId]);

  useEffect(() => {
    if (!data || !activePane?.id) {
      return;
    }
    syncBrowserPath(activePane, rooms, "replace");
  }, [data, activePane?.type, activePane?.id, rooms]);

  useEffect(() => {
    if (!showAgentModal || !agentDraft?.provider || isNotifierRuntimeDraft(agentDraft)) {
      return undefined;
    }
    const timer = window.setTimeout(() => loadAgentModels(agentDraft, { silent: true }), agentDraft.provider === "api" ? 420 : 0);
    return () => window.clearTimeout(timer);
  }, [showAgentModal, agentDraft?.provider, agentDraft?.base_url, agentDraft?.api_key, agentDraft?.headersText]);

  useEffect(() => {
    if (!selectedAgentForPage) {
      setAgentPageDraft(null);
      setAgentPageModels([]);
      setAgentPageError("");
      setAgentPagePublishBusy(false);
      return;
    }
    loadAgentPageDraft(selectedAgentForPage);
  }, [selectedAgentForPage?.id]);

  useEffect(() => {
    if (!hubTemplates.length) {
      setSelectedHubTemplateId("");
      setHubTemplateDetail(null);
      setHubTemplateDetailError("");
      setSelectedHubWorkspacePath("");
      setHubWorkspaceFile(null);
      setHubWorkspaceFileError("");
      return;
    }
    setSelectedHubTemplateId((current) => hubTemplates.some((item) => item.id === current) ? current : hubTemplates[0].id);
  }, [hubTemplates]);

  useEffect(() => {
    if (!selectedHubTemplateId) {
      setHubTemplateDetail(null);
      setHubTemplateDetailLoading(false);
      setHubTemplateDetailError("");
      setSelectedHubWorkspacePath("");
      setHubWorkspaceFile(null);
      setHubWorkspaceFileLoading(false);
      setHubWorkspaceFileError("");
      return;
    }
    loadHubTemplateDetail(selectedHubTemplateId);
  }, [selectedHubTemplateId]);

  useEffect(() => {
    if (activePane.type !== "agent" || !agentPageDraft?.provider || isNotifierRuntimeDraft(agentPageDraft)) {
      return undefined;
    }
    const timer = window.setTimeout(() => loadAgentPageModels(agentPageDraft, { silent: true }), agentPageDraft.provider === "api" ? 420 : 0);
    return () => window.clearTimeout(timer);
  }, [activePane.type, activePane.id, agentPageDraft?.provider, agentPageDraft?.base_url, agentPageDraft?.api_key, agentPageDraft?.headersText]);

  useEffect(() => {
    if (!managerProfileIncomplete || !profileDraft?.provider) {
      return undefined;
    }
    const timer = window.setTimeout(() => loadProfileModels(profileDraft, { silent: true }), profileDraft.provider === "api" ? 420 : 0);
    return () => window.clearTimeout(timer);
  }, [managerProfileIncomplete, profileDraft?.provider, profileDraft?.base_url, profileDraft?.api_key, profileDraft?.headersText]);

  useEffect(() => {
    refreshCLIProxyAuthStatus(managerProfile?.provider);
  }, [managerProfile?.provider]);

  useEffect(() => {
    refreshCLIProxyAuthStatus(profileDraft?.provider);
  }, [profileDraft?.provider]);

  useEffect(() => {
    if (!isNotifierRuntimeDraft(agentDraft)) {
      refreshCLIProxyAuthStatus(agentDraft?.provider);
    }
  }, [agentDraft?.provider, agentDraft?.runtime_kind]);

  useEffect(() => {
    if (!isNotifierRuntimeDraft(agentPageDraft)) {
      refreshCLIProxyAuthStatus(agentPageDraft?.provider);
    }
  }, [agentPageDraft?.provider, agentPageDraft?.runtime_kind]);

  useEffect(() => {
    const el = messageListRef.current;
    if (!el) {
      return;
    }
    const updateAutoScrollState = () => {
      const distanceFromBottom = el.scrollHeight - el.scrollTop - el.clientHeight;
      shouldAutoScrollRef.current = distanceFromBottom <= MESSAGE_LIST_BOTTOM_THRESHOLD;
    };
    updateAutoScrollState();
    el.addEventListener("scroll", updateAutoScrollState);
    return () => el.removeEventListener("scroll", updateAutoScrollState);
  }, [activeConversationId]);

  useLayoutEffect(() => {
    if (activePane.type !== "conversation") {
      return;
    }
    const el = messageListRef.current;
    if (!el) {
      return;
    }
    autoScrollConversationRef.current = activeConversationId;
    el.scrollTop = el.scrollHeight;
    shouldAutoScrollRef.current = true;
  }, [activePane.type, activeConversationId]);

  useEffect(() => {
    const el = messageListRef.current;
    if (autoScrollConversationRef.current !== activeConversationId) {
      autoScrollConversationRef.current = activeConversationId;
      shouldAutoScrollRef.current = false;
      return;
    }
    if (!el || !shouldAutoScrollRef.current) {
      return;
    }
    el.scrollTop = el.scrollHeight;
  }, [visibleMessages.length]);

  useEffect(() => {
    const editor = editorRef.current;
    if (!editor) {
      return;
    }
    if (!areComposerSegmentsEqual(parseComposerSegments(editor), draftSegments)) {
      renderComposerSegments(editor, draftSegments);
    }
    setComposerMentionState(null);
  }, [activeConversationId]);

  useEffect(() => {
    if (!activeConversationId || showCreateRoom || showInvite) {
      return;
    }
    const editor = editorRef.current;
    if (!editor) {
      return;
    }
    requestAnimationFrame(() => {
      if (editorRef.current !== editor) {
        return;
      }
      editor.focus();
      placeCaretAtEnd(editor);
    });
  }, [activeConversationId, showCreateRoom, showInvite]);

  async function refreshBootstrap() {
    try {
      const normalized = await fetchWorkspaceBootstrapData();
      setBootstrapData(normalized);
      setInviteUserIDs([]);
      if (!activeConversationId && normalized.rooms.length > 0) {
        if (activePane.id && activePane.type !== "conversation") {
          setActiveConversationId(normalized.rooms[0].id);
        } else {
          selectConversation(normalized.rooms[0].id, { replace: true, rooms: normalized.rooms });
        }
      }
      return normalized;
    } catch (_) {
      return null;
    }
  }

  async function refreshUpgradeStatus() {
    try {
      const payload = await fetchWorkspaceUpgradeStatus();
      setUpgradeStatusData(payload);
      if (payload?.upgrading) {
        setUpgradeBusy(true);
        setUpgradePhase((phase) => phase === "done" ? phase : "restarting");
      } else if (!payload?.update_available) {
        setUpgradeBusy(false);
      }
      return payload;
    } catch (_) {
      setUpgradeStatusData(null);
      setUpgradeBusy(false);
      return null;
    }
  }

  function stopUpgradePoll() {
    if (upgradePollTimerRef.current) {
      window.clearInterval(upgradePollTimerRef.current);
      upgradePollTimerRef.current = null;
    }
  }

  function startUpgradeReconnectPoll(expectedVersion) {
    stopUpgradePoll();
    let attempts = 0;
    const poll = async () => {
      attempts += 1;
      try {
        const payload = await fetchVersion({ cacheBust: true });
        const version = typeof payload?.version === "string" ? payload.version.trim() : "";
        const expected = (expectedVersion || "").trim();
        if (version && (!expected || version === expected)) {
          stopUpgradePoll();
          setAppVersionData(version);
          setUpgradeBusy(false);
          setUpgradePhase("done");
          setUpgradeStatusData((current) => ({
            ...(current || {}),
            current_version: version,
            latest_version: version,
            update_available: false,
            checking: false,
            upgrading: false,
            last_error: "",
          }));
          return;
        }
      } catch (_) {
        // The daemon is expected to be unavailable while the upgrade helper restarts it.
      }
      if (attempts >= 60) {
        stopUpgradePoll();
        setUpgradeBusy(false);
        setUpgradePhase("error");
        const latest = await refreshUpgradeStatus();
        const detail = latest?.last_error ? ` ${latest.last_error}` : "";
        setUpgradeError(`${t("upgradeApplyFailed")}${detail}`);
      }
    };
    poll();
    upgradePollTimerRef.current = window.setInterval(poll, 2000);
  }

  async function applyUpgrade() {
    if (upgradeBusy || upgradeStatus?.upgrading) {
      return;
    }

    setUpgradeBusy(true);
    setUpgradeError("");
    setUpgradePhase("starting");
    setShowUpgradeModal(true);
    try {
      await applyUpgradeRequest();
      setUpgradePhase("restarting");
      setUpgradeStatusData((current) => ({
        ...(current || {}),
        upgrading: true,
        last_error: "",
      }));
      startUpgradeReconnectPoll(upgradeStatus?.latest_version);
      setShowUpgradeModal(false);
    } catch (err) {
      setUpgradeBusy(false);
      setUpgradePhase("error");
      const detail = err?.message && err.message !== "upgrade apply failed" ? ` ${err.message}` : "";
      setUpgradeError(`${t("upgradeApplyFailed")}${detail}`);
    }
  }

  async function sendMessage() {
    if (managerProfileIncomplete) {
      setComposerError(t("profileIncomplete"));
      return;
    }
    const managerProvider = normalizeAuthProviderName(managerProfile?.provider);
    if (providerNeedsAuth(managerProvider) && cliproxyAuthStatuses[managerProvider]?.authenticated === false) {
      setComposerError(t("authRequired"));
      return;
    }
    if (!data || !activeConversation || !draftText.trim()) {
      return;
    }

    setComposerError("");
    try {
      const created = await sendMessageRequest({
        room_id: activeConversation.id,
        sender_id: data.current_user_id,
        content: serializeComposerSegments(draftSegments),
      });
      setBootstrapData((current) => appendMessageToData(current, activeConversation.id, created));
      clearComposer();
    } catch (_) {
      setComposerError(t("sendFailed"));
    }
  }

  async function createRoom() {
    if (!data || !roomTitle.trim()) {
      return;
    }

    setSubmitError("");
    const memberIDs = roomMemberIDs.filter((id) => id && id !== data.current_user_id);
    try {
      const created = await createRoomRequest({
        title: roomTitle,
        description: roomDescription,
        creator_id: data.current_user_id,
        member_ids: memberIDs,
        locale,
      });
      setBootstrapData((current) => upsertConversationInData(current, created));
      selectConversation(created.id);
      setComposerError("");
      setShowCreateRoom(false);
    } catch (err) {
      setSubmitError(localizeError(err.message, t));
    }
  }

  function selectConversation(id, options = {}) {
    setActiveConversationId(id);
    const next = { type: "conversation", id };
    setActivePane(next);
    setWorkspaceTab(WORKSPACE_TAB_MESSAGES);
    setShowMemberList(false);
    setShowChannelTools(false);
    if (options.updateURL !== false) {
      syncBrowserPath(next, options.rooms ?? rooms, options.replace ? "replace" : "push");
    }
  }

  function selectAgent(item, options = {}) {
    if (!item?.id) {
      return;
    }
    const next = { type: "agent", id: item.id };
    setActivePane(next);
    setWorkspaceTab(WORKSPACE_TAB_AGENTS);
    setShowMemberList(false);
    setShowChannelTools(false);
    if (options.updateURL !== false) {
      syncBrowserPath(next, rooms, options.replace ? "replace" : "push");
    }
  }

  function selectComputer(options = {}) {
    const next = { type: "computer", id: "local" };
    setActivePane(next);
    setWorkspaceTab(WORKSPACE_TAB_AGENTS);
    setShowMemberList(false);
    setShowChannelTools(false);
    if (options.updateURL !== false) {
      syncBrowserPath(next, rooms, options.replace ? "replace" : "push");
    }
  }

  function selectHub(options = {}) {
    const next = { type: "hub", id: "hub" };
    setActivePane(next);
    setWorkspaceTab(WORKSPACE_TAB_HUB);
    setShowMemberList(false);
    setShowChannelTools(false);
    if (options.updateURL !== false) {
      syncBrowserPath(next, rooms, options.replace ? "replace" : "push");
    }
  }

  function selectHubTemplate(item) {
    if (!item?.id) {
      selectHub();
      return;
    }
    setSelectedHubTemplateId(item.id);
    selectHub();
  }

  function toggleWorkspaceGroup(id) {
    setCollapsedWorkspaceGroups((current) => ({
      ...current,
      [id]: !current[id],
    }));
  }

  function openCreateRoomModal(options = {}) {
    if (!data) {
      return;
    }
    const lockedIDs = Array.from(new Set((options.lockedMemberIDs ?? [data.current_user_id]).filter(Boolean)));
    const selectedIDs = Array.from(new Set((options.preselectedMemberIDs ?? lockedIDs).filter(Boolean)));
    setRoomTitle(options.title ?? "");
    setRoomDescription(options.description ?? "");
    setRoomMemberIDs(selectedIDs);
    setLockedRoomMemberIDs(lockedIDs);
    setSubmitError("");
    setShowInvite(false);
    setShowCreateRoom(true);
  }

  function handleInviteAction() {
    if (!activeConversation) {
      return;
    }
    if (isDirectConversation(activeConversation)) {
      openCreateRoomModal({
        preselectedMemberIDs: activeConversation.members,
        lockedMemberIDs: activeConversation.members,
      });
      return;
    }
    setSubmitError("");
    setInviteUserIDs([]);
    setShowInvite(true);
  }

  async function inviteUsers() {
    if (!data || !activeConversation || inviteUserIDs.length === 0) {
      return;
    }

    setSubmitError("");
    try {
      const updated = await inviteRoomUsersRequest({
        room_id: activeConversation.id,
        inviter_id: data.current_user_id,
        user_ids: inviteUserIDs,
        locale,
      });
      setBootstrapData((current) => upsertConversationInData(current, updated));
      setComposerError("");
      setShowInvite(false);
    } catch (err) {
      setSubmitError(localizeError(err.message, t));
    }
  }

  async function deleteRoom(roomID) {
    if (!data || !roomID) {
      return;
    }

    try {
      await deleteRoomRequest(roomID);
    } catch (err) {
      setComposerError(localizeError(err.message, t));
      return;
    }

    const remainingRooms = rooms.filter((item) => item.id !== roomID);
    setBootstrapData((current) => removeConversationFromData(current, roomID));
    setDraftsByConversationId((current) => {
      if (!current[roomID]) {
        return current;
      }
      const next = { ...current };
      delete next[roomID];
      return next;
    });
    setComposerError("");
    setSubmitError("");
    if (activeConversationId === roomID) {
      const nextID = remainingRooms[0]?.id ?? "";
      if (nextID) {
        selectConversation(nextID, { replace: true });
      } else {
        setActiveConversationId("");
        selectComputer({ replace: true });
      }
    }
  }

  function applyMention(user) {
    const editor = editorRef.current;
    const state = getComposerMentionState(editor);
    if (!state) {
      return;
    }
    if (!replaceMentionQueryWithToken(editor, state, user)) {
      return;
    }
    syncComposerFromEditor();
  }

  function onComposerKeyDown(event) {
    if (mentionCandidates.length > 0) {
      if (event.key === "ArrowDown") {
        event.preventDefault();
        setMentionIndex((value) => (value + 1) % mentionCandidates.length);
        return;
      }
      if (event.key === "ArrowUp") {
        event.preventDefault();
        setMentionIndex((value) => (value - 1 + mentionCandidates.length) % mentionCandidates.length);
        return;
      }
      if (event.key === "Enter" && !event.shiftKey) {
        event.preventDefault();
        applyMention(mentionCandidates[mentionIndex]);
        return;
      }
    }

    if (event.key === "Backspace" && removeAdjacentMentionToken(editorRef.current, "backward")) {
      event.preventDefault();
      syncComposerFromEditor();
      return;
    }

    if (event.key === "Delete" && removeAdjacentMentionToken(editorRef.current, "forward")) {
      event.preventDefault();
      syncComposerFromEditor();
      return;
    }

    if (event.key === "Enter" && !event.shiftKey) {
      event.preventDefault();
      sendMessage();
      return;
    }

    if (event.key === "Enter" && event.shiftKey) {
      event.preventDefault();
      insertComposerLineBreak(editorRef.current);
      syncComposerFromEditor();
    }
  }

  function syncComposerFromEditor() {
    const editor = editorRef.current;
    if (!editor || !activeConversationId) {
      return;
    }
    const segments = parseComposerSegments(editor);
    setDraftsByConversationId((current) => updateDrafts(current, activeConversationId, segments));
    setComposerMentionState(getComposerMentionState(editor));
  }

  function clearComposer() {
    const editor = editorRef.current;
    if (editor) {
      editor.innerHTML = "";
      editor.focus();
    }
    if (activeConversationId) {
      setDraftsByConversationId((current) => updateDrafts(current, activeConversationId, []));
    }
    setComposerMentionState(null);
  }

  const selectedHubTemplate = useMemo(
    () => hubTemplates.find((item) => item.id === selectedHubTemplateId) || hubTemplates[0] || null,
    [hubTemplates, selectedHubTemplateId],
  );
  const selectedHubTemplateView = hubTemplateDetail?.id === selectedHubTemplateId ? hubTemplateDetail : selectedHubTemplate;

  if (!data) {
    return (<div className="empty-state">{loadingError || t("loading")}</div>);
  }

  const inviteCandidates = activeConversation
    ? data.users.filter((user) => !activeConversation.members.includes(user.id))
    : [];
  const activeConversationMembers = activeConversation
    ? activeConversation.members.map((id) => usersById.get(id)).filter(Boolean)
    : [];
  const inviteActionLabel = activeConversation && isDirectConversation(activeConversation)
    ? t("createRoomFromDM")
    : t("inviteMembers");

  const managerAgent = agents.find((item) => item.role === "manager" || item.id === "u-manager");
  const workerAgents = agents.filter((item) => item.id !== managerAgent?.id);
  const agentItems = [managerAgent, ...workerAgents].filter(Boolean);
  const runningAgentCount = agentItems.filter(isAgentRunning).length;
  const selectedAgent = selectedAgentForPage;
  const selectedConversation = activePane.type === "conversation" ? activeConversation : null;
  const activeChannel = selectedConversation && !isDirectConversation(selectedConversation) ? selectedConversation : null;
  const selectedMessageCount = selectedConversation?.messages?.length ?? 0;
  const currentWorkspaceLabel = activePane.type === "agent"
    ? t("agentOverview")
    : activePane.type === "computer"
      ? t("computerOverview")
      : activePane.type === "hub"
        ? t("hubOverview")
      : t("conversationOverview");
  const previewUser = profilePreview?.type === "user"
    ? usersById.get(profilePreview.id) ?? null
    : profilePreview?.type === "agent"
      ? usersById.get(profilePreview.id) ?? null
      : null;
  const previewAgent = profilePreview
    ? agentItems.find((item) => item.id === profilePreview.id || agentMatchesUser(item, previewUser)) ?? null
    : null;

  async function refreshManagerProfile() {
    try {
      const profile = await fetchManagerProfile();
      setManagerProfileData(profile);
      setProfileDraft({
        ...profileToDraft(profile),
        runtime_kind: normalizeRuntimeKind(bootstrapConfig?.runtime_kind || profile.runtime_kind),
      });
    } catch (_) {
      // The manager may not exist during the first bootstrap milliseconds.
    }
  }

  async function refreshCLIProxyAuthStatus(provider) {
    const normalized = normalizeAuthProviderName(provider);
    if (!providerNeedsAuth(normalized)) {
      return;
    }
    try {
      const status = await fetchCLIProxyAuthStatusRequest(normalized);
      setCLIProxyAuthStatuses((current) => ({ ...current, [normalized]: status }));
      setComposerError("");
    } catch (err) {
      setCLIProxyAuthStatuses((current) => ({
        ...current,
        [normalized]: {
          provider: normalized,
          authenticated: false,
          login_required: true,
          message: err.message || t("authMissing"),
        },
      }));
    }
  }

  async function refreshHubTemplates() {
    try {
      const payload = await fetchHubTemplatesRequest();
      setHubTemplatesData(Array.isArray(payload) ? payload : []);
      setHubManualError("");
    } catch (_) {
      setHubTemplatesData([]);
      setHubManualError(t("hubLoadFailed"));
    }
  }

  async function loadHubTemplateDetail(templateID) {
    if (!templateID) {
      return;
    }
    setHubTemplateDetailLoading(true);
    setHubTemplateDetailError("");
    setSelectedHubWorkspacePath("");
    setHubWorkspaceFile(null);
    setHubWorkspaceFileError("");
    try {
      const payload = await fetchHubTemplate(templateID);
      setHubTemplateDetail(payload);
    } catch (err) {
      setHubTemplateDetail(null);
      setHubTemplateDetailError(err.message || t("hubWorkspaceLoadFailed"));
    } finally {
      setHubTemplateDetailLoading(false);
    }
  }

  async function loadHubWorkspaceFile(templateID, workspacePath) {
    if (!templateID || !workspacePath) {
      return;
    }
    setHubWorkspaceFileLoading(true);
    setHubWorkspaceFileError("");
    try {
      setHubWorkspaceFile(await fetchHubWorkspaceFile(templateID, workspacePath));
    } catch (err) {
      setHubWorkspaceFile(null);
      setHubWorkspaceFileError(err.message || t("hubWorkspaceFileLoadFailed"));
    } finally {
      setHubWorkspaceFileLoading(false);
    }
  }

  async function loginCLIProxyProvider(provider) {
    const normalized = normalizeAuthProviderName(provider);
    if (!providerNeedsAuth(normalized) || cliproxyAuthBusy) {
      return;
    }
    setCLIProxyAuthBusy(normalized);
    setCLIProxyAuthStatuses((current) => ({
      ...current,
      [normalized]: {
        ...(current[normalized] || {}),
        provider: normalized,
        message: t("authConnecting"),
      },
    }));
    try {
      const status = await loginCLIProxyProviderRequest(normalized);
      setCLIProxyAuthStatuses((current) => ({ ...current, [normalized]: status }));
    } catch (err) {
      setCLIProxyAuthStatuses((current) => ({
        ...current,
        [normalized]: {
          provider: normalized,
          authenticated: false,
          login_required: true,
          message: err.message || t("authMissing"),
        },
      }));
    } finally {
      setCLIProxyAuthBusy("");
    }
  }

  async function loadProfileModels(draft = profileDraft, options = {}) {
    if (!draft?.provider) {
      return;
    }
    const requestKey = modelRequestKey(draft);
    if (!options.silent) {
      setProfileError("");
    }
    setProfileModelBusy(true);
    try {
      const payload = await fetchAgentProfileModels({
        ...draft,
        headers: parseJSONMap(draft.headersText),
      });
      if (modelRequestKey(profileDraftRef.current) !== requestKey) {
        return;
      }
      setProfileModels(payload.models ?? []);
      if (!profileDraftRef.current?.model_id && payload.models?.length > 0) {
        setProfileDraft((current) => {
          if (modelRequestKey(current) !== requestKey || current.model_id) {
            return current;
          }
          return { ...current, model_id: payload.models[0] };
        });
      }
    } catch (err) {
      if (!options.silent) {
        setProfileError(err.message || t("modelLoadFailed"));
      }
      if (modelRequestKey(profileDraftRef.current) === requestKey) {
        setProfileModels([]);
      }
    } finally {
      setProfileModelBusy(false);
    }
  }

  async function requestManagerRebuild() {
    await createManagerAgentRequest();
    await refreshAgents();
    await refreshManagerProfile();
  }

  async function rebuildManagerFromBrowser(options = {}) {
    const confirmText = options.confirm || t("managerRebuildConfirm");
    if (!options.skipConfirm && confirmText && !window.confirm(confirmText)) {
      return false;
    }
    setAgentActionBusy("u-manager:recreate");
    setAgentsError("");
    try {
      await requestManagerRebuild();
      return true;
    } catch (err) {
      setAgentsError(err.message || t("agentActionFailed"));
      return false;
    } finally {
      setAgentActionBusy("");
    }
  }

  async function handleMessageAction(action, message) {
    if (!action || action.id !== ACTION_REBUILD_MANAGER) {
      return;
    }
    const busyKey = `${message?.id || "message"}:${action.id}`;
    if (messageActionBusy || agentActionBusy) {
      return;
    }
    const confirmText = action.confirm || t("managerRebuildConfirm");
    if (confirmText && !window.confirm(confirmText)) {
      return;
    }
    setMessageActionBusy(busyKey);
    setMessageActionError({ key: "", message: "" });
    try {
      await requestManagerRebuild();
    } catch (err) {
      setMessageActionError({ key: busyKey, message: err.message || t("agentActionFailed") });
    } finally {
      setMessageActionBusy("");
    }
  }

  async function saveManagerProfile() {
    if (!profileDraft) {
      return;
    }
    setProfileBusy(true);
    setProfileError("");
    try {
      const payload = draftToProfile(profileDraft);
      const saved = await saveManagerProfileRequest(payload);
      setManagerProfileData(saved);
      setProfileDraft({ ...profileToDraft(saved), agent_id: "u-manager" });
      await refreshManagerProfile();
      setComposerError("");
    } catch (err) {
      setProfileError(err.message || t("sendFailed"));
    } finally {
      setProfileBusy(false);
    }
  }

  async function refreshAgents(options = {}) {
    try {
      setAgentsData(await fetchAgents(options));
      setAgentsError("");
    } catch (err) {
      if (!options.silent) {
        setAgentsError(errorMessage(err, t("agentActionFailed")));
      }
    }
  }

  function scheduleAgentsRefresh() {
    if (agentRefreshTimerRef.current) {
      window.clearTimeout(agentRefreshTimerRef.current);
    }
    agentRefreshTimerRef.current = window.setTimeout(() => {
      agentRefreshTimerRef.current = null;
      refreshAgents({ silent: true });
    }, 120);
  }

  async function openCreateAgentModal(template = undefined) {
    setAgentModalMode("create");
    setEditingAgent(null);
    setAgentError("");
    setAgentProgress(null);
    setAgentModels([]);
    const preferredRuntimeKind = normalizeRuntimeKind(bootstrapConfig?.runtime_kind || managerAgent?.runtime_kind || "") || "picoclaw_sandbox";
    const selectedTemplate = template === undefined
      ? pickDefaultAgentTemplate(hubTemplates, preferredRuntimeKind, bootstrapConfig)
      : normalizeTemplateSelection(template);
    try {
      const defaults = await fetchAgentProfileDefaults();
      const runtimeKind = normalizeRuntimeKind(selectedTemplate?.runtime_kind || bootstrapConfig?.runtime_kind || managerAgent?.runtime_kind || "") || "picoclaw_sandbox";
      let draft = agentToDraft({
        image: runtimeImageForKind(runtimeKind, bootstrapConfig, managerAgent?.image || ""),
        runtime_kind: runtimeKind,
        agent_profile: defaults,
      });
      draft = applyTemplateToDraft(draft, selectedTemplate, bootstrapConfig, managerAgent?.image || "");
      setAgentDraft(draft);
      setShowAgentModal(true);
      if (!isNotifierRuntimeDraft(draft)) {
        loadAgentModels(draft, { silent: true });
      }
    } catch (_) {
      const runtimeKind = normalizeRuntimeKind(selectedTemplate?.runtime_kind || bootstrapConfig?.runtime_kind || managerAgent?.runtime_kind || "") || "picoclaw_sandbox";
      let draft = agentToDraft({
        image: runtimeImageForKind(runtimeKind, bootstrapConfig, managerAgent?.image || ""),
        runtime_kind: runtimeKind,
        agent_profile: managerProfile,
      });
      draft = applyTemplateToDraft(draft, selectedTemplate, bootstrapConfig, managerAgent?.image || "");
      setAgentDraft(draft);
      setShowAgentModal(true);
      if (!isNotifierRuntimeDraft(draft)) {
        loadAgentModels(draft, { silent: true });
      }
    }
  }

  async function openEditAgentModal(item) {
    setAgentModalMode("edit");
    setEditingAgent(item);
    setAgentError("");
    setAgentProgress(null);
    setAgentModels([]);
    try {
      let profile = item.agent_profile;
      try {
        profile = await fetchAgentProfile(item.id);
      } catch (err) {
        if (!profile) {
          throw err;
        }
      }
      const draft = ensureNotifierPullSubscriptionDraft(agentToDraft({ ...item, agent_profile: profile }));
      setAgentDraft(draft);
      setShowAgentModal(true);
      if (!isNotifierRuntimeDraft(draft)) {
        loadAgentModels(draft, { silent: true });
      }
    } catch (err) {
      setAgentError(err.message || t("agentActionFailed"));
    }
  }

  async function loadAgentPageDraft(item) {
    if (!item?.id) {
      return;
    }
    setAgentPageError("");
    setAgentPageModels([]);
    try {
      let profile = item.agent_profile;
      try {
        profile = await fetchAgentProfile(item.id);
      } catch (err) {
        if (!profile) {
          throw err;
        }
      }
      const draft = ensureNotifierPullSubscriptionDraft(agentToDraft({ ...item, agent_profile: profile }));
      setAgentPageDraft(draft);
      if (!isNotifierRuntimeDraft(draft)) {
        loadAgentPageModels(draft, { silent: true });
      }
    } catch (err) {
      setAgentPageError(err.message || t("agentActionFailed"));
      const draft = ensureNotifierPullSubscriptionDraft(agentToDraft(item));
      setAgentPageDraft(draft);
      if (!isNotifierRuntimeDraft(draft)) {
        loadAgentPageModels(draft, { silent: true });
      }
    }
  }

  async function loadAgentPageModels(draft = agentPageDraft, options = {}) {
    if (!draft?.provider || isNotifierRuntimeDraft(draft)) {
      return;
    }
    const requestKey = modelRequestKey(draft);
    if (!options.silent) {
      setAgentPageError("");
    }
    setAgentPageModelBusy(true);
    try {
      const payload = await fetchAgentProfileModels({
        ...draft,
        headers: parseJSONMap(draft.headersText),
      });
      if (modelRequestKey(agentPageDraftRef.current) !== requestKey) {
        return;
      }
      setAgentPageModels(payload.models ?? []);
      if (!agentPageDraftRef.current?.model_id && payload.models?.length > 0) {
        setAgentPageDraft((current) => {
          if (modelRequestKey(current) !== requestKey || current.model_id) {
            return current;
          }
          return { ...current, model_id: payload.models[0] };
        });
      }
    } catch (err) {
      if (!options.silent) {
        setAgentPageError(err.message || t("modelLoadFailed"));
      }
      if (modelRequestKey(agentPageDraftRef.current) === requestKey) {
        setAgentPageModels([]);
      }
    } finally {
      setAgentPageModelBusy(false);
    }
  }

  async function loadAgentModels(draft = agentDraft, options = {}) {
    if (!draft?.provider || isNotifierRuntimeDraft(draft)) {
      return;
    }
    const requestKey = modelRequestKey(draft);
    if (!options.silent) {
      setAgentError("");
    }
    setAgentModelBusy(true);
    try {
      const payload = await fetchAgentProfileModels({
        ...draft,
        headers: parseJSONMap(draft.headersText),
      });
      if (modelRequestKey(agentDraftRef.current) !== requestKey) {
        return;
      }
      setAgentModels(payload.models ?? []);
      if (!agentDraftRef.current?.model_id && payload.models?.length > 0) {
        setAgentDraft((current) => {
          if (modelRequestKey(current) !== requestKey || current.model_id) {
            return current;
          }
          return { ...current, model_id: payload.models[0] };
        });
      }
    } catch (err) {
      if (!options.silent) {
        setAgentError(err.message || t("modelLoadFailed"));
      }
      if (modelRequestKey(agentDraftRef.current) === requestKey) {
        setAgentModels([]);
      }
    } finally {
      setAgentModelBusy(false);
    }
  }

  async function saveAgentPage() {
    if (!agentPageDraft || !selectedAgentForPage?.id) {
      return;
    }
    setAgentPageBusy(true);
    setAgentPageError("");
    try {
      const draft = ensureNotifierPullSubscriptionDraft(agentPageDraft);
      const profile = draftToProfile(draft, {
        name: agentPageDraft.name,
        description: agentPageDraft.description,
      });
      const runtimeOptions = draftNotifierRuntimeOptionsForSave(draft, {
        mergeNotifier: isNotifierRuntimeDraftOnAgentPage(agentPageDraft, selectedAgentForPage),
      });
      const payload = {
        name: agentPageDraft.name,
        description: agentPageDraft.description,
        agent_profile: profile,
      };
      if (runtimeOptions) {
        payload.runtime_options = runtimeOptions;
      }
      const saved = await updateAgentRequest(selectedAgentForPage.id, payload);
      await refreshAgents();
      if (saved.id === "u-manager") {
        await refreshManagerProfile();
      }
      setAgentPageDraft(agentToDraft(saved));
    } catch (err) {
      setAgentPageError(err.message || t("agentActionFailed"));
    } finally {
      setAgentPageBusy(false);
    }
  }

  async function publishAgentPage() {
    if (!selectedAgentForPage?.id || agentPagePublishBusy) {
      return;
    }
    setAgentPagePublishBusy(true);
    setAgentPageError("");
    try {
      const published = await publishAgentTemplateRequest(selectedAgentForPage.id);
      await refreshHubTemplates();
      if (published?.id) {
        setSelectedHubTemplateId(published.id);
      }
      selectHub();
    } catch (err) {
      setAgentPageError(err.message || t("agentActionFailed"));
    } finally {
      setAgentPagePublishBusy(false);
    }
  }

  async function saveAgent() {
    if (!agentDraft) {
      return;
    }
    setAgentBusy(true);
    setAgentError("");
    const isCreate = agentModalMode === "create";
    const runtimeKind = normalizeRuntimeKind(agentDraft.runtime_kind) || "picoclaw_sandbox";
    setAgentProgress(isCreate ? startAgentCreateProgress(runtimeKind) : null);
    try {
      const draft = ensureNotifierPullSubscriptionDraft(agentDraft);
      const profile = draftToProfile(draft, {
        name: agentDraft.name,
        description: agentDraft.description,
      });
      const runtimeOptions = draftNotifierRuntimeOptionsForSave(draft, {
        mergeNotifier: isNotifierRuntimeDraftOnAgentPage(agentDraft, editingAgent),
      });
      const payload = {
        name: agentDraft.name,
        role: "worker",
        description: agentDraft.description,
        image: isNotifierRuntimeDraft(draft) ? "" : agentDraft.image,
        runtime_kind: runtimeKind,
        from_template: agentDraft.from_template || "",
        agent_profile: profile,
      };
      if (runtimeOptions) {
        payload.runtime_options = runtimeOptions;
      }
      const saved = isCreate
        ? await createBotRequest(payload)
        : await updateAgentRequest(editingAgent.id, {
          name: payload.name,
          description: payload.description,
          agent_profile: payload.agent_profile,
          ...(payload.runtime_options ? { runtime_options: payload.runtime_options } : {}),
        });
      await refreshAgents();
      if (isCreate) {
        await refreshBootstrap();
      }
      if (saved.id === "u-manager") {
        await refreshManagerProfile();
      }
      if (isCreate) {
        setAgentProgress((current) => current ? { ...current, percent: 100, status: "done", index: Math.max(0, (current.steps?.length || 1) - 1) } : current);
      }
      setShowAgentModal(false);
      setAgentDraft(null);
      setAgentProgress(null);
    } catch (err) {
      setAgentProgress((current) => current ? { ...current, status: "failed" } : current);
      setAgentError(err.message || t("agentActionFailed"));
    } finally {
      setAgentBusy(false);
    }
  }

  async function runAgentAction(item, action) {
    if (!item?.id || agentActionBusy) {
      return;
    }
    if (action === "delete" && !window.confirm(`${t("agentDelete")} ${item.name}?`)) {
      return;
    }
    setAgentActionBusy(`${item.id}:${action}`);
    setAgentsError("");
    try {
      if (action === "recreate" && isManagerAgent(item)) {
        await rebuildManagerFromBrowser({ confirm: t("managerRebuildConfirm") });
        return;
      }
      if (action === "delete") {
        await deleteBotRequest(item.id);
      } else {
        await runAgentActionRequest(item.id, action);
      }
      await refreshAgents();
      if (item.id === "u-manager") {
        await refreshManagerProfile();
      }
    } catch (err) {
      setAgentsError(err.message || t("agentActionFailed"));
    } finally {
      setAgentActionBusy("");
    }
  }

  async function deletePreviewBot(item) {
    if (!item?.id || agentActionBusy) {
      return;
    }
    if (!window.confirm(`${t("agentDelete")} ${item.name}?`)) {
      return;
    }
    setAgentActionBusy(`${item.id}:delete-bot`);
    setAgentsError("");
    try {
      await deleteBotRequest(item.id);
      closeProfilePreview();
      await refreshAgents();
      await refreshBootstrap();
      if (item.id === "u-manager") {
        await refreshManagerProfile();
      }
    } catch (err) {
      setAgentsError(err.message || t("agentActionFailed"));
    } finally {
      setAgentActionBusy("");
    }
  }

  async function inviteAgentToRoom(item, options = {}) {
    if (!activeConversation || isDirectConversation(activeConversation) || !data?.current_user_id || !item?.id) {
      return;
    }
    if (!options.silent) {
      setAgentsError("");
    }
    try {
      await joinAgentToRoomRequest({
        agent_id: item.id,
        room_id: activeConversation.id,
        inviter_id: data.current_user_id,
        locale,
      });
      await refreshBootstrap();
    } catch (err) {
      if (!options.silent) {
        setAgentsError(err.message || t("agentActionFailed"));
      }
    }
  }

  function directConversationForUser(userID, roomList = rooms, currentUserID = data?.current_user_id) {
    if (!userID || !currentUserID) {
      return null;
    }
    return roomList.find((room) => (
      isDirectConversation(room) &&
      room.members.includes(currentUserID) &&
      room.members.includes(userID)
    )) ?? null;
  }

  async function openAgentDirectMessage(item) {
    if (!item?.id || !data?.current_user_id) {
      return;
    }

    setAgentsError("");
    try {
      let nextData = null;
      let direct = directConversationForUser(item.id);
      if (!direct) {
        await createUserRequest({
          id: item.id,
          name: item.name,
          handle: item.handle || item.id.replace(/^u-/, "") || item.name,
          role: item.role || "worker",
        });
        nextData = await refreshBootstrap();
        direct = directConversationForUser(
          item.id,
          nextData?.rooms ?? rooms,
          nextData?.current_user_id ?? data.current_user_id,
        );
      }

      if (!direct) {
        setAgentsError(t("agentActionFailed"));
        return;
      }
      selectConversation(direct.id, { rooms: nextData?.rooms ?? rooms });
      closeProfilePreview();
    } catch (err) {
      setAgentsError(err.message || t("agentActionFailed"));
    }
  }

  function openParticipantPreview(user, anchor) {
    if (!user?.id) {
      return;
    }
    const rect = anchor?.getBoundingClientRect?.();
    if (!rect) {
      return;
    }
    const agent = agents.find((item) => agentMatchesUser(item, user));
    setProfilePreview((current) => {
      const nextType = agent ? "agent" : "user";
      const nextID = agent ? agent.id : user.id;
      if (current?.type === nextType && current?.id === nextID) {
        return null;
      }
      return {
        type: nextType,
        id: nextID,
        anchorRect: {
          top: rect.top,
          right: rect.right,
          bottom: rect.bottom,
          left: rect.left,
        },
        anchorEl: anchor,
      };
    });
    setShowMemberList(false);
    setShowChannelTools(false);
  }

  function openAgentPreview(item, anchor) {
    if (!item?.id) {
      return;
    }
    const rect = anchor?.getBoundingClientRect?.();
    if (!rect) {
      return;
    }
    setProfilePreview((current) => {
      if (current?.type === "agent" && current?.id === item.id) {
        return null;
      }
      return {
        type: "agent",
        id: item.id,
        anchorRect: {
          top: rect.top,
          right: rect.right,
          bottom: rect.bottom,
          left: rect.left,
        },
        anchorEl: anchor,
      };
    });
    setShowChannelTools(false);
  }

  function closeProfilePreview() {
    setProfilePreview(null);
  }

  return (
    <React.Fragment>
      <div className={`app-shell ${isSidebarCollapsed ? "sidebar-collapsed" : ""}`}>
        <WorkspaceSidebar
          isSidebarCollapsed={isSidebarCollapsed}
          onCollapseSidebar={() => setIsSidebarCollapsed(true)}
          onExpandSidebar={() => setIsSidebarCollapsed(false)}
          theme={theme}
          onThemeChange={setTheme}
          locale={locale}
          onLocaleChange={setLocale}
          t={t}
          currentWorkspaceLabel={currentWorkspaceLabel}
          runningAgentCount={runningAgentCount}
          agentItems={agentItems}
          workspaceTab={workspaceTab}
          onWorkspaceTabChange={setWorkspaceTab}
          roomCount={roomCount}
          channels={channels}
          directMessages={directMessages}
          activePane={activePane}
          currentUserID={data.current_user_id}
          usersById={usersById}
          collapsedWorkspaceGroups={collapsedWorkspaceGroups}
          onToggleWorkspaceGroup={toggleWorkspaceGroup}
          onCreateRoom={() => openCreateRoomModal()}
          onCreateAgent={openCreateAgentModal}
          hubTemplates={hubTemplates}
          hubError={hubError}
          hubLoaded={hubLoaded}
          selectedHubTemplateId={selectedHubTemplateId}
          onSelectHubTemplate={selectHubTemplate}
          onSelectHub={selectHub}
          agentsError={agentsDisplayError}
          onSelectConversation={selectConversation}
          onPreviewUser={openParticipantPreview}
          onSelectAgent={selectAgent}
          onPreviewAgent={openAgentPreview}
          onSelectComputer={selectComputer}
          appVersion={appVersion}
          upgradeStatus={upgradeStatus}
          upgradeBusy={upgradeBusy}
          upgradePhase={upgradePhase}
          upgradeError={upgradeError}
          onOpenUpgrade={() => {
            setUpgradeError("");
            setUpgradePhase(upgradeBusy || upgradeStatus?.upgrading ? "restarting" : "idle");
            setShowUpgradeModal(true);
          }}
        />

        <main className="chat-panel">
          {activePane.type === "hub"
            ? (
                <HubDetailPane
                  t={t}
                  locale={locale}
                  templates={hubTemplates}
                  selectedTemplate={selectedHubTemplateView}
                  selectedTemplateId={selectedHubTemplateId}
                  loaded={hubLoaded}
                  error={hubError || hubTemplateDetailError}
                  detailLoading={hubTemplateDetailLoading}
                  selectedWorkspacePath={selectedHubWorkspacePath}
                  workspaceFile={hubWorkspaceFile}
                  workspaceFileLoading={hubWorkspaceFileLoading}
                  workspaceFileError={hubWorkspaceFileError}
                  onRetry={async () => {
                    await refreshHubTemplates();
                    if (selectedHubTemplateId) {
                      await loadHubTemplateDetail(selectedHubTemplateId);
                    }
                  }}
                  onSelectTemplate={selectHubTemplate}
                  onSelectWorkspaceFile={(workspacePath) => {
                    setSelectedHubWorkspacePath(workspacePath);
                    loadHubWorkspaceFile(selectedHubTemplateId, workspacePath);
                  }}
                  onCreateFromTemplate={openCreateAgentModal}
                />
              )
            : activePane.type === "agent" && selectedAgent
            ? (
                <AgentDetailPane
                  item={selectedAgent}
                  t={t}
                  activeRoom={activeChannel}
                  busyKey={agentActionBusy}
                  error={agentsDisplayError}
                  draft={agentPageDraft}
                  models={agentPageModels}
                  modelBusy={agentPageModelBusy}
                  saving={agentPageBusy}
                  publishBusy={agentPagePublishBusy}
                  saveError={agentPageError}
                  authStatuses={cliproxyAuthStatuses}
                  authBusyProvider={cliproxyAuthBusy}
                  notifierWebhookOrigin={notifierPageWebhookOrigin}
                  setNotifierWebhookOrigin={setNotifierPageWebhookOrigin}
                  onDraftChange={setAgentPageDraft}
                  onSave={saveAgentPage}
                  onPublish={publishAgentPage}
                  onProviderLogin={loginCLIProxyProvider}
                  onStart={(item) => runAgentAction(item, "start")}
                  onStop={(item) => runAgentAction(item, "stop")}
                  onRecreate={(item) => runAgentAction(item, "recreate")}
                  onDelete={(item) => runAgentAction(item, "delete")}
                  onInvite={inviteAgentToRoom}
                  onOpenDM={openAgentDirectMessage}
                />
              )
            : activePane.type === "computer"
              ? (
                  <ComputerDetailPane
                    t={t}
                    agents={agentItems}
                    channels={channels}
                    directMessages={directMessages}
                    activeAgentID={activePane.type === "agent" ? activePane.id : ""}
                    busyKey={agentActionBusy}
                    onSelectAgent={selectAgent}
                    onCreateAgent={openCreateAgentModal}
                    onStartAgent={(item) => runAgentAction(item, "start")}
                  />
                )
              : selectedConversation
            ? (
                <ConversationPane
                  conversation={selectedConversation}
                  visibleMessages={visibleMessages}
                  currentUserID={data.current_user_id}
                  usersById={usersById}
                  locale={locale}
                  t={t}
                  theme={theme}
                  selectedMessageCount={selectedMessageCount}
                  conversationMembers={activeConversationMembers}
                  showMemberList={showMemberList}
                  onToggleMemberList={setShowMemberList}
                  showChannelTools={showChannelTools}
                  onToggleChannelTools={setShowChannelTools}
                  showToolCalls={showToolCalls}
                  onToggleToolCalls={setShowToolCalls}
                  memberMenuRef={memberMenuRef}
                  channelToolsRef={channelToolsRef}
                  messageListRef={messageListRef}
                  editorRef={editorRef}
                  onPreviewUser={openParticipantPreview}
                  onDeleteRoom={deleteRoom}
                  inviteActionLabel={inviteActionLabel}
                  onInviteAction={handleInviteAction}
                  mentionCandidates={mentionCandidates}
                  mentionIndex={mentionIndex}
                  onApplyMention={applyMention}
                  managerProfile={managerProfile}
                  managerProfileIncomplete={managerProfileIncomplete}
                  authStatuses={cliproxyAuthStatuses}
                  authBusyProvider={cliproxyAuthBusy}
                  onProviderLogin={loginCLIProxyProvider}
                  draftSegments={draftSegments}
                  draftText={draftText}
                  mentionableUsersByHandle={mentionableUsersByHandle}
                  onSyncComposer={syncComposerFromEditor}
                  onComposerKeyDown={onComposerKeyDown}
                  onSendMessage={sendMessage}
                  composerError={composerError}
                  messageActionBusy={messageActionBusy}
                  messageActionError={messageActionError}
                  onMessageAction={handleMessageAction}
                />
              )
            : (
                <div className="empty-state shell-empty-state">
                  <span className="rich-empty-mark" aria-hidden="true">{">"}</span>
                  <strong>{t("emptyConversation")}</strong>
                </div>
              )}
        </main>
      </div>

      {profilePreview && (previewAgent || previewUser)
        ? (
            <ProfilePreviewPopover
              previewRef={profilePreviewRef}
              agent={previewAgent}
              user={previewUser}
              anchorRect={profilePreview.anchorRect}
              t={t}
              inDirectConversation={Boolean(selectedConversation && isDirectConversation(selectedConversation))}
              busyKey={agentActionBusy}
              onClose={closeProfilePreview}
              onOpenAgent={(item) => {
                selectAgent(item);
                closeProfilePreview();
              }}
              onOpenDM={openAgentDirectMessage}
              onDelete={deletePreviewBot}
            />
          )
        : null}

      {showCreateRoom
        ? (
            <CreateRoomModal
              t={t}
              roomTitle={roomTitle}
              onRoomTitleChange={setRoomTitle}
              roomDescription={roomDescription}
              onRoomDescriptionChange={setRoomDescription}
              candidates={data.users}
              roomMemberIDs={roomMemberIDs}
              lockedRoomMemberIDs={lockedRoomMemberIDs}
              onRoomMemberIDsChange={setRoomMemberIDs}
              submitError={submitError}
              onClose={() => setShowCreateRoom(false)}
              onCreate={createRoom}
            />
          )
        : null}

      {showInvite
        ? (
            <InviteMembersModal
              t={t}
              candidates={inviteCandidates}
              inviteUserIDs={inviteUserIDs}
              onInviteUserIDsChange={setInviteUserIDs}
              submitError={submitError}
              onClose={() => setShowInvite(false)}
              onInvite={inviteUsers}
            />
          )
        : null}

      {showUpgradeModal
        ? (
            <UpgradeModal
              t={t}
              upgradeStatus={upgradeStatus}
              appVersion={appVersion}
              upgradePhase={upgradePhase}
              upgradeBusy={upgradeBusy}
              upgradeError={upgradeError}
              onClose={() => setShowUpgradeModal(false)}
              onApply={applyUpgrade}
            />
          )
        : null}

      {showAgentModal && agentDraft
        ? (
            <AgentProfileModal
              t={t}
              agentModalMode={agentModalMode}
              editingAgent={editingAgent}
              agentDraft={agentDraft}
              onAgentDraftChange={setAgentDraft}
              onAgentModelsReset={() => setAgentModels([])}
              hubTemplates={hubTemplates}
              bootstrapConfig={bootstrapConfig}
              managerAgent={managerAgent}
              agentModels={agentModels}
              agentModelBusy={agentModelBusy}
              authStatuses={cliproxyAuthStatuses}
              authBusyProvider={cliproxyAuthBusy}
              notifierWebhookOrigin={notifierModalWebhookOrigin}
              setNotifierWebhookOrigin={setNotifierModalWebhookOrigin}
              onProviderLogin={loginCLIProxyProvider}
              agentError={agentError}
              agentProgress={agentProgress}
              agentBusy={agentBusy}
              onClose={() => setShowAgentModal(false)}
              onSave={saveAgent}
            />
          )
        : null}

      {managerProfileIncomplete && profileDraft
        ? (
            <ManagerProfileSetupModal
              t={t}
              managerProfile={managerProfile}
              profileDraft={profileDraft}
              onProfileDraftChange={setProfileDraft}
              onProfileModelsReset={() => setProfileModels([])}
              bootstrapConfig={bootstrapConfig}
              profileModels={profileModels}
              profileModelBusy={profileModelBusy}
              authStatuses={cliproxyAuthStatuses}
              authBusyProvider={cliproxyAuthBusy}
              onProviderLogin={loginCLIProxyProvider}
              profileError={profileError}
              profileBusy={profileBusy}
              onSave={saveManagerProfile}
            />
          )
        : null}
    </React.Fragment>
  );
}


export { WorkspacePage };
