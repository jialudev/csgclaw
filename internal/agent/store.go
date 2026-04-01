package agent

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type persistedState struct {
	Agents  []Agent        `json:"agents"`
	Workers []legacyWorker `json:"workers,omitempty"`
}

func (s persistedState) isObject() bool {
	return s.Agents != nil || s.Workers != nil
}

type legacyWorker struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description,omitempty"`
	Status      string    `json:"status"`
	CreatedAt   time.Time `json:"created_at"`
	ModelID     string    `json:"model_id,omitempty"`
}

func (w legacyWorker) toAgent() Agent {
	return Agent{
		ID:          w.ID,
		Name:        w.Name,
		Description: w.Description,
		Image:       "",
		Role:        RoleWorker,
		Status:      w.Status,
		CreatedAt:   w.CreatedAt,
		ModelID:     w.ModelID,
	}
}

func (s *Service) load() error {
	if s.state == "" {
		return nil
	}

	data, err := os.ReadFile(s.state)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("read agent state: %w", err)
	}

	var state persistedState
	if err := json.Unmarshal(data, &state); err == nil && state.isObject() {
		for _, a := range state.Agents {
			normalized := s.normalizeLoadedAgent(a)
			s.agents[normalized.ID] = normalized
		}
		for _, w := range state.Workers {
			normalized := s.normalizeLoadedAgent(w.toAgent())
			s.agents[normalized.ID] = normalized
		}
		return nil
	}

	var agents []Agent
	if err := json.Unmarshal(data, &agents); err != nil {
		return fmt.Errorf("decode agent state: %w", err)
	}
	for _, a := range agents {
		normalized := s.normalizeLoadedAgent(a)
		s.agents[normalized.ID] = normalized
	}
	return nil
}

func (s *Service) saveLocked() error {
	if s.state == "" {
		return nil
	}

	data, err := json.MarshalIndent(persistedState{
		Agents: sortedAgentsFromMap(s.agents),
	}, "", "  ")
	if err != nil {
		return fmt.Errorf("encode agent state: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(s.state), 0o755); err != nil {
		return fmt.Errorf("create state dir: %w", err)
	}
	if err := os.WriteFile(s.state, append(data, '\n'), 0o600); err != nil {
		return fmt.Errorf("write agent state: %w", err)
	}
	return nil
}

func (s *Service) normalizeLoadedAgent(a Agent) Agent {
	a = *cloneAgent(&a)
	a.Role = normalizeRole(a.Role)
	if isManagerAgent(a) {
		a.ID = ManagerUserID
		a.Name = ManagerName
		a.Role = RoleManager
		if strings.TrimSpace(a.Image) == "" {
			a.Image = s.managerImage
		}
	}
	return a
}
