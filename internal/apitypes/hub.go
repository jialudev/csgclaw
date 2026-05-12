package apitypes

import "time"

type CreateHubTemplateRequest struct {
	AgentID  string `json:"agent_id"`
	Registry string `json:"registry,omitempty"`
}

type HubTemplate struct {
	ID          string               `json:"id"`
	Name        string               `json:"name"`
	Description string               `json:"description,omitempty"`
	RuntimeKind string               `json:"runtime_kind,omitempty"`
	Image       string               `json:"image,omitempty"`
	Source      HubTemplateSource    `json:"source"`
	UpdatedAt   time.Time            `json:"updated_at,omitempty"`
	Workspace   HubTemplateWorkspace `json:"workspace,omitempty"`
}

type HubTemplateSource struct {
	Name string `json:"name"`
	Kind string `json:"kind"`
}

type HubTemplateWorkspace struct {
	Kind string `json:"kind"`
}
