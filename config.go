package main

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// DeviceKind selects which antenna switch profile the app talks to. Only
// one is ever active/polled at a time.
type DeviceKind string

const (
	DeviceAT14   DeviceKind = "at14"
	DeviceAS1289 DeviceKind = "as1289"
)

const as1289DefaultHost = "192.168.86.40"

// AT14Config holds the AT-14's JSON-API connection settings.
type AT14Config struct {
	Host  string `json:"host"`
	Token string `json:"token"`
}

// AS1289Config holds the Microbit AS-1289's connection settings. The device
// doesn't expose antenna names over its status protocol, so they're entered
// by the user here instead (empty slots fall back to "Ant N" for display).
type AS1289Config struct {
	Host     string   `json:"host"`
	AuthUser string   `json:"authUser"`
	AuthPass string   `json:"authPass"`
	Names    []string `json:"names"`
}

// Config holds the user's settings, persisted between runs.
type Config struct {
	Device DeviceKind   `json:"device"`
	AT14   AT14Config   `json:"at14"`
	AS1289 AS1289Config `json:"as1289"`
	Mini   bool         `json:"mini"`
	Light  bool         `json:"light"`
}

// legacyConfig captures the flat, pre-multi-device config shape (host/token
// at the top level) so existing users' config files keep working as-is.
type legacyConfig struct {
	Host  string `json:"host"`
	Token string `json:"token"`
}

func configPath() (string, error) {
	dir, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, ".antswitch-gui.json"), nil
}

func loadConfig() Config {
	var cfg Config

	path, err := configPath()
	if err != nil {
		return withDefaults(cfg)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return withDefaults(cfg)
	}
	if err := json.Unmarshal(data, &cfg); err != nil {
		return withDefaults(Config{})
	}

	if cfg.AT14.Host == "" && cfg.AT14.Token == "" {
		var legacy legacyConfig
		if json.Unmarshal(data, &legacy) == nil {
			cfg.AT14.Host = legacy.Host
			cfg.AT14.Token = legacy.Token
		}
	}

	return withDefaults(cfg)
}

func withDefaults(cfg Config) Config {
	if cfg.Device == "" {
		cfg.Device = DeviceAT14
	}
	if cfg.AS1289.Host == "" {
		cfg.AS1289.Host = as1289DefaultHost
	}
	cfg.AS1289.Names = padLength(cfg.AS1289.Names, as1289PortCount)
	return cfg
}

// padLength pads (with "") or truncates names to exactly n entries, without
// inventing placeholder text — that's left to callers that render for
// display, so an unset name never gets silently persisted as "Ant N".
func padLength(names []string, n int) []string {
	out := make([]string, n)
	copy(out, names)
	return out
}

func saveConfig(cfg Config) error {
	path, err := configPath()
	if err != nil {
		return err
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o600)
}
