-- +goose Up
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS spans (
    span_id        TEXT NOT NULL,
    trace_id       TEXT NOT NULL,
    parent_span_id TEXT,
    name           TEXT NOT NULL,
    service_name   TEXT NOT NULL,
    span_kind      INTEGER NOT NULL DEFAULT 0,
    start_time_ns  INTEGER NOT NULL,
    end_time_ns    INTEGER NOT NULL,
    duration_ns    INTEGER GENERATED ALWAYS AS (end_time_ns - start_time_ns) STORED,
    -- OTLP SpanStatus: 0=unset, 1=ok, 2=error
    status_code    INTEGER NOT NULL DEFAULT 0,
    status_message TEXT,
    attributes     TEXT,
    events         TEXT,
    resource_attrs TEXT,
    ingested_at    INTEGER NOT NULL DEFAULT (unixepoch()),
    PRIMARY KEY (trace_id, span_id)
);
-- +goose StatementEnd

-- List traces sorted by recency; partial keeps write cost low (~1 write per trace)
-- +goose StatementBegin
CREATE INDEX idx_spans_root_time ON spans(start_time_ns DESC)
    WHERE parent_span_id IS NULL;
-- +goose StatementEnd

-- Retention cleanup: DELETE WHERE ingested_at < unixepoch() - ?
-- +goose StatementBegin
CREATE INDEX idx_spans_ingested ON spans(ingested_at);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS spans;
-- +goose StatementEnd
