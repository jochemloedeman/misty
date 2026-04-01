-- name: CreateMonitor :one
INSERT INTO
    monitors (
        id,
        user_id,
        is_active,
        location_name,
        latitude,
        longitude,
        risk_window_start,
        risk_window_end
    )
VALUES
    (
        sqlc.arg('id'),
        sqlc.arg('user_id'),
        sqlc.arg('is_active'),
        sqlc.arg('location_name'),
        sqlc.arg('latitude'),
        sqlc.arg('longitude'),
        sqlc.arg('risk_window_start'),
        sqlc.arg('risk_window_end')
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

-- name: GetByMonitorID :one
SELECT
    *
FROM
    monitors
WHERE
    id = sqlc.arg('id')
    AND user_id = sqlc.arg('user_id');

-- name: UpdateMonitor :one
UPDATE
    monitors
SET
    is_active = sqlc.arg('is_active'),
    risk_window_start = sqlc.arg('risk_window_start'),
    risk_window_end = sqlc.arg('risk_window_end')
WHERE
    id = sqlc.arg('id')
    AND user_id = sqlc.arg('user_id') RETURNING *;

-- name: CountMonitorsByUser :one
SELECT
    count(*)
FROM
    monitors
WHERE
    user_id = sqlc.arg('user_id');

-- name: DeleteMonitor :execresult
DELETE FROM
    monitors
WHERE
    id = sqlc.arg('id')
    AND user_id = sqlc.arg('user_id');

-- name: ExistsMonitorByUserAndLocation :one
SELECT
    EXISTS (
        SELECT
            1
        FROM
            monitors
        WHERE
            user_id = sqlc.arg('user_id')
            AND latitude = sqlc.arg('latitude')
            AND longitude = sqlc.arg('longitude')
    );