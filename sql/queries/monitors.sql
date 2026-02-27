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
VALUES (
        @ is_active,
        @ location_name,
        @ latitude,
        @ longitude,
        @ daily_window_start,
        @ daily_window_end
    )
RETURNING
    *;