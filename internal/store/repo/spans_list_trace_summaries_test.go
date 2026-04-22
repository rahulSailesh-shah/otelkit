package repo_test

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"testing"

	_ "modernc.org/sqlite"

	"github.com/rahulSailesh-shah/otelkit/internal/store/repo"
)

const schemaSpans = `
CREATE TABLE IF NOT EXISTS spans (
    span_id         TEXT NOT NULL,
    trace_id        TEXT NOT NULL,
    parent_span_id  TEXT,
    name            TEXT NOT NULL,
    service_name    TEXT NOT NULL,
    span_kind       INTEGER NOT NULL DEFAULT 0,
    start_time_ns   INTEGER NOT NULL,
    end_time_ns     INTEGER NOT NULL,
    duration_ns     INTEGER GENERATED ALWAYS AS (end_time_ns - start_time_ns) STORED,
    status_code     INTEGER NOT NULL DEFAULT 0,
    status_message  TEXT,
    attributes      TEXT,
    events          TEXT,
    resource_attrs  TEXT,
    ingested_at     INTEGER NOT NULL DEFAULT (unixepoch()),
    PRIMARY KEY (trace_id, span_id)
);`

func newTestDB(t *testing.T) *sql.DB {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "test.db")
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() {
		db.Close()
		_ = os.Remove(path)
	})
	if _, err := db.Exec(schemaSpans); err != nil {
		t.Fatalf("schema: %v", err)
	}
	return db
}

func TestListTraceSummaries(t *testing.T) {
	db := newTestDB(t)
	q := repo.New(db)
	ctx := context.Background()

	insertSpan := func(traceID, spanID string, parent *string, service, name string, start, end int64, status int64) {
		t.Helper()
		if err := q.InsertSpan(ctx, repo.InsertSpanParams{
			SpanID:       spanID,
			TraceID:      traceID,
			ParentSpanID: parent,
			Name:         name,
			ServiceName:  service,
			SpanKind:     0,
			StartTimeNs:  start,
			EndTimeNs:    end,
			StatusCode:   status,
		}); err != nil {
			t.Fatalf("insert: %v", err)
		}
	}

	insertSpan("tA", "s1", nil, "svc-a", "GET /a", 1_000, 5_000, 0)
	child := "s1"
	insertSpan("tA", "s2", &child, "svc-a", "db.query", 2_000, 4_000, 0)

	insertSpan("tB", "s3", nil, "svc-b", "POST /b", 500, 800, 2)

	rows, err := q.ListTraceSummaries(ctx, repo.ListTraceSummariesParams{Limit: 10, Offset: 0})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("rows = %d, want 2", len(rows))
	}

	a, b := rows[0], rows[1]
	if a.TraceID != "tA" || b.TraceID != "tB" {
		t.Fatalf("order wrong: %q,%q", a.TraceID, b.TraceID)
	}
	if a.SpanCount != 2 || b.SpanCount != 1 {
		t.Errorf("span_count: a=%d b=%d", a.SpanCount, b.SpanCount)
	}
	if a.DurationNs != 4000 || b.DurationNs != 300 {
		t.Errorf("duration: a=%d b=%d", a.DurationNs, b.DurationNs)
	}
	if a.HasErrors != 0 || b.HasErrors != 1 {
		t.Errorf("has_errors: a=%d b=%d", a.HasErrors, b.HasErrors)
	}
	if a.RootService != "svc-a" {
		t.Errorf("root_service a=%q want svc-a", a.RootService)
	}
	if a.RootName != "GET /a" {
		t.Errorf("root_name a=%q want GET /a", a.RootName)
	}
}
