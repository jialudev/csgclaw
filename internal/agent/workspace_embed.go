package agent

import "embed"

// workspaceTemplateFS contains the bundled PicoClaw workspace templates.
//
//go:generate ../../scripts/sync-agent-runtimes.sh
//go:embed embed/runtimes/picoclaw/manager/workspace embed/runtimes/picoclaw/worker/workspace
var workspaceTemplateFS embed.FS
