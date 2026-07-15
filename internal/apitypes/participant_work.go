package apitypes

import "time"

const (
	ParticipantWorkKindAgentTurn = "agent_turn"

	ParticipantWorkStateWorking = "working"
	ParticipantWorkStateIdle    = "idle"

	ParticipantWorkReasonStarted  = "started"
	ParticipantWorkReasonRenewed  = "renewed"
	ParticipantWorkReasonReleased = "released"
	ParticipantWorkReasonExpired  = "expired"
)

type ParticipantWorkLeaseRequest struct {
	RoomID       string `json:"room_id"`
	ThreadRootID string `json:"thread_root_id,omitempty"`
	RequestID    string `json:"request_id"`
	Kind         string `json:"kind"`
	TTLSeconds   *int   `json:"ttl_seconds,omitempty"`
}

type ParticipantWorkUpdate struct {
	RegistryEpoch string    `json:"registry_epoch"`
	LeaseID       string    `json:"lease_id"`
	ParticipantID string    `json:"participant_id"`
	UserID        string    `json:"user_id"`
	RoomID        string    `json:"room_id"`
	ThreadRootID  string    `json:"thread_root_id,omitempty"`
	RequestID     string    `json:"request_id"`
	Kind          string    `json:"kind"`
	State         string    `json:"state"`
	Reason        string    `json:"reason"`
	Revision      uint64    `json:"revision"`
	ExpiresAt     time.Time `json:"expires_at"`
}

type ParticipantWorkClosedResponse struct {
	Error         string `json:"error"`
	RegistryEpoch string `json:"registry_epoch"`
}
