package agent

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"csgclaw/internal/config"
	"csgclaw/internal/sandbox"
)

const (
	gatewayBoxPhaseIdle uint32 = iota
	gatewayBoxPhasePreparing
	gatewayBoxPhaseCreating
)

func (s *Service) setGatewayWorkPhase(p uint32) {
	if s == nil {
		return
	}
	s.gatewayWorkPhase.Store(p)
}

func (s *Service) createGatewayBox(ctx context.Context, rt sandbox.Runtime, image, name, botID string, modelCfg config.ModelConfig) (sandbox.Instance, sandbox.Info, error) {
	if testCreateGatewayBoxHook != nil {
		return testCreateGatewayBoxHook(s, ctx, rt, image, name, botID, modelCfg)
	}
	if rt == nil {
		return nil, sandbox.Info{}, fmt.Errorf("invalid sandbox runtime")
	}
	defer s.setGatewayWorkPhase(gatewayBoxPhaseIdle)

	s.setGatewayWorkPhase(gatewayBoxPhasePreparing)
	log.Printf(`gateway box %q: stage "preparing" — workspace dirs, PicoClaw/OpenClaw JSON config, mounts, and bundled skills layout on the host`, name)
	spec, err := s.gatewayCreateSpec(image, name, botID, modelCfg)
	if err != nil {
		return nil, sandbox.Info{}, err
	}
	img := strings.TrimSpace(spec.Image)
	s.setGatewayWorkPhase(gatewayBoxPhaseCreating)
	log.Printf(`gateway box %q: stage "creating" — boxlite sandbox create/start for image %q (image layers may download here if missing; gateway process starts after VM is up)`, name, img)
	box, err := rt.Create(ctx, spec)
	if err != nil {
		return nil, sandbox.Info{}, fmt.Errorf("create gateway box: %w", err)
	}
	log.Printf(`gateway box %q: sandbox instance record created; inspecting running state/metadata`, name)
	info, err := box.Info(ctx)
	if err != nil {
		_ = s.closeBox(box)
		return nil, sandbox.Info{}, fmt.Errorf("read gateway box info: %w", err)
	}
	return box, info, nil
}

func (s *Service) forceRemoveBox(ctx context.Context, rt sandbox.Runtime, idOrName string) error {
	if testForceRemoveBoxHook != nil {
		return testForceRemoveBoxHook(s, ctx, rt, idOrName)
	}
	if rt == nil {
		return fmt.Errorf("invalid sandbox runtime")
	}
	return rt.Remove(ctx, idOrName, sandbox.RemoveOptions{Force: true})
}

func (s *Service) gatewayCreateSpec(image, name, botID string, modelCfg config.ModelConfig) (sandbox.CreateSpec, error) {
	modelCfg = modelCfg.Resolved()
	if strings.TrimSpace(modelCfg.ModelID) == "" {
		modelCfg = s.model.Resolved()
	}
	modelID := modelCfg.ModelID
	managerBaseURL := resolveManagerBaseURL(s.server)
	llmBaseURL := llmBridgeBaseURL(managerBaseURL, botID)

	if s.useOpenClawGateway() {
		if _, err := ensureAgentOpenClawConfig(name, botID, s.server, modelCfg); err != nil {
			return sandbox.CreateSpec{}, err
		}
		if err := ensureOpenClawCsgSkills(name, botID); err != nil {
			return sandbox.CreateSpec{}, err
		}
	} else {
		if _, err := ensureAgentPicoClawConfig(name, botID, s.server, modelCfg); err != nil {
			return sandbox.CreateSpec{}, err
		}
	}

	envVars := picoclawBoxEnvVars(managerBaseURL, s.server.AccessToken, botID, llmBaseURL, modelID)
	addFeishuBoxEnvVars(envVars, botID, s.channels)
	var spec sandbox.CreateSpec
	if s.useOpenClawGateway() {
		envVars["HOME"] = boxOpenClawUserHome
		envVars["NODE_ENV"] = "production"
		spec = sandbox.CreateSpec{
			Image:      image,
			Name:       name,
			Detach:     true,
			AutoRemove: false,
			Env:        envVars,
			Cmd: []string{
				"/bin/sh",
				"-c",
				openClawStartScript(),
			},
		}
	} else {
		envVars["HOME"] = "/home/picoclaw"
		spec = sandbox.CreateSpec{
			Image:      image,
			Name:       name,
			Detach:     true,
			AutoRemove: false,
			Env:        envVars,
			Cmd: []string{
				"/bin/sh",
				"-c",
				"/usr/local/bin/picoclaw gateway -d 1>~/.picoclaw/gateway.log 2>/dev/null",
			},
		}
	}

	projectsRoot, err := ensureAgentProjectsRoot()
	if err != nil {
		return sandbox.CreateSpec{}, err
	}

	if s.useOpenClawGateway() {
		if _, err := ensureAgentOpenClawWorkspace(name, workspaceTemplateForAgent(name, botID, true)); err != nil {
			return sandbox.CreateSpec{}, err
		}
		hostOpenRoot, err := agentOpenClawRoot(name)
		if err != nil {
			return sandbox.CreateSpec{}, err
		}
		spec.Mounts = append(spec.Mounts, sandbox.Mount{
			HostPath:  hostOpenRoot,
			GuestPath: boxOpenClawDir,
		}, sandbox.Mount{
			HostPath:  projectsRoot,
			GuestPath: boxOpenClawProjectsDir,
		})
		return spec, nil
	}

	hostWorkspaceRoot, err := ensureAgentWorkspace(name, workspaceTemplateForAgent(name, botID, false))
	if err != nil {
		return sandbox.CreateSpec{}, err
	}
	for _, mount := range gatewayVolumeMounts(hostWorkspaceRoot, projectsRoot) {
		spec.Mounts = append(spec.Mounts, sandbox.Mount{
			HostPath:  mount.hostPath,
			GuestPath: mount.guestPath,
		})
	}
	return spec, nil
}

