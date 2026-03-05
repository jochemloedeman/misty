-- name: CreateUser :one
INSERT INTO
    users (id)
VALUES
    (sqlc.arg('id')) RETURNING *;