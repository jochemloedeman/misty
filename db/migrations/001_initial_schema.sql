-- +goose Up
CREATE TABLE users(id UUID PRIMARY KEY);

CREATE TABLE monitors(
    id UUID PRIMARY KEY,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    is_active BOOLEAN NOT NULL,
    location_name TEXT NOT NULL,
    latitude DOUBLE PRECISION NOT NULL,
    longitude DOUBLE PRECISION NOT NULL,
    alert_start TIMESTAMPTZ,
    alert_end TIMESTAMPTZ CONSTRAINT valid_alert_period CHECK (
        (
            alert_start IS NULL
            AND alert_end IS NULL
        )
        OR (
            alert_start IS NOT NULL
            AND alert_end IS NOT NULL
            AND alert_start < alert_end
        )
    )
);

CREATE TABLE forecasts(
    forecast_at TIMESTAMPTZ NOT NULL,
    monitor_id UUID NOT NULL REFERENCES monitors(id) ON DELETE CASCADE,
    temperature DOUBLE PRECISION NOT NULL,
    dew_point DOUBLE PRECISION NOT NULL,
    relative_humidity DOUBLE PRECISION NOT NULL,
    wind_speed DOUBLE PRECISION NOT NULL,
    visibility INT NOT NULL,
    PRIMARY KEY (forecast_at, monitor_id)
);

CREATE TABLE notifications(
    id UUID PRIMARY KEY,
    recipient_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    message TEXT NOT NULL,
    sent_at TIMESTAMPTZ
);

-- +goose Down
DROP TABLE users;

DROP TABLE forecasts;

DROP TABLE monitors;

drop table notifications;