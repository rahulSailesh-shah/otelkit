package cli

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/rahulSailesh-shah/otelkit/internal/launcher"
	"github.com/rahulSailesh-shah/otelkit/internal/receiver"
	"github.com/rahulSailesh-shah/otelkit/internal/store/db"
	"github.com/rahulSailesh-shah/otelkit/internal/store/repo"
	"github.com/rahulSailesh-shah/otelkit/internal/tui"
)

type runOptions struct {
	GRPCAddr       string
	Service        string
	JaegerAddr     string
	PrometheusAddr string
	LokiAddr       string
	SigNozAddr     string
	TUI            bool
	ChildLog       string
	Refresh        time.Duration
}

func newRunCmd() *cobra.Command {
	var opts runOptions
	cmd := &cobra.Command{
		Use:   "run [flags] -- <command> [args...]",
		Short: "Launch a command with the OTLP receiver (and optional TUI) attached",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runExecute(cmd.Context(), opts, args)
		},
	}

	f := cmd.Flags()
	f.StringVar(&opts.GRPCAddr, "grpc-addr", ":4317", "OTLP gRPC listen address")
	f.StringVar(&opts.Service, "service", "", "OTEL_SERVICE_NAME injected into child")

	f.StringVar(&opts.JaegerAddr, "jaeger-addr", "", "Jaeger OTLP gRPC address (e.g. localhost:14317)")
	f.StringVar(&opts.PrometheusAddr, "prometheus-addr", "", "Prometheus metrics listen addr (e.g. :9091)")
	f.StringVar(&opts.LokiAddr, "loki-addr", "", "Loki push API URL (e.g. http://localhost:3100)")
	f.StringVar(&opts.SigNozAddr, "signoz-addr", "", "SigNoz OTLP gRPC address (e.g. localhost:24317)")

	f.BoolVar(&opts.TUI, "tui", false, "run TUI dashboard alongside child")
	f.StringVar(&opts.ChildLog, "child-log", "", "path for child stdout/stderr when --tui is set (default: temp file)")
	f.DurationVar(&opts.Refresh, "refresh", 2*time.Second, "TUI refresh interval")

	return cmd
}

func runExecute(ctx context.Context, opts runOptions, childArgs []string) error {
	database := db.NewSQLiteDB(ctx, globalOpts.DBPath)
	if err := database.Connect(); err != nil {
		return fmt.Errorf("open db: %w", err)
	}
	defer func() {
		if err := database.Close(); err != nil {
			log.Printf("close db: %v", err)
		}
	}()

	writeDB := database.GetWriteDB()

	fanout, err := buildFanout(fanoutConfig{
		JaegerAddr:     opts.JaegerAddr,
		PrometheusAddr: opts.PrometheusAddr,
		LokiAddr:       opts.LokiAddr,
		SigNozAddr:     opts.SigNozAddr,
	})
	if err != nil {
		return fmt.Errorf("build fanout: %w", err)
	}
	defer func() {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		fanout.Shutdown(shutdownCtx)
	}()

	traceHandler := receiver.NewTraceHandler(writeDB, fanout)
	metricsHandler := receiver.NewMetricsHandler(writeDB, fanout)
	logsHandler := receiver.NewLogsHandler(writeDB, fanout)

	srv, err := receiver.StartGRPC(opts.GRPCAddr, traceHandler, metricsHandler, logsHandler)
	if err != nil {
		return fmt.Errorf("start grpc: %w", err)
	}
	defer srv.Stop()

	endpoint := grpcAddrToEndpoint(opts.GRPCAddr)
	env := launcher.BuildEnv(launcher.Config{Endpoint: endpoint, ServiceName: opts.Service})

	if opts.TUI {
		return runWithTUI(ctx, opts, childArgs, env, database)
	}

	log.Printf("otelkit: receiver on %s -> spawning: %s", opts.GRPCAddr, strings.Join(childArgs, " "))

	code, err := launcher.Run(ctx, childArgs, env, launcher.IOConfig{})
	if err != nil {
		return err
	}
	if code != 0 {
		return fmt.Errorf("child exited with code %d", code)
	}
	return nil
}

func runWithTUI(ctx context.Context, opts runOptions, childArgs []string, env []string, database db.DB) error {
	logPath := opts.ChildLog
	if logPath == "" {
		f, err := os.CreateTemp("", "otelkit-child-*.log")
		if err != nil {
			return fmt.Errorf("create temp log: %w", err)
		}
		logPath = f.Name()
		_ = f.Close()
	}
	logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	if err != nil {
		return fmt.Errorf("open child log: %w", err)
	}
	defer logFile.Close()

	tuiCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	childErrCh := make(chan error, 1)
	go func() {
		code, err := launcher.Run(tuiCtx, childArgs, env, launcher.IOConfig{
			Stdout: logFile,
			Stderr: logFile,
		})
		if err != nil {
			childErrCh <- err
			return
		}
		if code != 0 {
			childErrCh <- fmt.Errorf("child exited with code %d", code)
			return
		}
		childErrCh <- nil
	}()

	queries := repo.New(database.GetReadDB())
	tuiErr := tui.Run(tuiCtx, tui.Options{
		Queries:         queries,
		RefreshInterval: opts.Refresh,
		ChildLogPath:    logPath,
	})

	cancel() // stop child if TUI exited first
	childErr := <-childErrCh

	if tuiErr != nil {
		return tuiErr
	}
	return childErr
}

func grpcAddrToEndpoint(addr string) string {
	if idx := strings.LastIndex(addr, ":"); idx >= 0 {
		return "localhost" + addr[idx:]
	}
	return addr
}
