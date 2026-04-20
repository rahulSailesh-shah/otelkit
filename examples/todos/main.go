package main

import (
	"context"
	"fmt"
	"log"
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
	"go.uber.org/zap"
)

func main() {
	// Load configuration
	cfg := config.Load()
	fmt.Printf("Loaded config: %+v\n", cfg)

	// Initialize OpenTelemetry Tracing
	tracerProvider, err := telemetry.InitTracer(
		cfg.ServiceName,
		cfg.ServiceVersion,
		cfg.Environment,
		cfg.OTLPEndpoint,
	)
	if err != nil {
		log.Printf("Warning: Failed to initialize tracer (continuing without tracing): %v", err)
	} else {
		defer func() {
			if err := tracerProvider.Shutdown(context.Background()); err != nil {
				log.Printf("Failed to shutdown tracer provider: %v", err)
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
		log.Printf("Warning: Failed to initialize metrics (continuing without metrics): %v", err)
		meterProvider = sdkmetric.NewMeterProvider(sdkmetric.WithReader(sdkmetric.NewManualReader()))
	} else {
		defer func() {
			if err := meterProvider.Shutdown(context.Background()); err != nil {
				log.Printf("Failed to shutdown meter provider: %v", err)
			}
		}()
	}

	// Set global meter provider
	otel.SetMeterProvider(meterProvider)

	// // Initialize OpenTelemetry Logging
	// logger, loggerProvider, err := telemetry.InitLogger(
	// 	cfg.ServiceName,
	// 	cfg.ServiceVersion,
	// 	cfg.Environment,
	// 	cfg.OTLPEndpoint,
	// )
	// if err != nil {
	// 	log.Printf("Warning: Failed to initialize logger (using fallback logger): %v", err)
	// 	logger, _ = zap.NewProduction()
	// 	loggerProvider = nil
	// } else {
	// 	defer func() {
	// 		if loggerProvider != nil {
	// 			if err := loggerProvider.Shutdown(context.Background()); err != nil {
	// 				log.Printf("Failed to shutdown logger provider: %v", err)
	// 			}
	// 		}
	// 	}()
	// 	defer logger.Sync()
	// }

	// logger.Info("Starting application",
	// 	zap.String("service", cfg.ServiceName),
	// 	zap.String("version", cfg.ServiceVersion),
	// 	zap.String("environment", cfg.Environment),
	// )

	logger, _ := zap.NewProduction()

	db, err := otelsql.Open("sqlite3", cfg.DatabasePath,
		otelsql.WithAttributes(semconv.DBSystemSqlite),
	)
	if err != nil {
		logger.Fatal("Failed to open database", zap.Error(err))
	}

	repo, err := repository.NewSQLiteRepository(db)
	if err != nil {
		logger.Fatal("Failed to initialize repository", zap.Error(err))
	}
	defer repo.Close()

	logger.Info("Database initialized", zap.String("path", cfg.DatabasePath))

	// Initialize external API client
	externalClient := external.NewClient()

	// Initialize handlers with meter
	meter := meterProvider.Meter("todo-handler")
	todoHandler := handlers.NewTodoHandler(repo, externalClient, logger, meter)

	// Setup HTTP routes
	mux := http.NewServeMux()

	// Apply telemetry middleware to all routes
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
			// External API endpoint
			todoHandler.GetExternalTodo(w, r)
		} else {
			// Individual todo endpoint
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

	// Health check endpoint
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	// Create server
	serverAddr := fmt.Sprintf("%s:%s", cfg.ServerHost, cfg.ServerPort)
	server := &http.Server{
		Addr:    serverAddr,
		Handler: otelhttp.NewHandler(mux, cfg.ServiceName),
	}

	// Start server in a goroutine
	go func() {
		logger.Info("Server starting", zap.String("address", serverAddr))
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Fatal("Server failed to start", zap.Error(err))
		}
	}()

	// Wait for interrupt signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info("Shutting down server...")

	// Graceful shutdown with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		logger.Error("Server forced to shutdown", zap.Error(err))
	}

	logger.Info("Server stopped")
}
