-- name: ListForecastsByMonitorID :many
SELECT
    *
FROM
    forecasts
WHERE
    monitor_id = sqlc.arg('monitor_id')
ORDER BY
    forecast_at;

-- name: UpsertForecast :one
INSERT INTO
    forecasts (
        forecast_at,
        monitor_id,
        temperature,
        dew_point,
        relative_humidity,
        wind_speed,
        visibility
    )
VALUES
    (
        sqlc.arg('forecast_at'),
        sqlc.arg('monitor_id'),
        sqlc.arg('temperature'),
        sqlc.arg('dew_point'),
        sqlc.arg('relative_humidity'),
        sqlc.arg('wind_speed'),
        sqlc.arg('visibility')
    ) ON CONFLICT (forecast_at, monitor_id) DO
UPDATE
SET
    temperature = EXCLUDED.temperature,
    dew_point = EXCLUDED.dew_point,
    relative_humidity = EXCLUDED.relative_humidity,
    wind_speed = EXCLUDED.wind_speed,
    visibility = EXCLUDED.visibility RETURNING *;