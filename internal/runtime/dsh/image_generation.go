package dsh

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"strings"
	"time"

	"csgclaw/internal/agentengine/contract"
)

const (
	imageBridgeURLEnvName   = "CSGCLAW_DSH_IMAGE_BRIDGE_URL"
	imageBridgeTokenEnvName = "CSGCLAW_DSH_IMAGE_BRIDGE_TOKEN"
)

// This native tool uses the exact executing Session and call ID. The host owns
// provider credentials, image generation, and acknowledged channel delivery.
const imageGenerationBridgeModule = `export const name = 'csgclaw-image-generation';
export const inject = ['tools'];
export function apply(ctx) {
  if (process.env.DSH_PERMISSION_MODE === 'read-only') return;
  const url = process.env.CSGCLAW_DSH_IMAGE_BRIDGE_URL;
  const token = process.env.CSGCLAW_DSH_IMAGE_BRIDGE_TOKEN;
  if (!url || !token) throw new Error('CSGClaw image bridge is unavailable');
  ctx.effect(() => ctx.tools.register({
    name: 'csgclaw_generate_image',
    description: "Generate one image from the user's requested description using this Agent's configured image generation model and deliver it to the current conversation. Use this tool whenever the user asks to create an image. Never switch the chat model or call providers with shell commands. If image_model_not_configured is returned, ask the user to configure the Image generation model in the Agent profile; do not retry automatically.",
    parameters: {type:'object',properties:{prompt:{type:'string',minLength:1,maxLength:32000}},required:['prompt'],additionalProperties:false},
    output: {schema:{type:'string'},render:(_args,value) => [{type:'text',text:value}]},
    async execute(args, exec) {
      if (!exec.agent) throw new Error('Image generation requires an active conversation');
      const response = await fetch(url, {
        method: 'POST',
        headers: {'Content-Type':'application/json',Authorization:'Bearer ' + token},
        body: JSON.stringify({sessionId:exec.agent.session.id,callId:exec.callId,prompt:args.prompt}),
        signal: exec.signal,
      });
      if (!response.ok) throw new Error('CSGClaw image bridge request failed');
      const result = await response.json();
      if (!result.success) throw new Error(result.message);
      return result.message;
    },
  }));
}
`

type imageGenerationRequest struct {
	SessionID string `json:"sessionId"`
	CallID    string `json:"callId"`
	Prompt    string `json:"prompt"`
}

// Each ACP process owns an authenticated loopback endpoint. Calls resolve only
// the supplied native Session's active turn, never another conversation.
func startImageGenerationBridge(proc *process) (*http.Server, []string, error) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, nil, fmt.Errorf("start DSH image bridge: %w", err)
	}
	token := rand.Text()
	server := &http.Server{ReadHeaderTimeout: 5 * time.Second, Handler: http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		if request.Header.Get("Authorization") != "Bearer "+token {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		if request.Method != http.MethodPost || request.URL.Path != "/generate-image" {
			http.NotFound(w, request)
			return
		}
		var args imageGenerationRequest
		decoder := json.NewDecoder(http.MaxBytesReader(w, request.Body, 128<<10))
		decoder.DisallowUnknownFields()
		if decoder.Decode(&args) != nil {
			http.Error(w, "invalid image generation arguments", http.StatusBadRequest)
			return
		}
		err := proc.generateImage(request.Context(), args)
		message := "Image generated and delivered to the current conversation. Do not generate again or publish another copy. Continue responding using the original chat model."
		if err != nil {
			message = err.Error()
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"success": err == nil, "message": message})
	})}
	go func() { _ = server.Serve(listener) }()
	return server, []string{imageBridgeURLEnvName + "=http://" + listener.Addr().String() + "/generate-image", imageBridgeTokenEnvName + "=" + token}, nil
}

func (proc *process) generateImage(ctx context.Context, args imageGenerationRequest) error {
	if proc.meta.PermissionMode == PermissionModeReadOnly {
		return fmt.Errorf("image generation is unavailable in read-only mode")
	}
	if strings.TrimSpace(args.SessionID) == "" || strings.TrimSpace(args.CallID) == "" || strings.TrimSpace(args.Prompt) == "" || len(args.Prompt) > 32000 {
		return fmt.Errorf("invalid image generation arguments")
	}
	proc.mu.Lock()
	turn := proc.active[args.SessionID]
	proc.mu.Unlock()
	if turn == nil || turn.ctx == nil || turn.ctx.Err() != nil {
		return fmt.Errorf("image generation requires an active turn")
	}
	callCtx, cancel := context.WithCancel(turn.ctx)
	defer cancel()
	stop := context.AfterFunc(ctx, cancel)
	defer stop()
	if err := ctx.Err(); err != nil {
		return err
	}
	return contract.GenerateImage(callCtx, args.CallID, args.Prompt)
}
