-- name: CreateMonitor :one
INSERT INTO
    monitors (
        id,
        user_id,
        is_active,
        location_name,
        latitude,
        longitude,
        alert_start,
        alert_end
    )
VALUES
    (
        sqlc.arg('id'),
        sqlc.arg('user_id'),
        sqlc.arg('is_active'),
        sqlc.arg('location_name'),
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
WHERE
    user_id = sqlc.arg('user_id')
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

-- name: GetByID :one
SELECT
    *
FROM
    monitors
WHERE
    id = sqlc.arg('id');

-- name: UpdateMonitorAlert :one
UPDATE
    monitors
SET
    alert_start = sqlc.arg('alert_start'),
    alert_end = sqlc.arg('alert_end')
WHERE
    id = sqlc.arg('id')
    AND user_id = sqlc.arg('user_id') RETURNING *;