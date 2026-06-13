package codex

import (
	"fmt"
	"strings"

	agentruntime "csgclaw/internal/runtime"
)

const localWorkspaceDirOptionKey = "local_workspace_dir"

type RuntimeOptions struct {
	LocalWorkspaceDir string `json:"local_workspace_dir"`
}

func DecodeRuntimeOptions(raw map[string]any) (RuntimeOptions, error) {
	if len(raw) == 0 {
		return RuntimeOptions{}, nil
	}
	opts := RuntimeOptions{}
	value, ok := raw[localWorkspaceDirOptionKey]
	if !ok || value == nil {
		return opts, nil
	}
	text, ok := value.(string)
	if !ok {
		return RuntimeOptions{}, fmt.Errorf("%s must be a string", localWorkspaceDirOptionKey)
	}
	opts.LocalWorkspaceDir = strings.TrimSpace(text)
	return opts, nil
}

func (r *Runtime) RuntimeOptionsSchema() []agentruntime.RuntimeOptionSchema {
	return []agentruntime.RuntimeOptionSchema{
		{
			Key:           localWorkspaceDirOptionKey,
			Path:          localWorkspaceDirOptionKey,
			Label:         "Local Workspace Dir",
			LabelZh:       "本地工作目录",
			LabelEn:       "Local Workspace Dir",
			Description:   "Leave empty to use the default agent workspace.",
			DescriptionZh: "留空时使用默认 Agent 工作目录。",
			DescriptionEn: "Leave empty to use the default agent workspace.",
			Type:          "directory",
			Picker:        "optional",
		},
	}
}
