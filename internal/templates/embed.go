package templates

import "embed"

const (
	Root                = "embed"
	ManifestFileName    = "agent.toml"
	WorkspaceDirName    = "workspace"
	PicoClawManagerRoot = Root + "/picoclaw-manager"
	PicoClawWorkerRoot  = Root + "/picoclaw-worker"
	OpenClawWorkerRoot  = Root + "/openclaw-worker"
)

//go:embed embed/picoclaw-manager embed/picoclaw-worker embed/openclaw-worker
var runtimeTemplateFS embed.FS
