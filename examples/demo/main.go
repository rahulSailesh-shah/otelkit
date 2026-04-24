package main

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	_ "github.com/mattn/go-sqlite3"
	"github.com/rahulSailesh-shah/otelkit/pkg/otelkitdb"
	"github.com/rahulSailesh-shah/otelkit/pkg/otelkitlog"
	"github.com/rahulSailesh-shah/otelkit/pkg/otelkitmw"
	"github.com/rahulSailesh-shah/otelkit/pkg/sdk"
)

func main() {
	shutdown, err := sdk.Bootstrap(
		sdk.WithServiceName("demo-api"),
		sdk.WithServiceVersion("0.1.0"),
		sdk.WithEndpoint("localhost:4317"),
	)
	if err != nil {
		slog.Error("init failed", "error", err)
		os.Exit(1)
	}
	defer shutdown(context.Background())

	slog.SetDefault(slog.New(
		otelkitlog.Wrap(slog.NewTextHandler(os.Stdout, nil)),
	))

	db, err := otelkitdb.Open("sqlite3", "./demo.db")
	if err != nil {
		slog.Error("db open failed", "error", err)
		os.Exit(1)
	}
	defer db.Close()

	if _, err := db.Exec(`CREATE TABLE IF NOT EXISTS items (id INTEGER PRIMARY KEY, name TEXT NOT NULL)`); err != nil {
		slog.Error("schema init failed", "error", err)
		os.Exit(1)
	}

	port := os.Getenv("APP_PORT")
	if port == "" {
		port = "8081"
	}
	addr := ":" + port

	mux := http.NewServeMux()

	mux.HandleFunc("/items", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {

		case http.MethodPost:
			var body struct{ Name string }
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Name == "" {
				http.Error(w, "name required", http.StatusBadRequest)
				return
			}
			res, err := db.ExecContext(r.Context(), `INSERT INTO items (name) VALUES (?)`, body.Name)
			if err != nil {
				http.Error(w, "insert failed", http.StatusInternalServerError)
				return
			}
			id, _ := res.LastInsertId()
			slog.InfoContext(r.Context(), "item created", "id", id, "name", body.Name)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusCreated)
			json.NewEncoder(w).Encode(map[string]any{"id": id, "name": body.Name})

		case http.MethodGet:
			rows, err := db.QueryContext(r.Context(), `SELECT id, name FROM items`)
			if err != nil {
				http.Error(w, "query failed", http.StatusInternalServerError)
				return
			}
			defer rows.Close()
			var items []map[string]any
			for rows.Next() {
				var id int64
				var name string
				rows.Scan(&id, &name)
				items = append(items, map[string]any{"id": id, "name": name})
			}
			slog.InfoContext(r.Context(), "items listed", "count", len(items))
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(items)

		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
	})

	server := &http.Server{
		Addr:    addr,
		Handler: otelkitmw.NewHandler(mux, "demo-api"),
	}

	go func() {
		slog.Info("server starting", "addr", addr)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("server error", "error", err)
			os.Exit(1)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	server.Shutdown(ctx)
	slog.Info("server stopped")
}
