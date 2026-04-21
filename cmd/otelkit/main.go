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

const jaegerOTLPEndpoint = "localhost:14317"
const prometheusExporterAddr = ":9091"
const lokiEndpoint = "http://localhost:3100"

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
	runCmd.Parse(flagArgs)

	if len(childArgs) == 0 {
		fmt.Fprintln(os.Stderr, "usage: otelkit run [--grpc-addr :4317] [--service <name>] -- <command> [args...]")
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

	fanout, err := buildFanout()
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

// splitOnDashDash splits args on the first "--" separator.
// Returns everything before "--" as flagArgs and everything after as childArgs.
// If "--" is not present, all args are flagArgs and childArgs is nil.
func splitOnDashDash(args []string) (flagArgs, childArgs []string) {
	for i, a := range args {
		if a == "--" {
			return args[:i], args[i+1:]
		}
	}
	return args, nil
}

// grpcAddrToEndpoint converts a listen address to a localhost connect address.
// ":4317" → "localhost:4317"
// "0.0.0.0:4317" → "localhost:4317"
func grpcAddrToEndpoint(addr string) string {
	if idx := strings.LastIndex(addr, ":"); idx >= 0 {
		return "localhost" + addr[idx:]
	}
	return addr
}

func buildFanout() (*export.Fanout, error) {
	jaeger, err := export.NewJaegerExporter(jaegerOTLPEndpoint)
	if err != nil {
		return nil, err
	}
	prometheus, err := export.NewPrometheusExporter(prometheusExporterAddr)
	if err != nil {
		return nil, err
	}
	loki := export.NewLokiExporter(lokiEndpoint)

	return export.NewFanout(
		[]export.TraceExporter{jaeger},
		[]export.MetricsExporter{prometheus},
		[]export.LogsExporter{loki},
	), nil
}
