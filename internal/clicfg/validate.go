package clicfg

import (
	"fmt"
	"strings"
)

var knownFanoutTypes = map[string]bool{
	"jaeger":     true,
	"prometheus": true,
	"loki":       true,
	"signoz":     true,
	"otlp":       true,
}

func (c *Config) Validate() ([]string, error) {
	var warnings []string
	var errs []string

	for i, f := range c.Fanout {
		if !knownFanoutTypes[f.Type] {
			errs = append(errs, fmt.Sprintf(
				"fanout[%d]: unknown type %q (allowed: jaeger, prometheus, loki, signoz, otlp)",
				i, f.Type,
			))
		}
	}

	if c.Storage.Enabled && c.Storage.Path == "" {
		errs = append(errs, "storage.enabled=true but storage.path is empty")
	}

	if len(errs) > 0 {
		return warnings, fmt.Errorf("config invalid: %s", strings.Join(errs, "; "))
	}
	return warnings, nil
}
