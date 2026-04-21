package launcher

// Config holds the values injected into the child process environment.
type Config struct {
	Endpoint    string // e.g. "localhost:4317"
	ServiceName string // injected as OTEL_SERVICE_NAME only if non-empty
}

// BuildEnv returns env var strings (KEY=VALUE) to append to the child's environment.
// Only injects what pkg/sdk's Init() actually reads.
func BuildEnv(cfg Config) []string {
	env := []string{
		"OTEL_EXPORTER_OTLP_ENDPOINT=" + cfg.Endpoint,
	}
	if cfg.ServiceName != "" {
		env = append(env, "OTEL_SERVICE_NAME="+cfg.ServiceName)
	}
	return env
}
