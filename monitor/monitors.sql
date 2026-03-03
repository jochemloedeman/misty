-- name: CreateMonitor :one
INSERT INTO
    monitors (
        id,
        is_active,
        latitude,
        longitude,
        alert_start,
        alert_end
    )
VALUES
    (
        sqlc.arg('id'),
        sqlc.arg('is_active'),
        sqlc.arg('latitude'),
        sqlc.arg('longitude'),
        sqlc.arg('alert_start'),
        sqlc.arg('alert_end')
    ) RETURNING *;

-- name: ListMonitors :many
SELECT
    *
FROM
    monitors
ORDER BY
    id;

-- name: ListActiveMonitors :many
SELECT
    *
FROM
    monitors
WHERE
    is_active = true
ORDER BY
    id;

-- name: UpdateMonitorAlert :one
UPDATE
    monitors
SET
    alert_start = sqlc.arg('alert_start'),
    alert_end = sqlc.arg('alert_end')
WHERE
    id = sqlc.arg('id') RETURNING *;