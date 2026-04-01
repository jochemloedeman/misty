-- +goose Up
ALTER TABLE monitors RENAME COLUMN alert_start TO risk_window_start;
ALTER TABLE monitors RENAME COLUMN alert_end TO risk_window_end;
ALTER TABLE monitors RENAME CONSTRAINT valid_alert_period TO valid_risk_window_period;

-- +goose Down
ALTER TABLE monitors RENAME CONSTRAINT valid_risk_window_period TO valid_alert_period;
ALTER TABLE monitors RENAME COLUMN risk_window_end TO alert_end;
ALTER TABLE monitors RENAME COLUMN risk_window_start TO alert_start;
