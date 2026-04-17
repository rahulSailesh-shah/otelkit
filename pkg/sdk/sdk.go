package sdk

import "context"

type config struct {
	serviceName    string
	serviceVersion string
	endpoint       string
}

type Option func(*config)

func WithServiceName(name string) Option {
	return func(c *config) { c.serviceName = name }
}

func WithServiceVersion(v string) Option {
	return func(c *config) { c.serviceVersion = v }
}

func WithEndpoint(endpoint string) Option {
	return func(c *config) { c.endpoint = endpoint }
}

type ShutdownFunc func(context.Context) error

// Init is the one-liner entrypoint users call from main().
// For now it is a no-op; later it wires the OTel SDK + OTLP exporter
// that points at the otelkit CLI's receiver.
func Init(opts ...Option) (ShutdownFunc, error) {
	c := &config{
		endpoint: "unix:///tmp/otelkit.sock",
	}
	for _, opt := range opts {
		opt(c)
	}
	return func(ctx context.Context) error { return nil }, nil
}
