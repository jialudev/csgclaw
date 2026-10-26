package contract

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
)

// ConfigurationRevision covers legacy writes as well as the unified editor,
// including the installed skill list which is stored outside the Agent record.
// Credentials are write-only and are deliberately excluded from the projection.
func ConfigurationRevision(spec AgentSpec, resourceVersion string) string {
	spec.Runtime.Credentials = nil
	spec.Model.APIKey = ""
	spec.Memory = nil // memory document revisions are managed by the document API
	raw, _ := json.Marshal(struct {
		Spec    AgentSpec
		Version string
	}{spec, resourceVersion})
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:])
}
