package feishu

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"csgclaw/internal/config"
)

func TestFileStoreLoadIfExistsMissingFile(t *testing.T) {
	store := NewFileStore(filepath.Join(t.TempDir(), config.ConfigFileName))
	snapshot, ok, err := store.LoadIfExists()
	if err != nil {
		t.Fatalf("LoadIfExists() error = %v", err)
	}
	if ok {
		t.Fatalf("LoadIfExists() ok = true, want false")
	}
	if snapshot.AdminOpenID != "" || snapshot.Bots != nil {
		t.Fatalf("LoadIfExists() snapshot = %+v, want zero snapshot", snapshot)
	}
}

func TestFileStoreSaveAndLoadRoundTrip(t *testing.T) {
	dir := t.TempDir()
	store := NewFileStore(filepath.Join(dir, config.ConfigFileName))

	want := Snapshot{
		AdminOpenID: "ou_admin",
		Bots: map[string]AppConfig{
			"u-dev": {
				AppID:     `cli_"dev"`,
				AppSecret: `sec-$DOLLAR-quote-"-slash-\\`,
			},
			"u-manager": {
				AppID:     "cli_manager",
				AppSecret: "manager-secret",
			},
		},
	}
	if err := store.Save(want); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	path := filepath.Join(dir, config.ChannelsDirName, FeishuChannelConfigFileName)
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("Stat() error = %v", err)
	}
	if got, wantMode := info.Mode().Perm(), os.FileMode(0o600); got != wantMode {
		t.Fatalf("feishu config mode = %v, want %v", got, wantMode)
	}

	loaded, err := store.Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if got, wantID := loaded.AdminOpenID, "ou_admin"; got != wantID {
		t.Fatalf("AdminOpenID = %q, want %q", got, wantID)
	}
	if got, wantID := loaded.Bots["u-dev"].AppID, `cli_"dev"`; got != wantID {
		t.Fatalf("u-dev app_id = %q, want %q", got, wantID)
	}
	if got, wantSecret := loaded.Bots["u-dev"].AppSecret, `sec-$DOLLAR-quote-"-slash-\\`; got != wantSecret {
		t.Fatalf("u-dev app_secret = %q, want %q", got, wantSecret)
	}
	if got, wantID := loaded.Bots["u-manager"].AdminOpenID, "ou_admin"; got != wantID {
		t.Fatalf("u-manager admin_open_id = %q, want %q", got, wantID)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	content := string(data)
	for _, wantLine := range []string{
		"[global]",
		`admin_open_id = "ou_admin"`,
		"[bots.u-dev]",
		`app_secret = "sec-$DOLLAR-quote-\"-slash-\\\\"`,
		"[bots.u-manager]",
	} {
		if !strings.Contains(content, wantLine) {
			t.Fatalf("saved config missing %q:\n%s", wantLine, content)
		}
	}
}

func TestFileStoreLoadRejectsInvalidBotID(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, config.ConfigFileName)
	feishuPath := filepath.Join(dir, config.ChannelsDirName, FeishuChannelConfigFileName)
	if err := os.MkdirAll(filepath.Dir(feishuPath), 0o700); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	if err := os.WriteFile(feishuPath, []byte("[bots.u.dev]\napp_id = \"cli\"\n"), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	_, err := NewFileStore(configPath).Load()
	if err == nil {
		t.Fatalf("Load() error = nil, want invalid bot id")
	}
	if !strings.Contains(err.Error(), `invalid bot_id "u.dev"`) {
		t.Fatalf("Load() error = %v, want invalid bot id", err)
	}
}

func TestFileStoreLoadMissingFileReturnsNotExist(t *testing.T) {
	store := NewFileStore(filepath.Join(t.TempDir(), config.ConfigFileName))
	_, err := store.Load()
	if !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("Load() error = %v, want os.ErrNotExist", err)
	}
}
