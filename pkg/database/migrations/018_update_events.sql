-- +goose Up
ALTER TABLE events RENAME COLUMN task TO payload;
ALTER TABLE events ALTER COLUMN deadline TYPE interval USING deadline::interval;
ALTER TABLE events ADD COLUMN retry_count INT NOT NULL DEFAULT 0;

-- +goose Down
ALTER TABLE events RENAME COLUMN payload TO task;
ALTER TABLE events ALTER COLUMN deadline TYPE TEXT;
ALTER TABLE events DROP COLUMN retry_count;