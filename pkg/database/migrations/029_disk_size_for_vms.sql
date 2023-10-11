-- +goose Up
ALTER TABLE compute_instances ADD COLUMN disk_size INTEGER NOT NULL DEFAULT 10;

-- +goose Down
ALTER TABLE compute_instances DROP COLUMN disk_size;
