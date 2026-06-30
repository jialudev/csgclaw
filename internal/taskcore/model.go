package taskcore

import "time"

const (
	AssignmentTypeTeam  = "team"
	AssignmentTypeAgent = "agent"

	StatusPending    = "pending"
	StatusAssigned   = "assigned"
	StatusInProgress = "in_progress"
	StatusBlocked    = "blocked"
	StatusCompleted  = "completed"
	StatusFailed     = "failed"
	StatusCancelled  = "cancelled"

	ApprovalStatusPending   = "pending"
	ApprovalStatusApproved  = "approved"
	ApprovalStatusRejected  = "rejected"
	ApprovalStatusCancelled = "cancelled"

	EventTaskCreated       = "task.created"
	EventTaskExecutionRoom = "task.execution_room"
	EventTaskAssigned      = "task.assigned"
	EventTaskClaimed       = "task.claimed"
	EventTaskBlocked       = "task.blocked"
	EventTaskCompleted     = "task.completed"
	EventTaskFailed        = "task.failed"
	EventTaskCancelled     = "task.cancelled"
	EventApprovalRequested = "approval.requested"
	EventApprovalResolved  = "approval.resolved"
	EventPresenceUpdated   = "presence.updated"
)

type Task struct {
	ID               string     `json:"id"`
	ParentID         string     `json:"parent_id,omitempty"`
	AssignmentType   string     `json:"assignment_type"`
	AssignmentID     string     `json:"assignment_id"`
	Title            string     `json:"title"`
	Body             string     `json:"body,omitempty"`
	Status           string     `json:"status"`
	CreatedBy        string     `json:"created_by"`
	AssignedTo       string     `json:"assigned_to,omitempty"`
	ClaimedBy        string     `json:"claimed_by,omitempty"`
	ExecutionChannel string     `json:"execution_channel,omitempty"`
	RoomID           string     `json:"room_id,omitempty"`
	DependsOn        []string   `json:"depends_on,omitempty"`
	Priority         int        `json:"priority,omitempty"`
	PlanSummary      string     `json:"plan_summary,omitempty"`
	DispatchedAt     *time.Time `json:"dispatched_at,omitempty"`
	DeadlineAt       *time.Time `json:"deadline_at,omitempty"`
	TimeoutAt        *time.Time `json:"timeout_at,omitempty"`
	Result           string     `json:"result,omitempty"`
	Error            string     `json:"error,omitempty"`
	CreatedAt        time.Time  `json:"created_at"`
	UpdatedAt        time.Time  `json:"updated_at"`
	CompletedAt      *time.Time `json:"completed_at,omitempty"`
}

type TaskEvent struct {
	Seq            int64     `json:"seq,omitempty"`
	AssignmentType string    `json:"assignment_type"`
	AssignmentID   string    `json:"assignment_id"`
	Channel        string    `json:"channel,omitempty"`
	RoomID         string    `json:"room_id,omitempty"`
	Type           string    `json:"type"`
	ActorID        string    `json:"actor_id,omitempty"`
	TaskID         string    `json:"task_id,omitempty"`
	TargetID       string    `json:"target_id,omitempty"`
	Summary        string    `json:"summary,omitempty"`
	CreatedAt      time.Time `json:"created_at"`
}

type TaskApproval struct {
	ID             string     `json:"id"`
	AssignmentType string     `json:"assignment_type"`
	AssignmentID   string     `json:"assignment_id"`
	RoomID         string     `json:"room_id,omitempty"`
	TaskID         string     `json:"task_id,omitempty"`
	RequestedBy    string     `json:"requested_by"`
	ApproverID     string     `json:"approver_id,omitempty"`
	Kind           string     `json:"kind"`
	Summary        string     `json:"summary"`
	Payload        string     `json:"payload,omitempty"`
	Status         string     `json:"status"`
	Resolution     string     `json:"resolution,omitempty"`
	CreatedAt      time.Time  `json:"created_at"`
	ResolvedAt     *time.Time `json:"resolved_at,omitempty"`
}

type TaskPresence struct {
	AssignmentType  string    `json:"assignment_type"`
	AssignmentID    string    `json:"assignment_id"`
	ParticipantID   string    `json:"participant_id"`
	UserID          string    `json:"user_id,omitempty"`
	AgentID         string    `json:"agent_id,omitempty"`
	Role            string    `json:"role,omitempty"`
	State           string    `json:"state"`
	CurrentTaskID   string    `json:"current_task_id,omitempty"`
	Summary         string    `json:"summary,omitempty"`
	LastHeartbeatAt time.Time `json:"last_heartbeat_at,omitempty"`
	UpdatedAt       time.Time `json:"updated_at"`
}

type Snapshot struct {
	Root      Task
	Children  []Task
	Approvals []TaskApproval
	Presence  []TaskPresence
	Events    []TaskEvent
}

type GlobalTaskView struct {
	Task Task `json:"task"`
}
