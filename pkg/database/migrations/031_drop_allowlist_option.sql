-- +goose Up
ALTER TABLE teams DROP COLUMN "enable_allowlist";

-- +goose Down
ALTER TABLE teams ADD COLUMN "enable_allowlist" BOOLEAN NOT NULL DEFAULT false;