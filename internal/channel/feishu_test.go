package channel

import (
	"os"
	"path/filepath"
	"testing"
)

func TestFeishuServiceDoesNotPersistState(t *testing.T) {
	statePath := filepath.Join(t.TempDir(), "channels", "feishu", "state.json")
	svc := NewFeishuService()

	if _, err := svc.CreateUser(FeishuCreateUserRequest{ID: "fsu-alice", Name: "Alice"}); err != nil {
		t.Fatalf("CreateUser() error = %v", err)
	}

	if _, err := os.Stat(statePath); !os.IsNotExist(err) {
		t.Fatalf("state.json exists after Feishu operation; stat error = %v", err)
	}
}

func TestFeishuServiceKeepsNamedAppConfigs(t *testing.T) {
	svc := NewFeishuService(map[string]FeishuAppConfig{
		"manager": {
			AppID:     "cli_manager",
			AppSecret: "manager-secret",
		},
		"dev": {
			AppID:     "cli_dev",
			AppSecret: "dev-secret",
		},
	})

	apps := svc.AppConfigs()
	if got, want := apps["manager"].AppID, "cli_manager"; got != want {
		t.Fatalf("manager app_id = %q, want %q", got, want)
	}
	if got, want := apps["dev"].AppSecret, "dev-secret"; got != want {
		t.Fatalf("dev app_secret = %q, want %q", got, want)
	}

	apps["manager"] = FeishuAppConfig{AppID: "mutated"}
	if got, want := svc.AppConfigs()["manager"].AppID, "cli_manager"; got != want {
		t.Fatalf("manager app_id after caller mutation = %q, want %q", got, want)
	}
}
