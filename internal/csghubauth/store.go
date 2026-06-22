package csghubauth

import (
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"csgclaw/internal/config"
)

const (
	authDirName  = "auth"
	authFileName = "csghub.json"

	DefaultAIGatewayBaseURL = "https://ai.space.opencsg.com/v1"
)

type Record struct {
	AIGatewayBuiltinAPIKey string    `json:"ai_gateway_builtin_api_key"`
	AccessToken            string    `json:"access_token"`
	UserID                 string    `json:"user_id"`
	UserUUID               string    `json:"user_uuid"`
	Avatar                 string    `json:"avatar"`
	CSGHubBaseURL          string    `json:"csghub_base_url"`
	PortalURL              string    `json:"portal_url"`
	LoggedInAt             time.Time `json:"logged_in_at"`
}

type Status struct {
	Authenticated bool       `json:"authenticated"`
	UserID        string     `json:"user_id,omitempty"`
	UserUUID      string     `json:"user_uuid,omitempty"`
	Avatar        string     `json:"avatar,omitempty"`
	CSGHubBaseURL string     `json:"csghub_base_url,omitempty"`
	PortalURL     string     `json:"portal_url,omitempty"`
	LoggedInAt    *time.Time `json:"logged_in_at,omitempty"`
}

type Store struct {
	path string
}

func NewStore(path string) Store {
	return Store{path: strings.TrimSpace(path)}
}

func DefaultPath() (string, error) {
	dir, err := config.DefaultDomainDir(authDirName)
	if err != nil {
		return "", fmt.Errorf("resolve csghub auth dir: %w", err)
	}
	return filepath.Join(dir, authFileName), nil
}

func DefaultStore() (Store, error) {
	path, err := DefaultPath()
	if err != nil {
		return Store{}, err
	}
	return NewStore(path), nil
}

func (s Store) Path() (string, error) {
	path := strings.TrimSpace(s.path)
	if path != "" {
		return path, nil
	}
	return DefaultPath()
}

func (s Store) Load() (Record, bool, error) {
	path, err := s.Path()
	if err != nil {
		return Record{}, false, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return Record{}, false, nil
		}
		return Record{}, false, fmt.Errorf("read csghub auth store: %w", err)
	}
	var record Record
	if err := json.Unmarshal(data, &record); err != nil {
		return Record{}, false, fmt.Errorf("decode csghub auth store: %w", err)
	}
	return normalizeRecord(record), true, nil
}

func (s Store) Save(record Record) error {
	path, err := s.Path()
	if err != nil {
		return err
	}
	record = normalizeRecord(record)
	if record.AccessToken == "" {
		return fmt.Errorf("csghub access token is required")
	}
	if record.CSGHubBaseURL == "" {
		return fmt.Errorf("csghub base url is required")
	}
	data, err := json.MarshalIndent(record, "", "  ")
	if err != nil {
		return fmt.Errorf("encode csghub auth store: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("create csghub auth dir: %w", err)
	}
	if err := os.WriteFile(path, append(data, '\n'), 0o600); err != nil {
		return fmt.Errorf("write csghub auth store: %w", err)
	}
	if err := os.Chmod(path, 0o600); err != nil {
		return fmt.Errorf("secure csghub auth store: %w", err)
	}
	return nil
}

func (s Store) Delete() error {
	path, err := s.Path()
	if err != nil {
		return err
	}
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("delete csghub auth store: %w", err)
	}
	return nil
}

func (s Store) Status() (Status, error) {
	record, ok, err := s.Load()
	if err != nil {
		return Status{}, err
	}
	if !ok {
		return Status{}, nil
	}
	return record.Status(), nil
}

func (s Store) Credentials() (baseURL, token string, ok bool, err error) {
	record, found, err := s.Load()
	if err != nil || !found {
		return "", "", false, err
	}
	baseURL = strings.TrimRight(strings.TrimSpace(record.CSGHubBaseURL), "/")
	token = strings.TrimSpace(record.AccessToken)
	return baseURL, token, baseURL != "" && token != "", nil
}

func (s Store) AIGatewayCredentials() (baseURL, apiKey string, ok bool, err error) {
	record, found, err := s.Load()
	if err != nil || !found {
		return "", "", false, err
	}
	baseURL = AIGatewayBaseURL(record.CSGHubBaseURL)
	apiKey = strings.TrimSpace(record.AIGatewayBuiltinAPIKey)
	if !isBuiltinAIGatewayAPIKey(apiKey) {
		apiKey = ""
	}
	return baseURL, apiKey, baseURL != "" && apiKey != "", nil
}

func (r Record) Status() Status {
	r = normalizeRecord(r)
	if r.AccessToken == "" {
		return Status{}
	}
	status := Status{
		Authenticated: true,
		UserID:        r.UserID,
		UserUUID:      r.UserUUID,
		Avatar:        r.Avatar,
		CSGHubBaseURL: r.CSGHubBaseURL,
		PortalURL:     r.PortalURL,
	}
	if !r.LoggedInAt.IsZero() {
		loggedInAt := r.LoggedInAt
		status.LoggedInAt = &loggedInAt
	}
	return status
}

func normalizeRecord(record Record) Record {
	record.AIGatewayBuiltinAPIKey = strings.TrimSpace(record.AIGatewayBuiltinAPIKey)
	record.AccessToken = strings.TrimSpace(record.AccessToken)
	record.UserID = strings.TrimSpace(record.UserID)
	record.UserUUID = strings.TrimSpace(record.UserUUID)
	record.Avatar = strings.TrimSpace(record.Avatar)
	record.CSGHubBaseURL = strings.TrimRight(strings.TrimSpace(record.CSGHubBaseURL), "/")
	record.PortalURL = strings.TrimSpace(record.PortalURL)
	return record
}

func AIGatewayBaseURL(_ string) string {
	if baseURL := normalizeAIGatewayBaseURL(os.Getenv("CSGHUB_AIGATEWAY_BASE_URL")); baseURL != "" {
		return baseURL
	}
	if baseURL := normalizeAIGatewayBaseURL(os.Getenv("CSGHUB_AIGATEWAY_URL")); baseURL != "" {
		return baseURL
	}
	return DefaultAIGatewayBaseURL
}

func normalizeAIGatewayBaseURL(raw string) string {
	raw = strings.TrimRight(strings.TrimSpace(raw), "/")
	if raw == "" {
		return ""
	}
	u, err := url.Parse(raw)
	if err != nil || u.Scheme == "" || u.Host == "" {
		return raw
	}
	if !strings.HasSuffix(strings.TrimRight(u.Path, "/"), "/v1") {
		u.Path = strings.TrimRight(u.Path, "/") + "/v1"
	}
	u.RawQuery = ""
	u.Fragment = ""
	return strings.TrimRight(u.String(), "/")
}
