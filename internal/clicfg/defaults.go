package clicfg

import "time"

func Defaults() *Config {
	return &Config{
		Receiver: ReceiverConfig{GRPCAddr: ":4317"},
		Storage:  StorageConfig{Enabled: true, Path: "./otelkit.db"},
		TUI:      TUIConfig{Enabled: false, Refresh: 2 * time.Second},
		Fanout:   nil,
	}
}
