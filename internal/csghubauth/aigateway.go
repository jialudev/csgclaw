package csghubauth

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

func (s Store) EnsureAIGatewayCredentials(ctx context.Context, client *http.Client) (baseURL, apiKey string, ok bool, err error) {
	record, found, err := s.Load()
	if err != nil || !found {
		return "", "", false, err
	}
	baseURL = AIGatewayBaseURL(record.CSGHubBaseURL)
	apiKey = strings.TrimSpace(record.AIGatewayBuiltinAPIKey)
	if apiKey != "" && isBuiltinAIGatewayAPIKey(apiKey) {
		return baseURL, apiKey, baseURL != "", nil
	}
	if strings.TrimSpace(record.AccessToken) == "" {
		return baseURL, "", false, nil
	}
	if strings.TrimSpace(record.CSGHubBaseURL) == "" {
		return baseURL, "", false, fmt.Errorf("csghub base url is required to fetch aigateway api key")
	}
	if strings.TrimSpace(record.UserID) == "" {
		return baseURL, "", false, fmt.Errorf("csghub user id is required to fetch aigateway api key")
	}
	if strings.TrimSpace(record.UserUUID) == "" {
		return baseURL, "", false, fmt.Errorf("csghub user uuid is required to fetch aigateway api key")
	}

	apiKey, err = fetchBuiltinAPIKey(ctx, client, record.CSGHubBaseURL, record.AccessToken, record.UserID, record.UserUUID)
	if err != nil {
		return baseURL, "", false, err
	}
	record.AIGatewayBuiltinAPIKey = apiKey
	if err := s.Save(record); err != nil {
		return "", "", false, err
	}
	return baseURL, apiKey, baseURL != "" && apiKey != "", nil
}

func fetchBuiltinAPIKey(ctx context.Context, client *http.Client, baseURL, authToken, userID, namespaceUUID string) (string, error) {
	authToken = strings.TrimSpace(authToken)
	if authToken == "" {
		return "", fmt.Errorf("csghub auth token is required")
	}
	namespaceUUID = strings.TrimSpace(namespaceUUID)
	if namespaceUUID == "" {
		return "", fmt.Errorf("namespace uuid is required")
	}
	endpoint, err := joinAPIPath(baseURL, "/api/v1/namespaces/"+url.PathEscape(namespaceUUID)+"/apikeys/builtin")
	if err != nil {
		return "", err
	}
	if userID = strings.TrimSpace(userID); userID != "" {
		q := endpoint.Query()
		q.Set("current_user", userID)
		endpoint.RawQuery = q.Encode()
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint.String(), nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", "Bearer "+authToken)

	var resp struct {
		Msg  string `json:"msg"`
		Data struct {
			Token string `json:"token"`
		} `json:"data"`
	}
	if err := doJSONWithClient(client, req, &resp); err != nil {
		return "", fmt.Errorf("get csghub ai gateway builtin api key: %w", err)
	}
	token := strings.TrimSpace(resp.Data.Token)
	if token == "" {
		return "", fmt.Errorf("get csghub ai gateway builtin api key: token not found")
	}
	return token, nil
}

func isBuiltinAIGatewayAPIKey(apiKey string) bool {
	return strings.HasPrefix(strings.TrimSpace(apiKey), "gk_")
}

func doJSONWithClient(client *http.Client, req *http.Request, out any) error {
	if client == nil {
		client = &http.Client{Timeout: 10 * time.Second}
	}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		msg := strings.TrimSpace(string(body))
		if msg == "" {
			msg = resp.Status
		}
		return fmt.Errorf("http %d: %s", resp.StatusCode, msg)
	}
	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		return fmt.Errorf("decode response: %w", err)
	}
	return nil
}
