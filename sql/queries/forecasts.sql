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
VALUES (
        @forecasted_at,
        @monitor_id,
        @temperature,
        @dew_point,
        @relative_humidity,
        @wind_speed,
        @visibility
    )
RETURNING *;
