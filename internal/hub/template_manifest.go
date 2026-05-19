package hub

import (
	"fmt"
	"strings"
	"time"

	"csgclaw/internal/runtime"
)

type templateManifest struct {
	Name        string `toml:"name"`
	Description string `toml:"description,omitempty"`
	Role        string `toml:"role"`
	RuntimeKind string `toml:"runtime_kind"`
	Image       string `toml:"image,omitempty"`
	UpdatedAt   string `toml:"updated_at,omitempty"`
}

func validateManifest(manifest templateManifest) error {
	manifest.Name = strings.TrimSpace(manifest.Name)
	if manifest.Name == "" {
		return ErrTemplateNameRequired
	}
	switch normalizeTemplateRole(manifest.Role) {
	case TemplateRoleManager, TemplateRoleWorker:
	default:
		return fmt.Errorf("role must be one of %q or %q", TemplateRoleManager, TemplateRoleWorker)
	}
	switch manifest.RuntimeKind {
	case runtime.KindPicoClawSandbox, runtime.KindOpenClawSandbox, runtime.KindCodex:
	default:
		return fmt.Errorf("%w: %s", ErrRuntimeKindRequired, manifest.RuntimeKind)
	}
	if requiresTemplateImage(manifest.RuntimeKind) && strings.TrimSpace(manifest.Image) == "" {
		return fmt.Errorf("image is required for runtime_kind %q", manifest.RuntimeKind)
	}
	if _, err := parseManifestUpdatedAt(manifest.UpdatedAt); err != nil {
		return err
	}
	return nil
}

func requiresTemplateImage(runtimeKind string) bool {
	switch runtimeKind {
	case runtime.KindPicoClawSandbox, runtime.KindOpenClawSandbox:
		return true
	default:
		return false
	}
}

func parseManifestUpdatedAt(value string) (time.Time, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return time.Time{}, nil
	}
	parsed, err := time.Parse(time.RFC3339Nano, value)
	if err != nil {
		return time.Time{}, fmt.Errorf("invalid updated_at %q", value)
	}
	return parsed.UTC(), nil
}

const (
	TemplateRoleManager = "manager"
	TemplateRoleWorker  = "worker"
)

func normalizeTemplateRole(role string) string {
	switch strings.ToLower(strings.TrimSpace(role)) {
	case TemplateRoleManager:
		return TemplateRoleManager
	case TemplateRoleWorker:
		return TemplateRoleWorker
	default:
		return strings.ToLower(strings.TrimSpace(role))
	}
}
