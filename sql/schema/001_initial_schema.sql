-- +goose Up

CREATE TABLE monitors (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid (),
    is_active BOOLEAN NOT NULL,
    location_name TEXT NOT NULL,
    latitude DOUBLE PRECISION NOT NULL,
    longitude DOUBLE PRECISION NOT NULL,
    daily_window_start TIME,
    daily_window_end TIME,
    CONSTRAINT daily_window_all_or_none CHECK (
        (daily_window_start IS NULL) = (daily_window_end IS NULL)
    )
);

CREATE TABLE forecasts (
    forecasted_at TIMESTAMPTZ NOT NULL,
    monitor_id UUID NOT NULL REFERENCES monitors (id) ON DELETE CASCADE,
    temperature DOUBLE PRECISION NOT NULL,
    dew_point DOUBLE PRECISION NOT NULL,
    relative_humidity DOUBLE PRECISION NOT NULL,
    wind_speed DOUBLE PRECISION NOT NULL,
    visibility INT NOT NULL,
    PRIMARY KEY (forecasted_at, monitor_id)
);

CREATE TABLE alerts (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid (),
    monitor_id UUID NOT NULL REFERENCES monitors (id),
    start_time TIMESTAMPTZ NOT NULL,
    end_time TIMESTAMPTZ NOT NULL
);

-- +goose Down
DROP TABLE alerts;

DROP TABLE forecasts;

DROP TABLE monitors;