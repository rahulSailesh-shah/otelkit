package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"todos/internal/models"
	"todos/internal/repository"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
)

// TodoHandler handles todo-related HTTP requests
type TodoHandler struct {
	repo     repository.Repository
	external ExternalClient
	tracer   trace.Tracer
	logger   *zap.Logger
	meter    metric.Meter

	requestCounter     metric.Int64Counter
	requestDuration    metric.Float64Histogram
	dbOperationCounter metric.Int64Counter
}

// ExternalClient defines the interface for external API calls
type ExternalClient interface {
	GetTodo(ctx context.Context, id int64) (*models.Todo, error)
}

// NewTodoHandler creates a new todo handler
func NewTodoHandler(repo repository.Repository, external ExternalClient, logger *zap.Logger, meter metric.Meter) *TodoHandler {
	tracer := otel.Tracer("todo-handler")

	requestCounter, err := meter.Int64Counter(
		"http_requests_total",
		metric.WithDescription("Total number of HTTP requests"),
	)
	if err != nil {
		logger.Error("Failed to create request counter", zap.Error(err))
	}

	requestDuration, err := meter.Float64Histogram(
		"http_request_duration_seconds",
		metric.WithDescription("HTTP request duration in seconds"),
	)
	if err != nil {
		logger.Error("Failed to create request duration histogram", zap.Error(err))
	}

	dbOperationCounter, err := meter.Int64Counter(
		"db_operations_total",
		metric.WithDescription("Total number of database operations"),
	)
	if err != nil {
		logger.Error("Failed to create db operation counter", zap.Error(err))
	}

	return &TodoHandler{
		repo:               repo,
		external:           external,
		tracer:             tracer,
		logger:             logger,
		meter:              meter,
		requestCounter:     requestCounter,
		requestDuration:    requestDuration,
		dbOperationCounter: dbOperationCounter,
	}
}

// logToBoth logs to both zap (terminal) and OpenTelemetry (Collector)
func (h *TodoHandler) logToBoth(ctx context.Context, level string, message string, fields ...zap.Field) {
	// Log to zap (terminal)
	switch level {
	case "debug":
		h.logger.Debug(message, fields...)
	case "info":
		h.logger.Info(message, fields...)
	case "warn":
		h.logger.Warn(message, fields...)
	case "error":
		h.logger.Error(message, fields...)
	}
}

// CreateTodo handles POST /todos
func (h *TodoHandler) CreateTodo(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	startTime := time.Now()
	defer func() {
		h.requestDuration.Record(ctx, time.Since(startTime).Seconds(), metric.WithAttributes(
			attribute.String("method", "POST"),
			attribute.String("route", "/todos"),
		))
	}()

	var req models.CreateTodoRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.requestCounter.Add(ctx, 1, metric.WithAttributes(
			attribute.String("method", "POST"),
			attribute.String("route", "/todos"),
			attribute.Int("status", http.StatusBadRequest),
		))
		return
	}

	if req.Title == "" {
		h.requestCounter.Add(ctx, 1, metric.WithAttributes(
			attribute.String("method", "POST"),
			attribute.String("route", "/todos"),
			attribute.Int("status", http.StatusBadRequest),
		))
		return
	}

	todo := &models.Todo{
		Title:     req.Title,
		Completed: req.Completed,
	}

	if err := h.repo.Create(ctx, todo); err != nil {
		h.requestCounter.Add(ctx, 1, metric.WithAttributes(
			attribute.String("method", "POST"),
			attribute.String("route", "/todos"),
			attribute.Int("status", http.StatusInternalServerError),
		))
		return
	}

	h.dbOperationCounter.Add(ctx, 1, metric.WithAttributes(
		attribute.String("operation", "create"),
		attribute.String("table", "todos"),
	))
	h.requestCounter.Add(ctx, 1, metric.WithAttributes(
		attribute.String("method", "POST"),
		attribute.String("route", "/todos"),
		attribute.Int("status", http.StatusCreated),
	))

	h.logToBoth(ctx, "info", fmt.Sprintf("Todo created: ID=%d, Title=%s", todo.ID, todo.Title),
		zap.Int64("id", todo.ID),
		zap.String("title", todo.Title),
	)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(todo)
}

