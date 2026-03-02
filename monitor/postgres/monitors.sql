-- name: CreateMonitor :one
INSERT INTO
    monitors (
        is_active,
        location_name,
        latitude,
        longitude,
        daily_window_start,
        daily_window_end
    )
VALUES
    (
        sqlc.arg('is_active'),
        sqlc.arg('location_name'),
        sqlc.arg('latitude'),
        sqlc.arg('longitude'),
        sqlc.arg('daily_window_start'),
        sqlc.arg('daily_window_end')
    ) RETURNING *;

-- name: ListMonitors :many
SELECT
    *
FROM
    monitors
ORDER BY
    id;