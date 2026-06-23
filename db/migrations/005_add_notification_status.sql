-- +goose Up
CREATE TYPE notification_status AS ENUM ('pending', 'sent', 'expired', 'undeliverable');

ALTER TABLE notifications
    ADD COLUMN status notification_status NOT NULL DEFAULT 'pending';

UPDATE notifications SET status = 'sent' WHERE sent_at IS NOT NULL;

-- +goose Down
ALTER TABLE notifications DROP COLUMN status;

DROP TYPE notification_status;