// GetTodo handles GET /todos/{id}
func (h *TodoHandler) GetTodo(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	ctx, span := h.tracer.Start(ctx, "GetTodo")
	defer span.End()

	startTime := time.Now()
	defer func() {
		h.requestDuration.Record(ctx, time.Since(startTime).Seconds(), metric.WithAttributes(
			attribute.String("method", "GET"),
			attribute.String("route", "/todos/{id}"),
		))
	}()

	idStr := r.URL.Path[len("/todos/"):]
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		h.handleError(w, r, span, "Invalid todo ID", err, http.StatusBadRequest)
		return
	}

	span.SetAttributes(attribute.Int64("todo.id", id))

	// Create database operation span
	ctx, dbSpan := h.tracer.Start(ctx, "database.get",
		trace.WithAttributes(attribute.String("operation", "get")),
	)
	todo, err := h.repo.GetByID(ctx, id)
	if err != nil {
		dbSpan.RecordError(err)
		dbSpan.SetStatus(codes.Error, "failed to get todo")
		dbSpan.End()
		h.handleError(w, r, span, "Failed to get todo", err, http.StatusNotFound)
		return
	}
	dbSpan.End()

	h.dbOperationCounter.Add(ctx, 1, metric.WithAttributes(
		attribute.String("operation", "get"),
		attribute.String("table", "todos"),
	))
	h.requestCounter.Add(ctx, 1, metric.WithAttributes(
		attribute.String("method", "GET"),
		attribute.String("route", "/todos/{id}"),
		attribute.Int("status", http.StatusOK),
	))

	h.logToBoth(ctx, "info", fmt.Sprintf("Todo retrieved: ID=%d", todo.ID),
		zap.Int64("id", todo.ID),
		zap.String("title", todo.Title),
		zap.String("trace_id", span.SpanContext().TraceID().String()),
	)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(todo)
}

// ListTodos handles GET /todos
func (h *TodoHandler) ListTodos(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	startTime := time.Now()
	defer func() {
		h.requestDuration.Record(ctx, time.Since(startTime).Seconds(), metric.WithAttributes(
			attribute.String("method", "GET"),
			attribute.String("route", "/todos"),
		))
	}()

	// Parse query parameters
	limitStr := r.URL.Query().Get("limit")
	offsetStr := r.URL.Query().Get("offset")

	limit := 10
	offset := 0

	if limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 {
			limit = l
		}
	}
	if offsetStr != "" {
		if o, err := strconv.Atoi(offsetStr); err == nil && o >= 0 {
			offset = o
		}
	}

	todos, _ := h.repo.List(ctx, limit, offset)

	h.dbOperationCounter.Add(ctx, 1, metric.WithAttributes(
		attribute.String("operation", "list"),
		attribute.String("table", "todos"),
	))
	h.requestCounter.Add(ctx, 1, metric.WithAttributes(
		attribute.String("method", "GET"),
		attribute.String("route", "/todos"),
		attribute.Int("status", http.StatusOK),
	))

	h.logToBoth(ctx, "info", fmt.Sprintf("Todos listed: count=%d", len(todos)),
		zap.Int("count", len(todos)),
	)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(todos)
}

// UpdateTodo handles PUT /todos/{id}
func (h *TodoHandler) UpdateTodo(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	ctx, span := h.tracer.Start(ctx, "UpdateTodo")
	defer span.End()

	startTime := time.Now()
	defer func() {
		h.requestDuration.Record(ctx, time.Since(startTime).Seconds(), metric.WithAttributes(
			attribute.String("method", "PUT"),
			attribute.String("route", "/todos/{id}"),
		))
	}()

	idStr := r.URL.Path[len("/todos/"):]
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		h.handleError(w, r, span, "Invalid todo ID", err, http.StatusBadRequest)
		return
	}

	span.SetAttributes(attribute.Int64("todo.id", id))

	var req models.UpdateTodoRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.handleError(w, r, span, "Invalid request body", err, http.StatusBadRequest)
		return
	}

	// Get existing todo
	ctx, dbSpan := h.tracer.Start(ctx, "database.get",
		trace.WithAttributes(attribute.String("operation", "get")),
	)
	todo, err := h.repo.GetByID(ctx, id)
	if err != nil {
		dbSpan.RecordError(err)
		dbSpan.SetStatus(codes.Error, "failed to get todo")
		dbSpan.End()
		h.handleError(w, r, span, "Todo not found", err, http.StatusNotFound)
		return
	}
	dbSpan.End()

	h.dbOperationCounter.Add(ctx, 1, metric.WithAttributes(
		attribute.String("operation", "get"),
		attribute.String("table", "todos"),
	))

	// Update fields
	if req.Title != nil {
		todo.Title = *req.Title
	}
	if req.Completed != nil {
		todo.Completed = *req.Completed
	}

	// Update database
	ctx, updateSpan := h.tracer.Start(ctx, "database.update",
		trace.WithAttributes(attribute.String("operation", "update")),
	)
	if err := h.repo.Update(ctx, todo); err != nil {
		updateSpan.RecordError(err)
		updateSpan.SetStatus(codes.Error, "failed to update todo")
		updateSpan.End()
		h.handleError(w, r, span, "Failed to update todo", err, http.StatusInternalServerError)
		return
	}
	updateSpan.End()

	h.dbOperationCounter.Add(ctx, 1, metric.WithAttributes(
		attribute.String("operation", "update"),
		attribute.String("table", "todos"),
	))
	h.requestCounter.Add(ctx, 1, metric.WithAttributes(
		attribute.String("method", "PUT"),
		attribute.String("route", "/todos/{id}"),
		attribute.Int("status", http.StatusOK),
	))

	h.logToBoth(ctx, "info", fmt.Sprintf("Todo updated: ID=%d", todo.ID),
		zap.Int64("id", todo.ID),
		zap.String("title", todo.Title),
		zap.String("trace_id", span.SpanContext().TraceID().String()),
	)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(todo)
}

