package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/rahulSailesh-shah/otelkit/internal/receiver"
	"github.com/rahulSailesh-shah/otelkit/internal/store/db"
	"github.com/rahulSailesh-shah/otelkit/internal/store/repo"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintf(os.Stderr, "error: no command provided\n")
		fmt.Fprintf(os.Stderr, "usage: %s <command>\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "available commands: run, version\n")
		os.Exit(2)
	}
	switch os.Args[1] {
	case "run":
		if err := runReceiverOnly(); err != nil {
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

func runReceiverOnly() error {
	db := db.NewSQLiteDB(context.Background())
	if err := db.Connect(); err != nil {
		return err
	}
	defer func() {
		if err := db.Close(); err != nil {
			log.Printf("close db: %v", err)
		}
	}()

	repo := repo.New(db.GetDB())

	traceHandler := receiver.NewTraceHandler(repo)
	srv, err := receiver.StartGRPC(":4317", traceHandler)
	if err != nil {
		return err
	}
	log.Printf("OTLP gRPC receiver listening on :4317")
	log.Printf("Waiting for traces... (Ctrl+C to stop)")

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh

	srv.Stop()
	return nil
}
