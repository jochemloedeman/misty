-- name: CreateAlert :one
INSERT INTO
    alerts (
        monitor_id,
        start_time,
        end_time
    )
VALUES (
        @monitor_id,
        @start_time,
        @end_time
    )
RETURNING *;
