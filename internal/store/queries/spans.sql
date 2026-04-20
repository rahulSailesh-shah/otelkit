-- name: InsertSpan :exec
INSERT INTO spans (
    span_id, trace_id, parent_span_id, name, service_name,
    span_kind, start_time_ns, end_time_ns,
    status_code, status_message, attributes, events, resource_attrs
) VALUES (
    ?, ?, ?, ?, ?,
    ?, ?, ?,
    ?, ?, ?, ?, ?
);

-- name: GetSpan :one
SELECT * FROM spans
WHERE trace_id = ? AND span_id = ?
LIMIT 1;

-- name: ListSpansByTrace :many
SELECT * FROM spans
WHERE trace_id = ?
ORDER BY start_time_ns ASC;

-- name: ListSpansByService :many
SELECT * FROM spans
WHERE service_name = ?
ORDER BY start_time_ns DESC
LIMIT ? OFFSET ?;

-- name: ListRecentSpans :many
SELECT * FROM spans
ORDER BY start_time_ns DESC
LIMIT ? OFFSET ?;

-- name: DeleteSpansOlderThan :exec
DELETE FROM spans
WHERE ingested_at < unixepoch() - ?;
