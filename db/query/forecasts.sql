-- name: ListForecastsByMonitorIDAndHorizon :many
SELECT
    *
FROM
    forecasts
WHERE
    monitor_id = sqlc.arg('monitor_id')
    AND forecast_at >= sqlc.arg('from')
    AND forecast_at <= sqlc.arg('until')
ORDER BY
    forecast_at;

-- name: UpsertForecasts :exec
INSERT INTO
    forecasts (
        forecast_at,
        monitor_id,
        temperature,
        dew_point,
        relative_humidity,
        wind_speed,
        visibility,
        weather_code
    )
SELECT
    unnest(sqlc.arg('forecast_at')::timestamptz[]),
    sqlc.arg('monitor_id')::uuid,
    unnest(sqlc.arg('temperature')::float8[]),
    unnest(sqlc.arg('dew_point')::float8[]),
    unnest(sqlc.arg('relative_humidity')::float8[]),
    unnest(sqlc.arg('wind_speed')::float8[]),
    unnest(sqlc.arg('visibility')::float8[]),
    unnest(sqlc.arg('weather_code')::int[])
ON CONFLICT (forecast_at, monitor_id) DO
UPDATE
SET
    temperature = EXCLUDED.temperature,
    dew_point = EXCLUDED.dew_point,
    relative_humidity = EXCLUDED.relative_humidity,
    wind_speed = EXCLUDED.wind_speed,
    visibility = EXCLUDED.visibility,
    weather_code = EXCLUDED.weather_code;
