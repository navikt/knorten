-- +goose Up
ALTER TABLE events ALTER COLUMN deadline TYPE TEXT;

-- +goose Down
ALTER TABLE events ALTER COLUMN deadline TYPE interval USING deadline::interval;
