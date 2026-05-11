package feishu

import (
	"path/filepath"
	"reflect"
	"testing"

	"csgclaw/internal/config"
)

func TestProviderLoadsInitialSnapshotAndBotConfig(t *testing.T) {
	dir := t.TempDir()
	store := NewFileStore(filepath.Join(dir, config.ConfigFileName))
	if err := store.Save(Snapshot{
		AdminOpenID: "ou_admin",
		Bots: map[string]AppConfig{
			"u-dev": {AppID: "cli_dev", AppSecret: "dev-secret"},
		},
	}); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	provider, err := NewProvider(store)
	if err != nil {
		t.Fatalf("NewProvider() error = %v", err)
	}
	app, ok := provider.BotConfig("u-dev")
	if !ok {
		t.Fatalf("BotConfig() ok = false, want true")
	}
	if got, want := app.AdminOpenID, "ou_admin"; got != want {
		t.Fatalf("BotConfig().AdminOpenID = %q, want %q", got, want)
	}

	snapshot := provider.Snapshot()
	if got, want := snapshot.Bots["u-dev"].AppSecret, "dev-secret"; got != want {
		t.Fatalf("Snapshot().Bots[u-dev].AppSecret = %q, want %q", got, want)
	}
}

func TestProviderUpdatePersistsSnapshot(t *testing.T) {
	dir := t.TempDir()
	store := NewFileStore(filepath.Join(dir, config.ConfigFileName))
	provider, err := NewProvider(store)
	if err != nil {
		t.Fatalf("NewProvider() error = %v", err)
	}

	view, snapshot, err := provider.Update(Update{
		BotID:       "u-dev",
		AppID:       "cli_dev",
		AppSecret:   "dev-secret",
		AdminOpenID: "ou_admin",
	})
	if err != nil {
		t.Fatalf("Update() error = %v", err)
	}
	if !view.Configured || !view.HasSecret || view.AppID != "cli_dev" || view.AdminOpenID != "ou_admin" {
		t.Fatalf("Update() view = %+v, want masked configured view", view)
	}
	if got, want := snapshot.Bots["u-dev"].AdminOpenID, "ou_admin"; got != want {
		t.Fatalf("Update() snapshot admin_open_id = %q, want %q", got, want)
	}

	reloaded, err := store.Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if !reflect.DeepEqual(reloaded, snapshot) {
		t.Fatalf("persisted snapshot = %#v, want %#v", reloaded, snapshot)
	}
}

func TestProviderReloadRefreshesSnapshotAndCallsHook(t *testing.T) {
	dir := t.TempDir()
	store := NewFileStore(filepath.Join(dir, config.ConfigFileName))
	provider, err := NewProvider(store)
	if err != nil {
		t.Fatalf("NewProvider() error = %v", err)
	}

	if _, _, err := provider.Update(Update{
		BotID:       "u-dev",
		AppID:       "cli_dev",
		AppSecret:   "dev-secret",
		AdminOpenID: "ou_old",
	}); err != nil {
		t.Fatalf("Update() error = %v", err)
	}
	if err := store.Save(Snapshot{
		AdminOpenID: "ou_new",
		Bots: map[string]AppConfig{
			"u-worker": {AppID: "cli_worker", AppSecret: "worker-secret"},
		},
	}); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	var hookSnapshot Snapshot
	provider.SetReloadHook(func(snapshot Snapshot) {
		hookSnapshot = snapshot
	})

	snapshot, err := provider.Reload()
	if err != nil {
		t.Fatalf("Reload() error = %v", err)
	}
	if got, want := sortedSnapshotBotIDs(snapshot), []string{"u-worker"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("Reload() bot ids = %#v, want %#v", got, want)
	}
	if got, want := hookSnapshot.AdminOpenID, "ou_new"; got != want {
		t.Fatalf("reload hook admin_open_id = %q, want %q", got, want)
	}
	if _, ok := provider.BotConfig("u-dev"); ok {
		t.Fatalf("BotConfig(u-dev) ok = true, want false after reload")
	}
}

func TestProviderUpdateValidatesRequest(t *testing.T) {
	provider, err := NewProvider(NewFileStore(filepath.Join(t.TempDir(), config.ConfigFileName)))
	if err != nil {
		t.Fatalf("NewProvider() error = %v", err)
	}
	_, _, err = provider.Update(Update{BotID: "u-dev", AppSecret: "secret"})
	if err == nil || !IsValidationError(err) || err.Error() != "app_id is required" {
		t.Fatalf("Update() error = %v, want app_id validation error", err)
	}
}
