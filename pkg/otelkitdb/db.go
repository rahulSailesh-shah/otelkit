package otelkitdb

import (
	"database/sql"

	"github.com/XSAM/otelsql"
	"go.opentelemetry.io/otel/attribute"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
)

// driverAttrs maps SQL driver names to OTel DB system semconv attributes.
var driverAttrs = map[string]attribute.KeyValue{
	"sqlite3":  semconv.DBSystemSqlite,
	"sqlite":   semconv.DBSystemSqlite,
	"postgres": semconv.DBSystemPostgreSQL,
	"pgx":      semconv.DBSystemPostgreSQL,
	"pgx/v5":   semconv.DBSystemPostgreSQL,
	"mysql":    semconv.DBSystemMySQL,
}

// Open wraps sql.Open with OTel instrumentation.
// Auto-detects the DB system semconv attribute from driverName.
// Registers DB connection pool metrics against the global MeterProvider.
func Open(driverName, dataSourceName string) (*sql.DB, error) {
	var attrs []attribute.KeyValue
	if attr, ok := driverAttrs[driverName]; ok {
		attrs = append(attrs, attr)
	}

	db, err := otelsql.Open(driverName, dataSourceName,
		otelsql.WithAttributes(attrs...),
		otelsql.WithSpanOptions(otelsql.SpanOptions{Ping: true}),
	)
	if err != nil {
		return nil, err
	}

	otelsql.RegisterDBStatsMetrics(db, otelsql.WithAttributes(attrs...))
	return db, nil
}
