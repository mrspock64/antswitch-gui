package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const requestTimeout = 3 * time.Second

// StatusResponse mirrors the AT-14's /api/status and /api/select JSON body.
type StatusResponse struct {
	Active int      `json:"active"`
	Names  []string `json:"names"`
}

type apiError struct {
	Error string `json:"error"`
}

// AntennaSwitchClient talks to a single AT-14 device over HTTP.
type AntennaSwitchClient struct {
	httpClient *http.Client
}

func newAntennaSwitchClient() *AntennaSwitchClient {
	return &AntennaSwitchClient{
		httpClient: &http.Client{Timeout: requestTimeout},
	}
}

func normalizeHost(host string) string {
	host = strings.TrimSpace(host)
	if host == "" {
		return host
	}
	if !strings.HasPrefix(host, "http://") && !strings.HasPrefix(host, "https://") {
		host = "http://" + host
	}
	return strings.TrimSuffix(host, "/")
}

func (c *AntennaSwitchClient) GetStatus(ctx context.Context, host string) (*StatusResponse, error) {
	return c.doGet(ctx, normalizeHost(host)+"/api/status")
}

func (c *AntennaSwitchClient) SelectAntenna(ctx context.Context, host, token string, ant int) (*StatusResponse, error) {
	u := fmt.Sprintf("%s/api/select?ant=%d&token=%s", normalizeHost(host), ant, url.QueryEscape(token))
	return c.doGet(ctx, u)
}

func (c *AntennaSwitchClient) doGet(ctx context.Context, u string) (*StatusResponse, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		var apiErr apiError
		if err := json.NewDecoder(resp.Body).Decode(&apiErr); err == nil && apiErr.Error != "" {
			return nil, fmt.Errorf("%s", apiErr.Error)
		}
		return nil, fmt.Errorf("unexpected status: %s", resp.Status)
	}

	var status StatusResponse
	if err := json.NewDecoder(resp.Body).Decode(&status); err != nil {
		return nil, err
	}
	return &status, nil
}
