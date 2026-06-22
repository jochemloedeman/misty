-- name: CreateNotification :one
INSERT INTO
    notifications (
        id,
        recipient_id,
        message,
        location_name,
        fog_start,
        fog_end,
        sent_at
    )
VALUES
    (
        sqlc.arg('id'),
        sqlc.arg('recipient_id'),
        sqlc.arg('message'),
        sqlc.arg('location_name'),
        sqlc.arg('fog_start'),
        sqlc.arg('fog_end'),
        sqlc.arg('sent_at')
    ) RETURNING *;

-- name: ListUnsentNotifications :many
SELECT
    *
FROM
    notifications
WHERE
    status = 'pending'
ORDER BY
    id;

-- name: GetUnsentNotification :one
SELECT
    *
FROM
    notifications
WHERE
    id = sqlc.arg('id')
    AND status = 'pending';

-- name: MarkNotificationSent :one
UPDATE
    notifications
SET
    sent_at = sqlc.arg('sent_at'),
    status = 'sent'
WHERE
    id = sqlc.arg('id') RETURNING *;

-- name: MarkNotificationExpired :one
UPDATE
    notifications
SET
    status = 'expired'
WHERE
    id = sqlc.arg('id') RETURNING *;