-- name: CreateUser :one
INSERT INTO
    users (
        id,
        refresh_token
    )
VALUES
    (
        sqlc.arg('id'),
        sqlc.arg('refresh_token')
    ) RETURNING *;

-- name: EnsureUser :exec
INSERT INTO
    users (
        id,
        refresh_token
    )
VALUES
    (
        sqlc.arg('id'),
        sqlc.arg('refresh_token')
    ) ON CONFLICT (id) DO NOTHING;

-- name: GetByRefreshToken :one
SELECT
    id,
    push_token,
    refresh_token
FROM
    users
WHERE
    refresh_token = sqlc.arg('refresh_token');

-- name: GetPushTokenByUserID :one
SELECT
    push_token
FROM
    users
WHERE
    id = sqlc.arg('id');

-- name: UpdatePushToken :one
UPDATE users
SET push_token = sqlc.arg('push_token')
WHERE id = sqlc.arg('id')
RETURNING *;

-- name: ClearPushToken :exec
UPDATE users
SET push_token = NULL
WHERE id = sqlc.arg('id') AND push_token = sqlc.arg('push_token');