type gatewayVolumeMount struct {
	hostPath  string
	guestPath string
}

func gatewayVolumeMounts(hostWorkspaceRoot, projectsRoot string) []gatewayVolumeMount {
	return []gatewayVolumeMount{
		{
			hostPath:  hostWorkspaceRoot,
			guestPath: boxWorkspaceDir,
		},
		{
			hostPath:  projectsRoot,
			guestPath: boxProjectsDir,
		},
	}
}

// openClawStartScript launches the OpenClaw gateway. csgclaw-cli is baked into
// the OpenClaw image (see csgclaw-extension/Dockerfile) under /usr/local/bin,
// so the agent does not need to fetch or install it at runtime.
//
// The "gateway stop" prefix gracefully terminates any pre-existing gateway
// instance before starting a new one, preventing a port-already-in-use error
// if this CMD is re-executed on an already-running box.
func openClawStartScript() string {
	// Stop any pre-existing gateway (same-VM restart guard), then wait briefly so
	// the port is fully released before the new process tries to bind it.
	return "node /app/openclaw.mjs gateway stop 2>/dev/null; sleep 2; exec node /app/openclaw.mjs gateway --allow-unconfigured --bind lan --port 18789 1>>" + openClawGatewayLog + " 2>&1"
}

func gatewayStartCommand(debug bool) ([]string, []string) {
	if debug {
		return []string{"sleep"}, []string{"infinity"}
	}
	return []string{"tini"}, []string{"--", "picoclaw", "gateway", "-d"}
}

func ensureAgentProjectsRoot() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolve host home dir: %w", err)
	}
	hostProjectsRoot := filepath.Join(homeDir, config.AppDirName, hostProjectsDir)
	if err := os.MkdirAll(hostProjectsRoot, 0o755); err != nil {
		return "", fmt.Errorf("create host projects dir: %w", err)
	}
	return hostProjectsRoot, nil
}

func ProjectsRoot() (string, error) {
	return ensureAgentProjectsRoot()
}

func llmBridgeBaseURL(managerBaseURL, botID string) string {
	managerBaseURL = strings.TrimRight(strings.TrimSpace(managerBaseURL), "/")
	return managerBaseURL + "/api/bots/" + strings.TrimSpace(botID) + "/llm"
}

func bridgeLLMEnvVars(llmBaseURL, accessToken, modelID string) map[string]string {
	return map[string]string{
		"CSGCLAW_LLM_BASE_URL": llmBaseURL,
		"CSGCLAW_LLM_API_KEY":  accessToken,
		"CSGCLAW_LLM_MODEL_ID": modelID,
		"OPENAI_BASE_URL":      llmBaseURL,
		"OPENAI_API_KEY":       accessToken,
		"OPENAI_MODEL":         modelID,
	}
}

func picoclawBoxEnvVars(baseURL, accessToken, botID, llmBaseURL, modelID string) map[string]string {
	env := bridgeLLMEnvVars(llmBaseURL, accessToken, modelID)
	picoclawModelID := picoclawBridgeModelID(modelID)
	env["CSGCLAW_BASE_URL"] = baseURL
	env["CSGCLAW_ACCESS_TOKEN"] = accessToken
	env["PICOCLAW_CHANNELS_CSGCLAW_BASE_URL"] = baseURL
	env["PICOCLAW_CHANNELS_CSGCLAW_ACCESS_TOKEN"] = accessToken
	env["PICOCLAW_CHANNELS_CSGCLAW_BOT_ID"] = botID
	env["PICOCLAW_AGENTS_DEFAULTS_MODEL_NAME"] = modelID
	env["PICOCLAW_CUSTOM_MODEL_NAME"] = modelID
	env["PICOCLAW_CUSTOM_MODEL_ID"] = picoclawModelID
	env["PICOCLAW_CUSTOM_MODEL_API_KEY"] = accessToken
	env["PICOCLAW_CUSTOM_MODEL_BASE_URL"] = llmBaseURL
	return env
}

func addFeishuBoxEnvVars(envVars map[string]string, botID string, channels config.ChannelsConfig) {
	if envVars == nil {
		return
	}
	botID = strings.TrimSpace(botID)
	if botID == "" || len(channels.Feishu) == 0 {
		return
	}
	feishu, ok := channels.Feishu[botID]
	if !ok {
		return
	}
	envVars["PICOCLAW_CHANNELS_FEISHU_APP_ID"] = feishu.AppID
	envVars["PICOCLAW_CHANNELS_FEISHU_APP_SECRET"] = feishu.AppSecret
}
