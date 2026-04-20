-- name: InsertMetricPoint :exec
INSERT INTO metric_points (
    name,
    description,
    unit,
    type,
    service_name,
    attributes,
    timestamp_ns,
    value_int,
    value_double,
    hist_count,
    hist_sum,
    hist_bounds,
    hist_counts,
    resource_attrs
) VALUES (
    ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?
);

-- name: GetMetricPoint :one
SELECT * FROM metric_points WHERE id = ? LIMIT 1;

-- name: ListMetricPointsByName :many
SELECT * FROM metric_points
WHERE name = ?
ORDER BY timestamp_ns DESC
LIMIT ? OFFSET ?;

-- name: ListMetricPointsByNameAndTimeRange :many
SELECT * FROM metric_points
WHERE name = ?
  AND timestamp_ns >= ?
  AND timestamp_ns <= ?
ORDER BY timestamp_ns ASC;

-- name: ListMetricPointsByService :many
SELECT * FROM metric_points
WHERE service_name = ?
ORDER BY timestamp_ns DESC
LIMIT ? OFFSET ?;

-- name: ListMetricPointsByServiceAndTimeRange :many
SELECT * FROM metric_points
WHERE service_name = ?
  AND timestamp_ns >= ?
  AND timestamp_ns <= ?
ORDER BY timestamp_ns ASC;

-- name: ListMetricPointsByType :many
SELECT * FROM metric_points
WHERE type = ?
ORDER BY timestamp_ns DESC
LIMIT ? OFFSET ?;

-- name: ListRecentMetricPoints :many
SELECT * FROM metric_points
ORDER BY timestamp_ns DESC
LIMIT ? OFFSET ?;

-- name: ListMetricNames :many
SELECT DISTINCT name FROM metric_points ORDER BY name;

-- name: ListMetricNamesByService :many
SELECT DISTINCT name FROM metric_points
WHERE service_name = ?
ORDER BY name;

-- name: ListServicesWithMetrics :many
SELECT DISTINCT service_name FROM metric_points ORDER BY service_name;

-- name: GetMetricPointCountByName :one
SELECT COUNT(*) FROM metric_points WHERE name = ?;

-- name: GetMetricPointCountByService :one
SELECT COUNT(*) FROM metric_points WHERE service_name = ?;

-- name: GetFirstMetricTimestamp :one
SELECT MIN(timestamp_ns) FROM metric_points WHERE name = ?;

-- name: GetLastMetricTimestamp :one
SELECT MAX(timestamp_ns) FROM metric_points WHERE name = ?;

-- name: DeleteMetricPointsOlderThan :exec
DELETE FROM metric_points WHERE ingested_at < unixepoch() - ?;

-- name: DeleteMetricPointsByName :exec
DELETE FROM metric_points WHERE name = ?;

-- name: DeleteMetricPointsByService :exec
DELETE FROM metric_points WHERE service_name = ?;
