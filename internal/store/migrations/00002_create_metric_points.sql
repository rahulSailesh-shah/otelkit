-- +goose Up
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS metric_points (
    id INTEGER PRIMARY KEY,
    name TEXT NOT NULL,
    description TEXT,
    unit TEXT,
    type INTEGER NOT NULL, -- 1=gauge 2=sum 3=histogram
    service_name TEXT NOT NULL,
    attributes TEXT, -- JSON object (low-cardinality labels)
    timestamp_ns INTEGER NOT NULL,
    value_int INTEGER,
    value_double REAL,
    hist_count INTEGER,
    hist_sum REAL,
    hist_bounds TEXT, -- JSON array of float64
    hist_counts TEXT, -- JSON array of uint64
    resource_attrs TEXT, -- JSON object
    ingested_at INTEGER NOT NULL DEFAULT (unixepoch())
);
-- +goose StatementEnd

-- Index for querying metric series by name and time range
-- +goose StatementBegin
CREATE INDEX idx_mp_name_ts ON metric_points(name, timestamp_ns);
-- +goose StatementEnd

-- Index for filtering by service name
-- +goose StatementBegin
CREATE INDEX idx_mp_service ON metric_points(service_name);
-- +goose StatementEnd

-- Retention cleanup: DELETE WHERE ingested_at < unixepoch() - ?
-- +goose StatementBegin
CREATE INDEX idx_mp_ingested ON metric_points(ingested_at);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS metric_points;
-- +goose StatementEnd
