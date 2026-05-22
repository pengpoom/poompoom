package cliproxy

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"path/filepath"
	"strings"
	"time"

	"imagestudio/internal/outboundproxy"
)

type AuthFileInfo struct {
	Name      string `json:"name"`
	Type      string `json:"type"`
	Provider  string `json:"provider"`
	Email     string `json:"email"`
	Disabled  bool   `json:"disabled"`
	Note      string `json:"note"`
	Priority  int    `json:"priority"`
	AuthIndex string `json:"auth_index"`
}

type Client struct {
	enabled       bool
	baseURL       string
	managementKey string
	httpClient    *http.Client
	providerType  string
}

func New(enabled bool, baseURL, managementKey, providerType string, timeout time.Duration, proxyURL ...string) *Client {
	if timeout <= 0 {
		timeout = 20 * time.Second
	}
	transport, err := outboundproxy.NewHTTPTransport(firstProxyURL(proxyURL...))
	if err != nil {
		panic(err)
	}
	return &Client{
		enabled:       enabled,
		baseURL:       strings.TrimRight(strings.TrimSpace(baseURL), "/"),
		managementKey: normalizeManagementKey(managementKey),
		httpClient:    &http.Client{Timeout: timeout, Transport: transport},
		providerType:  strings.TrimSpace(providerType),
	}
}

func (c *Client) Configured() bool {
	return c != nil && c.enabled && c.baseURL != "" && c.managementKey != ""
}

func (c *Client) ListAuthFiles(ctx context.Context) (map[string]AuthFileInfo, error) {
	if !c.Configured() {
		return nil, fmt.Errorf("cliproxy sync is not configured")
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/v0/management/auth-files", nil)
	if err != nil {
		return nil, err
	}
	c.setHeaders(req)
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return nil, fmt.Errorf("list auth files failed: HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var payload struct {
		Files []AuthFileInfo `json:"files"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return nil, err
	}

	result := make(map[string]AuthFileInfo, len(payload.Files))
	for _, item := range payload.Files {
		if !c.matchesProvider(item.Type, item.Provider) {
			continue
		}
		result[item.Name] = item
	}
	return result, nil
}

func (c *Client) DownloadAuthFile(ctx context.Context, name string) ([]byte, error) {
	if !c.Configured() {
		return nil, fmt.Errorf("cliproxy sync is not configured")
	}

	values := url.Values{}
	values.Set("name", filepath.Base(strings.TrimSpace(name)))

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/v0/management/auth-files/download?"+values.Encode(), nil)
	if err != nil {
		return nil, err
	}
	c.setHeaders(req)
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("download auth file failed: HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	return body, nil
}

func (c *Client) UploadAuthFile(ctx context.Context, name string, data []byte) error {
	if !c.Configured() {
		return fmt.Errorf("cliproxy sync is not configured")
	}

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, err := writer.CreateFormFile("file", filepath.Base(strings.TrimSpace(name)))
	if err != nil {
		return err
	}
	if _, err := part.Write(data); err != nil {
		return err
	}
	if err := writer.Close(); err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/v0/management/auth-files", body)
	if err != nil {
		return err
	}
	c.setHeaders(req)
	req.Header.Set("Content-Type", writer.FormDataContentType())

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusCreated || resp.StatusCode == http.StatusConflict {
		return nil
	}
	payload, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
	return fmt.Errorf("upload auth file failed: HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(payload)))
}

func (c *Client) PatchAuthFileStatus(ctx context.Context, name string, disabled bool) error {
	if !c.Configured() {
		return fmt.Errorf("cliproxy sync is not configured")
	}

	payload, err := json.Marshal(map[string]any{
		"name":     filepath.Base(strings.TrimSpace(name)),
		"disabled": disabled,
	})
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPatch, c.baseURL+"/v0/management/auth-files/status", bytes.NewReader(payload))
	if err != nil {
		return err
	}
	c.setHeaders(req)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusNoContent {
		return nil
	}
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
	return fmt.Errorf("patch auth file status failed: HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
}

func (c *Client) setHeaders(req *http.Request) {
	req.Header.Set("Authorization", "Bearer "+c.managementKey)
}

func (c *Client) matchesProvider(providerType, provider string) bool {
	expected := strings.ToLower(strings.TrimSpace(c.providerType))
	if expected == "" {
		return true
	}

	candidates := []string{
		strings.ToLower(strings.TrimSpace(providerType)),
		strings.ToLower(strings.TrimSpace(provider)),
	}
	for _, candidate := range candidates {
		if candidate == expected || candidate == "" {
			if candidate == expected {
				return true
			}
		}
	}
	return false
}

func firstProxyURL(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}

func normalizeManagementKey(value string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return ""
	}
	if strings.EqualFold(trimmed, "your-cliproxy-management-key") {
		return ""
	}
	return trimmed
}
