package clicfg

import "time"

type TLSConfig struct {
	CAFile             string `yaml:"ca_file"`
	CertFile           string `yaml:"cert_file"`
	KeyFile            string `yaml:"key_file"`
	InsecureSkipVerify bool   `yaml:"insecure_skip_verify"`
}

type RetryConfig struct {
	Enabled         bool          `yaml:"enabled"`
	InitialInterval time.Duration `yaml:"initial_interval"`
	MaxInterval     time.Duration `yaml:"max_interval"`
	MaxElapsedTime  time.Duration `yaml:"max_elapsed_time"`
}

type Headers map[string]string

type Exporter struct {
	Endpoint    string        `yaml:"endpoint"`
	Insecure    bool          `yaml:"insecure"`
	Compression string        `yaml:"compression"`
	Timeout     time.Duration `yaml:"timeout"`
	Headers     Headers       `yaml:"headers"`
	TLS         TLSConfig     `yaml:"tls"`
	Retry       RetryConfig   `yaml:"retry"`
}

func (h Headers) ExpandHeaders() Headers {
	return Headers(ExpandMap(map[string]string(h)))
}
