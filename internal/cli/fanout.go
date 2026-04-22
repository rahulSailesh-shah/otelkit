package cli

import (
	"github.com/rahulSailesh-shah/otelkit/internal/export"
)

type fanoutConfig struct {
	JaegerAddr     string
	PrometheusAddr string
	LokiAddr       string
	SigNozAddr     string
}

func buildFanout(cfg fanoutConfig) (*export.Fanout, error) {
	var traceExporters []export.TraceExporter
	var metricsExporters []export.MetricsExporter
	var logsExporters []export.LogsExporter

	if cfg.JaegerAddr != "" {
		j, err := export.NewJaegerExporter(cfg.JaegerAddr)
		if err != nil {
			return nil, err
		}
		traceExporters = append(traceExporters, j)
	}
	if cfg.PrometheusAddr != "" {
		p, err := export.NewPrometheusExporter(cfg.PrometheusAddr)
		if err != nil {
			return nil, err
		}
		metricsExporters = append(metricsExporters, p)
	}
	if cfg.LokiAddr != "" {
		logsExporters = append(logsExporters, export.NewLokiExporter(cfg.LokiAddr))
	}
	if cfg.SigNozAddr != "" {
		s, err := export.NewSigNozExporter(cfg.SigNozAddr)
		if err != nil {
			return nil, err
		}
		traceExporters = append(traceExporters, s)
		metricsExporters = append(metricsExporters, s)
		logsExporters = append(logsExporters, s)
	}
	return export.NewFanout(traceExporters, metricsExporters, logsExporters), nil
}
