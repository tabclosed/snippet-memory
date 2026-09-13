package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// defaultPort is used when config.json is missing the "port" field, or
// doesn't exist yet at all.
const defaultPort = 8080

// Config holds user-editable server settings, stored as a small JSON
// file next to the executable (or, in dev, at the project root) — so
// this can be changed by editing a file, without setting environment
// variables or rebuilding anything.
type Config struct {
	// Port is which TCP port the server listens on. Defaults to 8080,
	// matching this project's documented default throughout the README.
	Port int `json:"port"`
}

var defaultConfig = Config{Port: defaultPort}

// LoadConfig reads the config file at path, creating it (populated with
// defaultConfig) if it doesn't exist yet — so there's always a real,
// discoverable file to open and edit, rather than a setting that only
// exists once someone knows to create it by hand.
func LoadConfig(path string) (Config, error) {
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		if err := saveConfig(path, defaultConfig); err != nil {
			return Config{}, err
		}
		return defaultConfig, nil
	}
	if err != nil {
		return Config{}, fmt.Errorf("reading config file: %w", err)
	}

	// Start from the defaults so a config file from an older version
	// (missing a field this version added) still gets a sane value for
	// whatever it doesn't specify, instead of Go's zero value — a
	// config.json that's just `{}` shouldn't silently try to listen on
	// port 0.
	cfg := defaultConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return Config{}, fmt.Errorf("parsing config file: %w", err)
	}
	if cfg.Port <= 0 || cfg.Port > 65535 {
		cfg.Port = defaultPort
	}
	return cfg, nil
}

func saveConfig(path string, cfg Config) error {
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("encoding config file: %w", err)
	}
	if dir := filepath.Dir(path); dir != "." {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("creating config directory: %w", err)
		}
	}
	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("writing config file: %w", err)
	}
	return nil
}
