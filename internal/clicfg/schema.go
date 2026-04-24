package clicfg

import (
	"time"
)

type Config struct {
	Receiver ReceiverConfig        `yaml:"receiver"`
	Storage  StorageConfig         `yaml:"storage"`
	TUI      TUIConfig             `yaml:"tui"`
	Fanout   []FanoutEntry         `yaml:"fanout"`
	Profiles map[string]RawProfile `yaml:"profiles"`

	activeProfile string
	sourcePath    string
}

func (c *Config) ActiveProfile() string { return c.activeProfile }
func (c *Config) SourcePath() string    { return c.sourcePath }

type ReceiverConfig struct {
	GRPCAddr string `yaml:"grpc_addr"`
}

type StorageConfig struct {
	Enabled bool   `yaml:"enabled"`
	Path    string `yaml:"path"`
}

type TUIConfig struct {
	Enabled bool          `yaml:"enabled"`
	Refresh time.Duration `yaml:"refresh"`
}

type FanoutEntry struct {
	Type     string    `yaml:"type"`
	Endpoint string    `yaml:"endpoint"`
	Listen   string    `yaml:"listen"`
	Insecure bool      `yaml:"insecure"`
	Headers  Headers   `yaml:"headers"`
	TLS      TLSConfig `yaml:"tls"`
}

type RawProfile map[string]any
