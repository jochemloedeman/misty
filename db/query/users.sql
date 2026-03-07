-- name: CreateUser :one
INSERT INTO
    users (id)
VALUES
    (sqlc.arg('id')) RETURNING *;

-- name: EnsureUser :exec
INSERT INTO
    users (id)
VALUES
    (sqlc.arg('id')) ON CONFLICT (id) DO NOTHING;