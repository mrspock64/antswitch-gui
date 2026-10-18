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

type displayResponse struct {
	Flip bool `json:"flip"`
}

// GetFlip reads whether the device's own screen is flipped 180 degrees.
func (c *AntennaSwitchClient) GetFlip(ctx context.Context, host string) (bool, error) {
	return c.doDisplay(ctx, normalizeHost(host)+"/api/display")
}

// SetFlip flips (or un-flips) the device's own screen; it is stored on the
// device and applied immediately.
func (c *AntennaSwitchClient) SetFlip(ctx context.Context, host, token string, flip bool) (bool, error) {
	v := 0
	if flip {
		v = 1
	}
	u := fmt.Sprintf("%s/api/display?flip=%d&token=%s", normalizeHost(host), v, url.QueryEscape(token))
	return c.doDisplay(ctx, u)
}

func (c *AntennaSwitchClient) doDisplay(ctx context.Context, u string) (bool, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return false, err
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		var apiErr apiError
		if err := json.NewDecoder(resp.Body).Decode(&apiErr); err == nil && apiErr.Error != "" {
			return false, fmt.Errorf("%s", apiErr.Error)
		}
		return false, fmt.Errorf("unexpected status: %s", resp.Status)
	}
	var d displayResponse
	if err := json.NewDecoder(resp.Body).Decode(&d); err != nil {
		return false, err
	}
	return d.Flip, nil
}
