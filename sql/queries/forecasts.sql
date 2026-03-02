-- name: CreateForecast :one
INSERT INTO
    forecasts (
        forecasted_at,
        monitor_id,
        temperature,
        dew_point,
        relative_humidity,
        wind_speed,
        visibility
    )
VALUES
    (
        sqlc.arg ('forecasted_at'),
        sqlc.arg ('monitor_id'),
        sqlc.arg ('temperature'),
        sqlc.arg ('dew_point'),
        sqlc.arg ('relative_humidity'),
        sqlc.arg ('wind_speed'),
        sqlc.arg ('visibility')
    ) RETURNING *;