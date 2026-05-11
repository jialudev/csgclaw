package feishu

import "sync"

type BotCredentialProvider interface {
	BotConfig(botID string) (AppConfig, bool)
}

type Provider interface {
	BotCredentialProvider
	Snapshot() Snapshot
	Reload() (Snapshot, error)
	Update(Update) (Entry, Snapshot, error)
	SetReloadHook(func(Snapshot))
}

type ConfigProvider struct {
	mu         sync.RWMutex
	store      Store
	snapshot   Snapshot
	reloadHook func(Snapshot)
}

func NewProvider(store Store) (*ConfigProvider, error) {
	p := &ConfigProvider{store: store}
	if store == nil {
		return p, nil
	}
	snapshot, ok, err := store.LoadIfExists()
	if err != nil {
		return nil, err
	}
	if ok {
		p.snapshot = cloneSnapshot(snapshot)
	}
	return p, nil
}

func (p *ConfigProvider) Snapshot() Snapshot {
	if p == nil {
		return Snapshot{}
	}
	p.mu.RLock()
	defer p.mu.RUnlock()
	return cloneSnapshot(p.snapshot)
}

func (p *ConfigProvider) BotConfig(botID string) (AppConfig, bool) {
	if p == nil {
		return AppConfig{}, false
	}
	botID, err := normalizeConfigBotID(botID)
	if err != nil {
		return AppConfig{}, false
	}
	p.mu.RLock()
	defer p.mu.RUnlock()
	app, ok := p.snapshot.Bots[botID]
	return app, ok
}

func (p *ConfigProvider) Reload() (Snapshot, error) {
	if p == nil || p.store == nil {
		return Snapshot{}, nil
	}
	snapshot, ok, err := p.store.LoadIfExists()
	if err != nil {
		return Snapshot{}, err
	}
	if !ok {
		snapshot = Snapshot{}
	}
	snapshot = cloneSnapshot(snapshot)

	p.mu.Lock()
	p.snapshot = snapshot
	hook := p.reloadHook
	p.mu.Unlock()

	if hook != nil {
		hook(cloneSnapshot(snapshot))
	}
	return cloneSnapshot(snapshot), nil
}

func (p *ConfigProvider) Update(req Update) (Entry, Snapshot, error) {
	if p == nil || p.store == nil {
		return Entry{}, Snapshot{}, nil
	}
	botID, appID, appSecret, adminOpenID, err := normalizeConfigUpdate(req)
	if err != nil {
		return Entry{}, Snapshot{}, err
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	snapshot, ok, err := p.store.LoadIfExists()
	if err != nil {
		return Entry{}, Snapshot{}, err
	}
	if !ok {
		snapshot = Snapshot{}
	}
	snapshot = normalizeSnapshot(snapshot)
	if snapshot.Bots == nil {
		snapshot.Bots = make(map[string]AppConfig)
	}
	if adminOpenID != "" {
		snapshot.AdminOpenID = adminOpenID
		for id, app := range snapshot.Bots {
			app.AdminOpenID = snapshot.AdminOpenID
			snapshot.Bots[id] = app
		}
	}
	snapshot.Bots[botID] = AppConfig{
		AppID:       appID,
		AppSecret:   appSecret,
		AdminOpenID: snapshot.AdminOpenID,
	}
	snapshot = normalizeSnapshot(snapshot)
	if err := p.store.Save(snapshot); err != nil {
		return Entry{}, Snapshot{}, err
	}
	p.snapshot = cloneSnapshot(snapshot)
	return MaskAppConfig(botID, snapshot.Bots[botID], true), cloneSnapshot(snapshot), nil
}

func (p *ConfigProvider) SetReloadHook(hook func(Snapshot)) {
	if p == nil {
		return
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	p.reloadHook = hook
}
