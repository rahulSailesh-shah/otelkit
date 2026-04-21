package main

import (
	"context"
	"fmt"
	"log"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"todos/internal/config"
	"todos/internal/external"
	"todos/internal/handlers"
	"todos/internal/repository"
	"todos/internal/telemetry"

	"github.com/XSAM/otelsql"
	_ "github.com/mattn/go-sqlite3"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
)

func main() {
	cfg := config.Load()
	fmt.Printf("Loaded config: %+v\n", cfg)

	// Initialize OpenTelemetry Logging — must be first so log.X calls are captured
	logger, loggerProvider, err := telemetry.InitLogger(
		cfg.ServiceName,
		cfg.ServiceVersion,
		cfg.Environment,
		cfg.OTLPEndpoint,
	)
	if err != nil {
		log.Printf("Warning: Failed to initialize logger (continuing with default): %v", err)
		logger = slog.Default()
	} else {
		defer func() {
			if err := loggerProvider.Shutdown(context.Background()); err != nil {
				log.Printf("Failed to shutdown logger provider: %v", err)
			}
		}()
		// redirect std log output through slog so log.X calls also ship to OTLP
		log.SetOutput(slog.NewLogLogger(logger.Handler(), slog.LevelInfo).Writer())
		log.SetFlags(0)
	}

	logger.Info("Starting application",
		"service", cfg.ServiceName,
		"version", cfg.ServiceVersion,
		"environment", cfg.Environment,
	)

	// Initialize OpenTelemetry Tracing
	tracerProvider, err := telemetry.InitTracer(
		cfg.ServiceName,
		cfg.ServiceVersion,
		cfg.Environment,
		cfg.OTLPEndpoint,
	)
	if err != nil {
		logger.Warn("Failed to initialize tracer (continuing without tracing)", "error", err)
	} else {
		defer func() {
			if err := tracerProvider.Shutdown(context.Background()); err != nil {
				logger.Error("Failed to shutdown tracer provider", "error", err)
			}
		}()
	}

	// Initialize OpenTelemetry Metrics
	meterProvider, err := telemetry.InitMetrics(
		cfg.ServiceName,
		cfg.ServiceVersion,
		cfg.Environment,
		cfg.OTLPEndpoint,
	)
	if err != nil {
		logger.Warn("Failed to initialize metrics (continuing without metrics)", "error", err)
		meterProvider = sdkmetric.NewMeterProvider(sdkmetric.WithReader(sdkmetric.NewManualReader()))
	} else {
		defer func() {
			if err := meterProvider.Shutdown(context.Background()); err != nil {
				logger.Error("Failed to shutdown meter provider", "error", err)
			}
		}()
	}

	otel.SetMeterProvider(meterProvider)

	db, err := otelsql.Open("sqlite3", cfg.DatabasePath,
		otelsql.WithAttributes(semconv.DBSystemSqlite),
	)
	if err != nil {
		logger.Error("Failed to open database", "error", err)
		os.Exit(1)
	}

	repo, err := repository.NewSQLiteRepository(db)
	if err != nil {
		logger.Error("Failed to initialize repository", "error", err)
		os.Exit(1)
	}
	defer repo.Close()

	logger.Info("Database initialized", "path", cfg.DatabasePath)

	externalClient := external.NewClient()

	meter := meterProvider.Meter("todo-handler")
	todoHandler := handlers.NewTodoHandler(repo, externalClient, logger, meter)

	mux := http.NewServeMux()

	mux.HandleFunc("/todos", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodPost:
			todoHandler.CreateTodo(w, r)
		case http.MethodGet:
			todoHandler.ListTodos(w, r)
		default:
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	})

	mux.HandleFunc("/todos/", func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path
		if path == "/todos/external/" || len(path) > len("/todos/external/") {
			todoHandler.GetExternalTodo(w, r)
		} else {
			switch r.Method {
			case http.MethodGet:
				todoHandler.GetTodo(w, r)
			case http.MethodPut:
				todoHandler.UpdateTodo(w, r)
			case http.MethodDelete:
				todoHandler.DeleteTodo(w, r)
			default:
				http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			}
		}
	})

	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	serverAddr := fmt.Sprintf("%s:%s", cfg.ServerHost, cfg.ServerPort)
	server := &http.Server{
		Addr:    serverAddr,
		Handler: otelhttp.NewHandler(mux, cfg.ServiceName),
	}

	go func() {
		logger.Info("Server starting", "address", serverAddr)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("Server failed to start", "error", err)
			os.Exit(1)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info("Shutting down server...")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		logger.Error("Server forced to shutdown", "error", err)
	}

	logger.Info("Server stopped")
}
