-- +goose Up
ALTER TABLE forecasts ADD COLUMN weather_code INTEGER NOT NULL DEFAULT 0;

-- +goose Down
ALTER TABLE forecasts DROP COLUMN weather_code;
