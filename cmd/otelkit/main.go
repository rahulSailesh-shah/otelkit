package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/rahulSailesh-shah/otelkit/internal/export"
	"github.com/rahulSailesh-shah/otelkit/internal/receiver"
	"github.com/rahulSailesh-shah/otelkit/internal/store/db"
	"github.com/rahulSailesh-shah/otelkit/internal/store/repo"
)

// OTLP gRPC on Jaeger all-in-one, mapped to host 14317 in deploy/docker-compose.yml
// so it does not collide with otelkit's receiver on :4317.
const jaegerOTLPEndpoint = "localhost:14317"

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintf(os.Stderr, "error: no command provided\n")
		fmt.Fprintf(os.Stderr, "usage: %s <command>\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "available commands: run, version\n")
		os.Exit(2)
	}
	switch os.Args[1] {
	case "run":
		if err := runReceiverOnly(os.Args[2:]); err != nil {
			fmt.Fprintf(os.Stderr, "run failed: %v\n", err)
			os.Exit(1)
		}
	case "version":
		fmt.Println("otelkit dev")
	default:
		fmt.Fprintf(os.Stderr, "unknown command: %s\n", os.Args[1])
		os.Exit(2)
	}
}

func runReceiverOnly(args []string) error {
	runCmd := flag.NewFlagSet("run", flag.ExitOnError)
	grpcAddr := runCmd.String("grpc-addr", ":4317", "OTLP gRPC listen address")
	if err := runCmd.Parse(args); err != nil {
		return err
	}

	database := db.NewSQLiteDB(context.Background())
	if err := database.Connect(); err != nil {
		return err
	}
	defer func() {
		if err := database.Close(); err != nil {
			log.Printf("close db: %v", err)
		}
	}()

	queries := repo.New(database.GetDB())

	fanout, err := buildFanout()
	if err != nil {
		return err
	}
	defer func() {
		if fanout == nil {
			return
		}
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		fanout.Shutdown(shutdownCtx)
	}()

	traceHandler := receiver.NewTraceHandler(queries, fanout)
	metricsHandler := receiver.NewMetricsHandler()
	srv, err := receiver.StartGRPC(*grpcAddr, traceHandler, metricsHandler)
	if err != nil {
		return err
	}
	log.Printf("OTLP gRPC receiver listening — traces + metrics on %s", *grpcAddr)
	log.Printf("Fan-out: jaeger OTLP -> %s", jaegerOTLPEndpoint)
	log.Printf("Waiting for traces... (Ctrl+C to stop)")

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh

	srv.Stop()
	return nil
}

func buildFanout() (*export.Fanout, error) {
	jaeger, err := export.NewJaegerExporter(jaegerOTLPEndpoint)
	if err != nil {
		return nil, err
	}
	return export.NewFanout(jaeger), nil
}
