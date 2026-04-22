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

-- name: ListTraceSummaries :many
-- Aggregates spans per trace_id for the TUI list view.
-- root_service/root_name come from the span with NULL parent_span_id (the root).
SELECT
    trace_id                                             AS trace_id,
    CAST(MIN(start_time_ns) AS INTEGER)                  AS start_time_ns,
    CAST(MAX(end_time_ns) - MIN(start_time_ns) AS INTEGER) AS duration_ns,
    CAST(COUNT(*) AS INTEGER)                            AS span_count,
    CAST(MAX(CASE WHEN status_code = 2 THEN 1 ELSE 0 END) AS INTEGER) AS has_errors,
    CAST(COALESCE(
        (
            SELECT s2.service_name FROM spans s2
            WHERE s2.trace_id = spans.trace_id AND s2.parent_span_id IS NULL
            LIMIT 1
        ),
        ''
    ) AS TEXT)                                           AS root_service,
    CAST(COALESCE(
        (
            SELECT s2.name FROM spans s2
            WHERE s2.trace_id = spans.trace_id AND s2.parent_span_id IS NULL
            LIMIT 1
        ),
        ''
    ) AS TEXT)                                           AS root_name
FROM spans
GROUP BY trace_id
ORDER BY start_time_ns DESC
LIMIT ? OFFSET ?;
