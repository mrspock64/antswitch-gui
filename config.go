package main

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// Config holds the user's connection settings, persisted between runs.
type Config struct {
	Host  string `json:"host"`
	Token string `json:"token"`
	Mini  bool   `json:"mini"`
	Light bool   `json:"light"`
}

func configPath() (string, error) {
	dir, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, ".antswitch-gui.json"), nil
}

func loadConfig() Config {
	path, err := configPath()
	if err != nil {
		return Config{}
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return Config{}
	}
	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return Config{}
	}
	return cfg
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
