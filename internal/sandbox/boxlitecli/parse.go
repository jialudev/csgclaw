package boxlitecli

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"csgclaw/internal/sandbox"
)

type inspectBox struct {
	ID      string `json:"Id"`
	Name    string `json:"Name"`
	Created string `json:"Created"`
	Status  string `json:"Status"`
	State   struct {
		Status string `json:"Status"`
	} `json:"State"`
}

func parseInspect(data []byte) (sandbox.Info, error) {
	var boxes []inspectBox
	if err := json.Unmarshal(data, &boxes); err != nil {
		return sandbox.Info{}, fmt.Errorf("parse boxlite inspect json: %w", err)
	}
	if len(boxes) == 0 {
		return sandbox.Info{}, sandbox.ErrNotFound
	}
	box := boxes[0]
	createdAt, err := parseCreatedAt(box.Created)
	if err != nil {
		return sandbox.Info{}, err
	}
	status := box.Status
	if status == "" {
		status = box.State.Status
	}
	return sandbox.Info{
		ID:        box.ID,
		Name:      box.Name,
		State:     mapState(status),
		CreatedAt: createdAt,
	}, nil
}

func parseCreatedAt(value string) (time.Time, error) {
	if strings.TrimSpace(value) == "" {
		return time.Time{}, nil
	}
	createdAt, err := time.Parse(time.RFC3339Nano, value)
	if err != nil {
		return time.Time{}, fmt.Errorf("parse boxlite created time %q: %w", value, err)
	}
	return createdAt, nil
}

func mapState(status string) sandbox.State {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "configured", "created":
		return sandbox.StateCreated
	case "running":
		return sandbox.StateRunning
	case "stopped":
		return sandbox.StateStopped
	case "exited":
		return sandbox.StateExited
	case "stopping":
		return sandbox.StateUnknown
	default:
		return sandbox.StateUnknown
	}
}
