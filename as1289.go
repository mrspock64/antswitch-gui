package main

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// as1289PortCount is fixed: the Microbit AS-1289 is a 5-way HF switch.
const as1289PortCount = 5

// AS1289Client talks to a Microbit AS-1289 over its (undocumented, reverse
// engineered from the device's own /relays.js) query-string protocol. Unlike
// the AT-14 it has no CORS header, which is exactly why this needs a native
// HTTP client rather than a browser page.
type AS1289Client struct {
	httpClient *http.Client
}

func newAS1289Client() *AS1289Client {
	return &AS1289Client{
		httpClient: &http.Client{Timeout: requestTimeout},
	}
}

// GetStatus polls the same endpoint the device's own web page polls.
func (c *AS1289Client) GetStatus(ctx context.Context, host, authUser, authPass string, names []string) (*DeviceStatus, error) {
	u := fmt.Sprintf("%s/setswitch.htm?upd=%d", normalizeHost(host), time.Now().UnixMilli())
	body, err := c.doGet(ctx, u, authUser, authPass)
	if err != nil {
		return nil, err
	}
	return &DeviceStatus{Active: parseAS1289Active(body), Names: displayNames(names, as1289PortCount)}, nil
}

// SelectAntenna activates port idx+1 (the switch is exclusive, so every
// other port is released automatically). The endpoint's response body is
// empty, so a follow-up status fetch confirms what actually took effect.
//
// The device can boot into "Automatic control" mode, where it re-follows
// band changes from a connected radio via an RRC-1258 link and can revert
// a manual selection once one is plugged in. A manual ap<N> select works
// fine either way, but to make it stick we switch automatic mode off
// immediately beforehand (two quick calls in a row).
func (c *AS1289Client) SelectAntenna(ctx context.Context, host, authUser, authPass string, idx int, names []string) (*DeviceStatus, error) {
	autoOffURL := fmt.Sprintf("%s/setswitch.htm?ga&OFF&set=%d", normalizeHost(host), time.Now().UnixMilli())
	if _, err := c.doGet(ctx, autoOffURL, authUser, authPass); err != nil {
		return nil, err
	}

	port := idx + 1
	selectURL := fmt.Sprintf("%s/setswitch.htm?ap%d&ON%%20&set=%d", normalizeHost(host), port, time.Now().UnixMilli())
	if _, err := c.doGet(ctx, selectURL, authUser, authPass); err != nil {
		return nil, err
	}
	return c.GetStatus(ctx, host, authUser, authPass, names)
}

// as1289NameRe matches the antenna-name table cells in the device's own
// /setswitch.htm page, e.g. `id="ar1">OCD</td>`. An empty port renders as
// `id="ar3"></td>` (empty string between the tags, not a placeholder).
var as1289NameRe = regexp.MustCompile(`id="ar(\d)">([^<]*)</td>`)

// FetchNames scrapes antenna names off the device's own status page. The
// names aren't in the pipe-separated status protocol, and they change
// rarely (only when someone edits them on the device's own /settings page),
// so this is meant to be called once when the profile is set up or on
// demand — not on every poll.
func (c *AS1289Client) FetchNames(ctx context.Context, host, authUser, authPass string) ([]string, error) {
	body, err := c.doGet(ctx, normalizeHost(host)+"/setswitch.htm", authUser, authPass)
	if err != nil {
		return nil, err
	}
	return parseAS1289Names(body), nil
}

// parseAS1289Names returns exactly as1289PortCount entries, "" for any port
// not found or left blank on the device (fallback text is applied later, at
// display time, by displayNames).
func parseAS1289Names(html string) []string {
	names := make([]string, as1289PortCount)
	for _, m := range as1289NameRe.FindAllStringSubmatch(html, -1) {
		idx, err := strconv.Atoi(m[1])
		if err != nil || idx < 1 || idx > as1289PortCount {
			continue
		}
		names[idx-1] = strings.TrimSpace(m[2])
	}
	return names
}

func (c *AS1289Client) doGet(ctx context.Context, u, authUser, authPass string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return "", err
	}
	if authUser != "" {
		req.SetBasicAuth(authUser, authPass)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusUnauthorized {
		return "", fmt.Errorf("unauthorized (check username/password in settings)")
	}
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("unexpected status: %s", resp.Status)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	return string(body), nil
}

// parseAS1289Active reads a pipe-separated status string, e.g.
// "ap1|aa2|aa3|aa4|aa5|h0|ga|l64039", and returns the 0-based index of the
// single "ap<N>" (active) token, or -1 if none is present.
func parseAS1289Active(body string) int {
	for _, tok := range strings.Split(strings.TrimSpace(body), "|") {
		tok = strings.TrimSpace(tok)
		if !strings.HasPrefix(tok, "ap") {
			continue
		}
		n, err := strconv.Atoi(tok[2:])
		if err == nil && n >= 1 {
			return n - 1
		}
	}
	return -1
}

// displayNames pads/truncates to exactly n entries and fills any blank
// slot with a generic "Ant N" label, purely for display — the underlying
// config keeps blank slots blank until the user actually names them.
func displayNames(names []string, n int) []string {
	out := make([]string, n)
	for i := 0; i < n; i++ {
		if i < len(names) && strings.TrimSpace(names[i]) != "" {
			out[i] = names[i]
		} else {
			out[i] = fmt.Sprintf("Ant %d", i+1)
		}
	}
	return out
}
