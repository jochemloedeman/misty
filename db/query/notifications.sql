-- name: CreateNotification :one
INSERT INTO
    notifications (
        id,
        recipient_id,
        message,
        sent_at
    )
VALUES
    (
        sqlc.arg('id'),
        sqlc.arg('recipient_id'),
        sqlc.arg('message'),
        sqlc.arg('sent_at')
    ) RETURNING *;

-- name: ListUnsentNotifications :many
SELECT
    id,
    recipient_id,
    message,
    sent_at
FROM
    notifications
WHERE
    sent_at IS NULL
ORDER BY
    id;

-- name: UpdateNotificationSentAt :one
UPDATE
    notifications
SET
    sent_at = sqlc.arg('sent_at')
WHERE
    id = sqlc.arg('id') RETURNING *;