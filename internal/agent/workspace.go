package agent

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

const (
	workspaceTemplateManagerPicoclaw = "embed/runtimes/picoclaw/manager/workspace"
	workspaceTemplateWorkerPicoclaw  = "embed/runtimes/picoclaw/worker/workspace"
	workspaceTemplateManagerOpenClaw = "embed/runtimes/openclaw/manager/workspace"
	workspaceTemplateWorkerOpenClaw  = "embed/runtimes/openclaw/worker/workspace"
	// openclawCsgSkillsEmbedPath is the CSG-owned skill pack; see ensureOpenClawCsgSkills in openclaw_config.go.
	openclawCsgSkillsEmbedPath = "embed/runtimes/openclaw/csg-skills"
)

// managerGatewayMatch reports whether a gateway run should use manager templates (PicoClaw or OpenClaw),
// by agent name and bot id (the same test as the workspace template chooser).
func managerGatewayMatch(name, botID string) bool {
	return strings.EqualFold(strings.TrimSpace(name), ManagerName) || strings.TrimSpace(botID) == ManagerUserID
}

func workspaceTemplateForAgent(name, botID string, openClaw bool) string {
	isManager := managerGatewayMatch(name, botID)
	if openClaw {
		if isManager {
			return workspaceTemplateManagerOpenClaw
		}
		return workspaceTemplateWorkerOpenClaw
	}
	if isManager {
		return workspaceTemplateManagerPicoclaw
	}
	return workspaceTemplateWorkerPicoclaw
}

func ensureAgentWorkspace(agentName, template string) (string, error) {
	hostRoot, err := agentWorkspaceRoot(agentName)
	if err != nil {
		return "", err
	}
	return ensureWorkspaceAtRoot(hostRoot, template)
}

func ensureAgentOpenClawWorkspace(agentName, template string) (string, error) {
	hostRoot, err := agentOpenClawWorkspaceRoot(agentName)
	if err != nil {
		return "", err
	}
	return ensureWorkspaceAtRoot(hostRoot, template)
}

func ensureWorkspaceAtRoot(hostRoot, template string) (string, error) {
	if strings.TrimSpace(template) == "" {
		return "", fmt.Errorf("workspace template is required")
	}
	if err := os.MkdirAll(hostRoot, 0o755); err != nil {
		return "", fmt.Errorf("create agent workspace dir: %w", err)
	}
	if err := copyEmbeddedTree(template, hostRoot); err != nil {
		return "", err
	}
	return hostRoot, nil
}

func agentWorkspaceRoot(agentName string) (string, error) {
	agentHome, err := agentHomeDir(agentName)
	if err != nil {
		return "", err
	}
	return filepath.Join(agentHome, hostWorkspaceDir), nil
}

func agentOpenClawWorkspaceRoot(agentName string) (string, error) {
	hostRoot, err := agentOpenClawRoot(agentName)
	if err != nil {
		return "", err
	}
	return filepath.Join(hostRoot, hostWorkspaceDir), nil
}

func copyOpenClawCsgSkillsPack(dstRoot string) error {
	return copyEmbeddedTree(openclawCsgSkillsEmbedPath, dstRoot)
}

func copyEmbeddedTree(template, dstRoot string) error {
	template = strings.Trim(strings.TrimSpace(template), "/")
	return fs.WalkDir(workspaceTemplateFS, template, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return fmt.Errorf("walk embedded workspace %q: %w", template, walkErr)
		}
		rel := strings.TrimPrefix(path, template)
		rel = strings.TrimPrefix(rel, "/")
		if rel == "" {
			return nil
		}
		dst := filepath.Join(dstRoot, filepath.FromSlash(rel))
		if d.IsDir() {
			if err := os.MkdirAll(dst, 0o755); err != nil {
				return fmt.Errorf("create workspace dir %q: %w", dst, err)
			}
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return fmt.Errorf("read embedded workspace file info %q: %w", path, err)
		}
		data, err := fs.ReadFile(workspaceTemplateFS, path)
		if err != nil {
			return fmt.Errorf("read embedded workspace file %q: %w", path, err)
		}
		if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
			return fmt.Errorf("create workspace parent %q: %w", filepath.Dir(dst), err)
		}
		mode := info.Mode().Perm()
		if mode == 0 {
			mode = 0o644
		}
		mode |= 0o200
		if err := os.WriteFile(dst, data, mode); err != nil {
			return fmt.Errorf("write workspace file %q: %w", dst, err)
		}
		return nil
	})
}
