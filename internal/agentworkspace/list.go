package agentworkspace

import (
	"csgclaw/internal/apitypes"
	"csgclaw/internal/utils/filebrowse"
)

const filePreviewMaxBytes = filebrowse.FilePreviewMaxBytes

func List(root, relativePath string) (apitypes.WorkspaceListing, error) {
	return filebrowse.List(root, relativePath)
}

func ReadFile(root, relativePath string) (apitypes.WorkspaceFile, error) {
	return filebrowse.ReadFile(root, relativePath)
}
