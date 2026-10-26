package controlplane

import "time"

const DefaultAssistantKey = "default_assistant"

// AssistantBinding is local to one private CSGClaw service. User-to-instance
// allocation and quotas belong to CSGHub Server, not this record.
type AssistantBinding struct {
	AgentID       string    `json:"agent_id"`
	OperationID   string    `json:"operation_id"`
	Template      string    `json:"template,omitempty"`
	Runtime       string    `json:"runtime"`
	ModelSelector string    `json:"model_selector,omitempty"`
	State         string    `json:"state"`
	ErrorCode     string    `json:"error_code,omitempty"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}
