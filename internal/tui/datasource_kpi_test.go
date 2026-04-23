package tui

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"testing"
	"time"

	_ "modernc.org/sqlite"

	"github.com/rahulSailesh-shah/otelkit/internal/store/repo"
)

const schemaMetricPoints = `
CREATE TABLE IF NOT EXISTS metric_points (
    id INTEGER PRIMARY KEY,
    name TEXT NOT NULL,
    description TEXT,
    unit TEXT,
    type INTEGER NOT NULL,
    service_name TEXT NOT NULL,
    attributes TEXT,
    timestamp_ns INTEGER NOT NULL,
    value_int INTEGER,
    value_double REAL,
    hist_count INTEGER,
    hist_sum REAL,
    hist_bounds TEXT,
    hist_counts TEXT,
    resource_attrs TEXT,
    ingested_at INTEGER NOT NULL DEFAULT (unixepoch())
);
CREATE INDEX idx_mp_name_ts ON metric_points(name, timestamp_ns);
CREATE INDEX idx_mp_service ON metric_points(service_name);
CREATE INDEX idx_mp_ingested ON metric_points(ingested_at);
`

func newMetricTestDB(t *testing.T) *sql.DB {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "metrics.db")
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() {
		db.Close()
		_ = os.Remove(path)
	})
	if _, err := db.Exec(schemaMetricPoints); err != nil {
		t.Fatalf("schema: %v", err)
	}
	return db
}

func TestLoadKPIsCmdFallsBackToLatestWhenWindowEmpty(t *testing.T) {
	db := newMetricTestDB(t)
	q := repo.New(db)
	ctx := context.Background()

	staleTs := time.Now().Add(-2 * time.Hour).UnixNano()
	unitSec := "s"

	insert := func(arg repo.InsertMetricPointParams) {
		t.Helper()
		if err := q.InsertMetricPoint(ctx, arg); err != nil {
			t.Fatalf("insert: %v", err)
		}
	}

	insert(repo.InsertMetricPointParams{
		Name:        "db.sql.latency",
		Type:        3,
		ServiceName: "svc",
		Attributes:  strPtr(`{"method":"sql.conn.exec"}`),
		TimestampNs: staleTs,
		HistCount:   i64Ptr(4),
		HistSum:     f64Ptr(40),
	})
	insert(repo.InsertMetricPointParams{
		Name:        "http.server.request.duration",
		Unit:        &unitSec,
		Type:        3,
		ServiceName: "svc",
		Attributes:  strPtr(`{"http.request.method":"GET"}`),
		TimestampNs: staleTs,
		HistCount:   i64Ptr(4),
		HistSum:     f64Ptr(1),
	})
	insert(repo.InsertMetricPointParams{
		Name:        "http.server.request.duration",
		Unit:        &unitSec,
		Type:        3,
		ServiceName: "svc",
		Attributes:  strPtr(`{"http.request.method":"POST"}`),
		TimestampNs: staleTs,
		HistCount:   i64Ptr(4),
		HistSum:     f64Ptr(2),
	})
	insert(repo.InsertMetricPointParams{
		Name:        "db.sql.connection.open",
		Type:        1,
		ServiceName: "svc",
		Attributes:  strPtr(`{"status":"idle"}`),
		TimestampNs: staleTs,
		ValueInt:    i64Ptr(3),
	})

	msg := loadKPIsCmd(ctx, q)().(kpisLoadedMsg)
	if msg.Err != nil {
		t.Fatalf("unexpected err: %v", msg.Err)
	}
	if msg.Data.SQLExecLatency != "10.00ms" {
		t.Fatalf("SQLExecLatency = %q, want %q", msg.Data.SQLExecLatency, "10.00ms")
	}
	if msg.Data.AvgGETDuration != "250.00ms" {
		t.Fatalf("AvgGETDuration = %q, want %q", msg.Data.AvgGETDuration, "250.00ms")
	}
	if msg.Data.AvgPOSTDuration != "500.00ms" {
		t.Fatalf("AvgPOSTDuration = %q, want %q", msg.Data.AvgPOSTDuration, "500.00ms")
	}
	if msg.Data.IdleConns != "3" {
		t.Fatalf("IdleConns = %q, want %q", msg.Data.IdleConns, "3")
	}
}

func TestLoadKPIsCmdReturnsErrorWhenQueryFails(t *testing.T) {
	db := newMetricTestDB(t)
	q := repo.New(db)
	ctx := context.Background()
	if err := db.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}

	msg := loadKPIsCmd(ctx, q)().(kpisLoadedMsg)
	if msg.Err == nil {
		t.Fatal("expected query error, got nil")
	}
}
