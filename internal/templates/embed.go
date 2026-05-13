package templates

import "embed"

const (
	Root                = "embed/runtimes"
	ManifestFileName    = "agent.toml"
	WorkspaceDirName    = "workspace"
	PicoClawManagerRoot = Root + "/picoclaw/manager"
	PicoClawWorkerRoot  = Root + "/picoclaw/worker"
	OpenClawWorkerRoot  = Root + "/openclaw/worker"
)

//go:embed embed/runtimes/picoclaw/manager embed/runtimes/picoclaw/worker
//go:embed embed/runtimes/openclaw/worker
var runtimeTemplateFS embed.FS
