-- name: CreateAlert :one
INSERT INTO
    alerts(monitor_id, start_time, end_time)
VALUES
    (
        sqlc.arg('monitor_id'),
        sqlc.arg('start_time'),
        sqlc.arg('end_time')
    ) RETURNING *;