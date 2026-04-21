-- +goose Up
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS log_records (
    id             INTEGER PRIMARY KEY,
    trace_id       TEXT,
    span_id        TEXT,
    service_name   TEXT NOT NULL,
    severity       INTEGER,
    severity_text  TEXT,
    body           TEXT,
    attributes     TEXT,
    resource_attrs TEXT,
    timestamp_ns   INTEGER NOT NULL,
    ingested_at    INTEGER NOT NULL DEFAULT (unixepoch())
);
-- +goose StatementEnd

-- +goose StatementBegin
CREATE INDEX idx_lr_trace    ON log_records(trace_id) WHERE trace_id IS NOT NULL;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE INDEX idx_lr_sev      ON log_records(severity);
-- +goose StatementEnd

-- +goose StatementBegin
CREATE INDEX idx_lr_ts       ON log_records(timestamp_ns);
-- +goose StatementEnd

-- +goose StatementBegin
CREATE INDEX idx_lr_service  ON log_records(service_name);
-- +goose StatementEnd

-- +goose StatementBegin
CREATE INDEX idx_lr_ingested ON log_records(ingested_at);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS log_records;
-- +goose StatementEnd
