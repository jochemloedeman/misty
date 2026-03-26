-- +goose Up
ALTER TABLE notifications ADD COLUMN location_name TEXT NOT NULL DEFAULT '';
ALTER TABLE notifications ADD COLUMN fog_start TIMESTAMPTZ;
ALTER TABLE notifications ADD COLUMN fog_end TIMESTAMPTZ;

-- +goose Down
ALTER TABLE notifications DROP COLUMN location_name;
ALTER TABLE notifications DROP COLUMN fog_start;
ALTER TABLE notifications DROP COLUMN fog_end;
