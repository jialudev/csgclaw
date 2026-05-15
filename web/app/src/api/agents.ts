// @ts-nocheck
import { del, get, patch, post, put } from "@/api/client";

function modelPayload(draft) {
  return {
    agent_id: draft.agent_id,
    provider: draft.provider,
    base_url: draft.base_url,
    api_key: draft.api_key,
    headers: draft.headers,
  };
}

export function fetchManagerProfile() {
  return get("api/v1/agents/u-manager/profile");
}

export function saveManagerProfileRequest(profile) {
  return put("api/v1/agents/u-manager/profile", profile);
}

export function fetchAgents(options = {}) {
  return get(options.silent ? "api/v1/agents?poll=1" : "api/v1/agents");
}

export function createManagerAgentRequest() {
  return post("api/v1/agents", {
    id: "u-manager",
    replace: true,
  });
}

export function fetchAgentProfileDefaults() {
  return get("api/v1/agent-profile-defaults");
}

export function fetchAgentProfile(agentID) {
  return get(`api/v1/agents/${encodeURIComponent(agentID)}/profile`);
}

export function fetchAgentProfileModels(draft) {
  return post("api/v1/agent-profiles/models", modelPayload(draft));
}

export function updateAgentRequest(agentID, payload) {
  return patch(`api/v1/agents/${encodeURIComponent(agentID)}`, payload);
}

export function createBotRequest(payload) {
  return post("api/v1/channels/csgclaw/bots", payload);
}

export function deleteBotRequest(botID) {
  return del(`api/v1/channels/csgclaw/bots/${encodeURIComponent(botID)}`);
}

export function runAgentActionRequest(agentID, action) {
  return post(`api/v1/agents/${encodeURIComponent(agentID)}/${action}`);
}
