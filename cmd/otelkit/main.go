package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/rahulSailesh-shah/otelkit/internal/export"
	"github.com/rahulSailesh-shah/otelkit/internal/launcher"
	"github.com/rahulSailesh-shah/otelkit/internal/receiver"
	"github.com/rahulSailesh-shah/otelkit/internal/store/db"
	"github.com/rahulSailesh-shah/otelkit/internal/store/repo"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintf(os.Stderr, "usage: %s <command>\navailable commands: run, version\n", os.Args[0])
		os.Exit(2)
	}
	switch os.Args[1] {
	case "run":
		code, err := runLauncher(os.Args[2:])
		if err != nil {
			fmt.Fprintf(os.Stderr, "run failed: %v\n", err)
			os.Exit(1)
		}
		os.Exit(code)
	case "version":
		fmt.Println("otelkit dev")
	default:
		fmt.Fprintf(os.Stderr, "unknown command: %s\n", os.Args[1])
		os.Exit(2)
	}
}

func runLauncher(args []string) (int, error) {
	flagArgs, childArgs := splitOnDashDash(args)

	runCmd := flag.NewFlagSet("run", flag.ExitOnError)
	grpcAddr := runCmd.String("grpc-addr", ":4317", "OTLP gRPC listen address")
	service := runCmd.String("service", "", "service name injected as OTEL_SERVICE_NAME")

	// ── Grafana stack
	jaegerAddr := runCmd.String("jaeger-addr", "", "Jaeger OTLP gRPC address (e.g. localhost:14317); empty = disabled")
	prometheusAddr := runCmd.String("prometheus-addr", "", "Prometheus metrics listen addr (e.g. :9091); empty = disabled")
	lokiAddr := runCmd.String("loki-addr", "", "Loki push API URL (e.g. http://localhost:3100); empty = disabled")

	// ── SigNoz metrics, logs, traces all in one
	signozAddr := runCmd.String("signoz-addr", "", "SigNoz OTLP gRPC address (e.g. localhost:24317); empty = disabled")

	runCmd.Parse(flagArgs)

	if len(childArgs) == 0 {
		fmt.Fprintln(os.Stderr, "usage: otelkit run [flags] -- <command> [args...]")
		fmt.Fprintln(os.Stderr, "")
		fmt.Fprintln(os.Stderr, "flags:")
		runCmd.PrintDefaults()
		return 2, nil
	}

	database := db.NewSQLiteDB(context.Background())
	if err := database.Connect(); err != nil {
		return 1, err
	}
	defer func() {
		if err := database.Close(); err != nil {
			log.Printf("close db: %v", err)
		}
	}()

	queries := repo.New(database.GetDB())

	fanout, err := buildFanout(fanoutConfig{
		jaegerAddr:     *jaegerAddr,
		prometheusAddr: *prometheusAddr,
		lokiAddr:       *lokiAddr,
		signozAddr:     *signozAddr,
	})
	if err != nil {
		return 1, err
	}
	defer func() {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		fanout.Shutdown(shutdownCtx)
	}()

	traceHandler := receiver.NewTraceHandler(queries, fanout)
	metricsHandler := receiver.NewMetricsHandler(queries, fanout)
	logsHandler := receiver.NewLogsHandler(queries, fanout)

	srv, err := receiver.StartGRPC(*grpcAddr, traceHandler, metricsHandler, logsHandler)
	if err != nil {
		return 1, err
	}
	defer srv.Stop()

	endpoint := grpcAddrToEndpoint(*grpcAddr)
	env := launcher.BuildEnv(launcher.Config{
		Endpoint:    endpoint,
		ServiceName: *service,
	})

	log.Printf("otelkit: receiver on %s → spawning: %s", *grpcAddr, strings.Join(childArgs, " "))

	code, err := launcher.Run(context.Background(), childArgs, env)
	if err != nil {
		return 1, err
	}
	return code, nil
}

type fanoutConfig struct {
	jaegerAddr     string
	prometheusAddr string
	lokiAddr       string
	signozAddr     string
}

func buildFanout(cfg fanoutConfig) (*export.Fanout, error) {
	var traceExporters []export.TraceExporter
	var metricsExporters []export.MetricsExporter
	var logsExporters []export.LogsExporter

	if cfg.jaegerAddr != "" {
		jaeger, err := export.NewJaegerExporter(cfg.jaegerAddr)
		if err != nil {
			return nil, err
		}
		traceExporters = append(traceExporters, jaeger)
	}

	if cfg.prometheusAddr != "" {
		prometheus, err := export.NewPrometheusExporter(cfg.prometheusAddr)
		if err != nil {
			return nil, err
		}
		metricsExporters = append(metricsExporters, prometheus)
	}

	if cfg.lokiAddr != "" {
		logsExporters = append(logsExporters, export.NewLokiExporter(cfg.lokiAddr))
	}

	if cfg.signozAddr != "" {
		signoz, err := export.NewSigNozExporter(cfg.signozAddr)
		if err != nil {
			return nil, err
		}
		traceExporters = append(traceExporters, signoz)
		metricsExporters = append(metricsExporters, signoz)
		logsExporters = append(logsExporters, signoz)
	}

	return export.NewFanout(traceExporters, metricsExporters, logsExporters), nil
}

func splitOnDashDash(args []string) (flagArgs, childArgs []string) {
	for i, a := range args {
		if a == "--" {
			return args[:i], args[i+1:]
		}
	}
	return args, nil
}

func grpcAddrToEndpoint(addr string) string {
	if idx := strings.LastIndex(addr, ":"); idx >= 0 {
		return "localhost" + addr[idx:]
	}
	return addr
}
