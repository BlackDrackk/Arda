// Package config handles loading and saving the Arda TOML configuration file.
package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"github.com/BurntSushi/toml"
)

// NetworkMode represents the container network isolation mode.
type NetworkMode string

const (
	NetworkBridge NetworkMode = "bridge"
	NetworkHost   NetworkMode = "host"
	NetworkNone   NetworkMode = "none"
	NetworkSlirp  NetworkMode = "slirp4netns"
)

// Config is the structure of ~/.arda/config.toml.
type Config struct {
	// Socket is the Podman API socket URI (e.g. unix:///run/user/1000/podman/podman.sock).
	Socket string `toml:"socket"`

	// DefaultImage is the OCI image used when none is specified at runtime.
	DefaultImage string `toml:"default_image"`

	// Timeout is the maximum number of seconds to wait for an API response.
	Timeout int `toml:"timeout"`

	// Network is the container network isolation mode.
	Network NetworkMode `toml:"network"`
}

// Default returns a Config populated with sensible defaults.
func Default() Config {
	return Config{
		Socket:       defaultSocket(),
		DefaultImage: "docker.io/library/debian:latest",
		Timeout:      30,
		Network:      NetworkBridge,
	}
}

// defaultSocket resolves the Podman socket path using a priority order that
// covers rootless Podman, rootful Podman, and Docker compatibility.
func defaultSocket() string {
	// 1. Explicit user overrides take precedence.
	if v := os.Getenv("CONTAINER_HOST"); v != "" {
		return v
	}
	if v := os.Getenv("PODMAN_HOST"); v != "" {
		return v
	}

	// 2. Rootless Podman — most common on Linux / WSL.
	if xdg := os.Getenv("XDG_RUNTIME_DIR"); xdg != "" {
		return "unix://" + filepath.Join(xdg, "podman", "podman.sock")
	}

	// 3. Rootful Podman socket.
	if _, err := os.Stat("/run/podman/podman.sock"); err == nil {
		return "unix:///run/podman/podman.sock"
	}

	// 4. Docker compatibility fallback.
	if _, err := os.Stat("/var/run/docker.sock"); err == nil {
		return "unix:///var/run/docker.sock"
	}

	// 5. Safe UID-based fallback for rootless systems without XDG_RUNTIME_DIR.
	return fmt.Sprintf("unix:///run/user/%d/podman/podman.sock", os.Getuid())
}

// Path returns the canonical path of the user configuration file.
func Path() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		// BUG FIX: the error was previously discarded with _.
		return "", fmt.Errorf("resolving home directory: %w", err)
	}
	return filepath.Join(home, ".arda", "config.toml"), nil
}

// Load reads the configuration file if it exists, otherwise returns the
// defaults. A missing file is not an error.
func Load() (Config, error) {
	cfg := Default()

	path, err := Path()
	if err != nil {
		return cfg, err
	}

	if _, err := os.Stat(path); errors.Is(err, os.ErrNotExist) {
		// No config file — silently use defaults.
		return cfg, nil
	}

	if _, err := toml.DecodeFile(path, &cfg); err != nil {
		return cfg, fmt.Errorf("parsing config file %q: %w", path, err)
	}

	return cfg, nil
}

// Save serialises cfg to the TOML configuration file, creating the directory
// tree if necessary. The file is written with mode 0600 (owner read/write only).
func Save(cfg Config) error {
	path, err := Path()
	if err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("creating config directory: %w", err)
	}

	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		return fmt.Errorf("opening config file: %w", err)
	}
	defer f.Close()

	if err := toml.NewEncoder(f).Encode(cfg); err != nil {
		return fmt.Errorf("encoding config: %w", err)
	}
	return nil
}
