// Package config loads non-secret JSON configuration.
package config

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/sunerpy/router-tools/internal/device"
)

// Config contains only non-secret user preferences.
type Config struct {
	SchemaVersion  int    `json:"schema_version"`
	Host           string `json:"host"`
	WebPort        int    `json:"web_port"`
	SSHPort        int    `json:"ssh_port"`
	TelnetPort     int    `json:"telnet_port"`
	Profile        string `json:"profile"`
	LogLevel       string `json:"log_level"`
	KnownHostsFile string `json:"known_hosts_file"`
	RunDir         string `json:"run_dir"`
}

// Default returns built-in preferences.
func Default() Config {
	target := device.DefaultTarget()
	return Config{
		SchemaVersion: 1,
		Host:          target.Host,
		WebPort:       target.WebPort,
		SSHPort:       target.SSHPort,
		TelnetPort:    target.TelnetPort,
		Profile:       "auto",
		LogLevel:      "info",
	}
}

// DefaultPath returns the OS user configuration location.
func DefaultPath() string {
	directory, err := os.UserConfigDir()
	if err != nil {
		return filepath.Join(".", "routerctl", "config.json")
	}
	return filepath.Join(directory, "routerctl", "config.json")
}

// Load strictly decodes one JSON object and rejects unknown/secret fields.
func Load(path string) (Config, error) {
	cfg := Default()
	data, err := os.ReadFile(path)
	if err != nil {
		return cfg, device.WrapError(device.CodeInvalidArgument, "cannot read config file", err)
	}
	if len(data) > 1<<20 {
		return cfg, &device.Error{Code: device.CodeInvalidArgument, Message: "config file exceeds 1 MiB"}
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&cfg); err != nil {
		return cfg, &device.Error{Code: device.CodeInvalidArgument, Message: "invalid config JSON or unknown/secret field"}
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		return cfg, &device.Error{Code: device.CodeInvalidArgument, Message: "config must contain one JSON object"}
	}
	if cfg.SchemaVersion != 1 {
		return cfg, &device.Error{Code: device.CodeInvalidArgument, Message: "unsupported config schema_version"}
	}
	target := device.Target{Host: cfg.Host, WebPort: cfg.WebPort, SSHPort: cfg.SSHPort, TelnetPort: cfg.TelnetPort}
	if err := target.Validate(); err != nil {
		return cfg, err
	}
	switch cfg.LogLevel {
	case "debug", "info", "warn", "error":
	default:
		return cfg, &device.Error{Code: device.CodeInvalidArgument, Message: fmt.Sprintf("invalid log_level %q", cfg.LogLevel)}
	}
	return cfg, nil
}
