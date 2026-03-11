-- name: CreateUser :one
INSERT INTO
    users (
        id,
        push_token,
        refresh_token
    )
VALUES
    (
        sqlc.arg('id'),
        sqlc.arg('push_token'),
        sqlc.arg('refresh_token')
    ) RETURNING *;

-- name: EnsureUser :exec
INSERT INTO
    users (
        id,
        push_token,
        refresh_token
    )
VALUES
    (
        sqlc.arg('id'),
        sqlc.arg('push_token'),
        sqlc.arg('refresh_token')
    ) ON CONFLICT (id) DO NOTHING;