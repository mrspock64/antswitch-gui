package main

import (
	"context"
	"io"
	"net"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	scanProbeTimeout = 1500 * time.Millisecond
	scanConcurrency  = 128
)

// looksLikeAS1289Status reports whether body is an AS-1289 status string:
// five port tokens in order (ap<N> = on, aa<N> = off) followed by the
// frequency/flag/counter tokens, e.g. "aa1|ap2|aa3|aa4|aa5|h0|ga|l57627".
func looksLikeAS1289Status(body string) bool {
	toks := strings.Split(strings.TrimSpace(body), "|")
	if len(toks) < as1289PortCount {
		return false
	}
	for i := 0; i < as1289PortCount; i++ {
		tok := toks[i]
		if len(tok) < 3 || (tok[:2] != "ap" && tok[:2] != "aa") {
			return false
		}
		if n, err := strconv.Atoi(tok[2:]); err != nil || n != i+1 {
			return false
		}
	}
	return true
}

// candidateScanPrefixes returns the /24 prefixes ("192.168.86.") worth
// scanning: first the one the configured host is on (a DHCP lease moving
// within the same network is the usual reason the device "disappears", and
// it may only be reachable through a VPN route with no local interface),
// then every directly attached IPv4 /16–/24 network.
func candidateScanPrefixes(configuredHost string) []string {
	seen := map[string]bool{}
	var out []string
	add := func(ip net.IP) {
		v4 := ip.To4()
		if v4 == nil || v4.IsLoopback() || v4.IsLinkLocalUnicast() {
			return
		}
		p := strconv.Itoa(int(v4[0])) + "." + strconv.Itoa(int(v4[1])) + "." + strconv.Itoa(int(v4[2])) + "."
		if !seen[p] {
			seen[p] = true
			out = append(out, p)
		}
	}

	host := strings.TrimPrefix(strings.TrimPrefix(strings.TrimSpace(configuredHost), "http://"), "https://")
	if h, _, err := net.SplitHostPort(host); err == nil {
		host = h
	}
	host = strings.TrimSuffix(host, "/")
	add(net.ParseIP(host))

	addrs, _ := net.InterfaceAddrs()
	for _, a := range addrs {
		ipn, ok := a.(*net.IPNet)
		if !ok {
			continue
		}
		if ones, _ := ipn.Mask.Size(); ones < 16 || ones > 24 {
			continue
		}
		add(ipn.IP)
	}
	return out
}

// scanForAS1289 probes every host on the given /24 prefixes for the
// AS-1289 status endpoint and returns the IPs that answered like one.
func scanForAS1289(ctx context.Context, prefixes []string, authUser, authPass string) []string {
	client := &http.Client{Timeout: scanProbeTimeout}
	sem := make(chan struct{}, scanConcurrency)
	var (
		mu    sync.Mutex
		wg    sync.WaitGroup
		found []string
	)

	for _, prefix := range prefixes {
		for i := 1; i <= 254; i++ {
			ip := prefix + strconv.Itoa(i)
			sem <- struct{}{}
			wg.Add(1)
			go func() {
				defer wg.Done()
				defer func() { <-sem }()
				if ctx.Err() != nil {
					return
				}
				req, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://"+ip+"/setswitch.htm?upd=1", nil)
				if err != nil {
					return
				}
				if authUser != "" {
					req.SetBasicAuth(authUser, authPass)
				}
				resp, err := client.Do(req)
				if err != nil {
					return
				}
				defer resp.Body.Close()
				if resp.StatusCode != http.StatusOK {
					return
				}
				body, err := io.ReadAll(io.LimitReader(resp.Body, 512))
				if err == nil && looksLikeAS1289Status(string(body)) {
					mu.Lock()
					found = append(found, ip)
					mu.Unlock()
				}
			}()
		}
	}
	wg.Wait()
	sort.Strings(found)
	return found
}