// DeleteTodo handles DELETE /todos/{id}
func (h *TodoHandler) DeleteTodo(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	ctx, span := h.tracer.Start(ctx, "DeleteTodo")
	defer span.End()

	startTime := time.Now()
	defer func() {
		h.requestDuration.Record(ctx, time.Since(startTime).Seconds(), metric.WithAttributes(
			attribute.String("method", "DELETE"),
			attribute.String("route", "/todos/{id}"),
		))
	}()

	idStr := r.URL.Path[len("/todos/"):]
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		h.handleError(w, r, span, "Invalid todo ID", err, http.StatusBadRequest)
		return
	}

	span.SetAttributes(attribute.Int64("todo.id", id))

	// Create database operation span
	ctx, dbSpan := h.tracer.Start(ctx, "database.delete",
		trace.WithAttributes(attribute.String("operation", "delete")),
	)
	if err := h.repo.Delete(ctx, id); err != nil {
		dbSpan.RecordError(err)
		dbSpan.SetStatus(codes.Error, "failed to delete todo")
		dbSpan.End()
		h.handleError(w, r, span, "Failed to delete todo", err, http.StatusNotFound)
		return
	}
	dbSpan.End()

	h.dbOperationCounter.Add(ctx, 1, metric.WithAttributes(
		attribute.String("operation", "delete"),
		attribute.String("table", "todos"),
	))
	h.requestCounter.Add(ctx, 1, metric.WithAttributes(
		attribute.String("method", "DELETE"),
		attribute.String("route", "/todos/{id}"),
		attribute.Int("status", http.StatusNoContent),
	))

	h.logToBoth(ctx, "info", fmt.Sprintf("Todo deleted: ID=%d", id),
		zap.Int64("id", id),
		zap.String("trace_id", span.SpanContext().TraceID().String()),
	)

	w.WriteHeader(http.StatusNoContent)
}

// GetExternalTodo handles GET /todos/external/{id}
func (h *TodoHandler) GetExternalTodo(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	ctx, span := h.tracer.Start(ctx, "GetExternalTodo")
	defer span.End()

	startTime := time.Now()
	defer func() {
		h.requestDuration.Record(ctx, time.Since(startTime).Seconds(), metric.WithAttributes(
			attribute.String("method", "GET"),
			attribute.String("route", "/todos/external/{id}"),
		))
	}()

	idStr := r.URL.Path[len("/todos/external/"):]
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		h.handleError(w, r, span, "Invalid todo ID", err, http.StatusBadRequest)
		return
	}

	span.SetAttributes(attribute.Int64("todo.id", id))

	// Create external API call span
	ctx, externalSpan := h.tracer.Start(ctx, "external_api.get_todo",
		trace.WithAttributes(
			attribute.String("endpoint", "jsonplaceholder.typicode.com"),
			attribute.String("path", "/todos"),
		),
	)
	todo, err := h.external.GetTodo(ctx, id)
	if err != nil {
		externalSpan.RecordError(err)
		externalSpan.SetStatus(codes.Error, "failed to fetch from external API")
		externalSpan.End()
		h.handleError(w, r, span, "Failed to fetch external todo", err, http.StatusBadGateway)
		return
	}
	externalSpan.End()

	h.requestCounter.Add(ctx, 1, metric.WithAttributes(
		attribute.String("method", "GET"),
		attribute.String("route", "/todos/external/{id}"),
		attribute.Int("status", http.StatusOK),
	))

	h.logToBoth(ctx, "info", fmt.Sprintf("External todo fetched: ID=%d", todo.ID),
		zap.Int64("id", todo.ID),
		zap.String("title", todo.Title),
		zap.String("trace_id", span.SpanContext().TraceID().String()),
	)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(todo)
}

// handleError handles errors consistently with telemetry
func (h *TodoHandler) handleError(w http.ResponseWriter, r *http.Request, span trace.Span, message string, err error, statusCode int) {
	span.RecordError(err)
	span.SetStatus(codes.Error, message)

	h.requestCounter.Add(r.Context(), 1, metric.WithAttributes(
		attribute.String("method", r.Method),
		attribute.String("route", r.URL.Path),
		attribute.Int("status", statusCode),
	))

	h.logToBoth(r.Context(), "error", message,
		zap.Error(err),
		zap.Int("status", statusCode),
		zap.String("trace_id", span.SpanContext().TraceID().String()),
	)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(map[string]string{"error": message})
}
