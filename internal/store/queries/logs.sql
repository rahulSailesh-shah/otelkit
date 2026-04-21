-- name: InsertLogRecord :exec
INSERT INTO log_records (
    trace_id,
    span_id,
    service_name,
    severity,
    severity_text,
    body,
    attributes,
    resource_attrs,
    timestamp_ns
) VALUES (
    ?, ?, ?, ?, ?, ?, ?, ?, ?
);

-- name: ListRecentLogRecords :many
SELECT * FROM log_records
ORDER BY timestamp_ns DESC
LIMIT ? OFFSET ?;

-- name: ListLogRecordsByService :many
SELECT * FROM log_records
WHERE service_name = ?
ORDER BY timestamp_ns DESC
LIMIT ? OFFSET ?;

-- name: ListLogRecordsBySeverity :many
SELECT * FROM log_records
WHERE severity = ?
ORDER BY timestamp_ns DESC
LIMIT ? OFFSET ?;

-- name: ListLogRecordsByTrace :many
SELECT * FROM log_records
WHERE trace_id = ?
ORDER BY timestamp_ns ASC;

-- name: ListLogRecordsByTimeRange :many
SELECT * FROM log_records
WHERE timestamp_ns >= ?
  AND timestamp_ns <= ?
ORDER BY timestamp_ns ASC;

-- name: DeleteLogRecordsOlderThan :exec
DELETE FROM log_records WHERE ingested_at < unixepoch() - ?;
