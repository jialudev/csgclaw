package codex

import (
	"context"
	"testing"

	agentruntime "csgclaw/internal/runtime"
)

func TestValidateConfigResolvesOpenCSGCredentialsForResponsesProbe(t *testing.T) {
	restoreProbe := TestOnlySetResponsesAPIProbe(func(_ context.Context, baseURL, apiKey, modelID string, headers map[string]string) error {
		if baseURL != "https://ai.space.opencsg.com/v1" {
			t.Fatalf("probe baseURL = %q, want OpenCSG AIGateway", baseURL)
		}
		if apiKey != "gateway-key" {
			t.Fatalf("probe apiKey = %q, want gateway-key", apiKey)
		}
		if modelID != "glm-5.1" {
			t.Fatalf("probe modelID = %q, want glm-5.1", modelID)
		}
		if len(headers) != 0 {
			t.Fatalf("probe headers = %#v, want empty", headers)
		}
		return nil
	})
	defer restoreProbe()

	previousCreds := openCSGCredentialsForResponsesProbe
	openCSGCredentialsForResponsesProbe = func(context.Context) (string, string, bool, error) {
		return "https://ai.space.opencsg.com/v1", "gateway-key", true, nil
	}
	defer func() {
		openCSGCredentialsForResponsesProbe = previousCreds
	}()

	rt := &Runtime{}
	err := rt.ValidateConfig(context.Background(), agentruntime.RuntimeConfigSnapshot{
		Profile: agentruntime.RuntimeProfileConfig{
			Provider: "csghub",
			ModelID:  "glm-5.1",
		},
	})
	if err != nil {
		t.Fatalf("ValidateConfig() error = %v", err)
	}
}
